package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// midjourneyPollSummary is the result recorded on a midjourney_poll system task
// row, summarizing one polling pass.
type midjourneyPollSummary struct {
	UnfinishedTasks int `json:"unfinished_tasks"`
	ChannelsScanned int `json:"channels_scanned"`
	NullTasksFailed int `json:"null_tasks_failed"`
}

// runMidjourneyTaskUpdateOnce performs one Midjourney polling pass synchronously.
// It honors ctx cancellation (the system-task runner cancels it when the lease
// is lost) and, when report is non-nil, reports progress as (processedChannels,
// totalChannels) so the system task surfaces a percentage.
func runMidjourneyTaskUpdateOnce(ctx context.Context, report func(processed, total int)) midjourneyPollSummary {
	summary := midjourneyPollSummary{}
	if ctx == nil {
		ctx = context.Background()
	}

	tasks := model.GetAllUnFinishTasks()
	if len(tasks) == 0 {
		return summary
	}
	summary.UnfinishedTasks = len(tasks)

	logger.LogInfo(ctx, fmt.Sprintf("检测到未完成的任务数有: %v", len(tasks)))
	taskChannelM := make(map[int][]string)
	taskM := make(map[string]*model.Midjourney)
	nullTaskIds := make([]int, 0)
	for _, task := range tasks {
		if task.MjId == "" {
			// 统计失败的未完成任务
			nullTaskIds = append(nullTaskIds, task.Id)
			continue
		}
		taskM[task.MjId] = task
		taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], task.MjId)
	}
	if len(nullTaskIds) > 0 {
		summary.NullTasksFailed = len(nullTaskIds)
		err := model.MjBulkUpdateByTaskIds(nullTaskIds, map[string]any{
			"status":   "FAILURE",
			"progress": "100%",
		})
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Fix null mj_id task error: %v", err))
		} else {
			logger.LogInfo(ctx, fmt.Sprintf("Fix null mj_id task success: %v", nullTaskIds))
		}
	}
	if len(taskChannelM) == 0 {
		return summary
	}

	totalChannels := len(taskChannelM)
	processedChannels := 0
	for channelId, taskIds := range taskChannelM {
		if ctx != nil && ctx.Err() != nil {
			break
		}
		if report != nil {
			report(processedChannels, totalChannels)
		}
		processedChannels++
		summary.ChannelsScanned++
		logger.LogInfo(ctx, fmt.Sprintf("渠道 #%d 未完成的任务有: %d", channelId, len(taskIds)))
		if len(taskIds) == 0 {
			continue
		}
		midjourneyChannel, err := model.CacheGetChannel(channelId)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("CacheGetChannel: %v", err))
			err := model.MjBulkUpdate(taskIds, map[string]any{
				"fail_reason": fmt.Sprintf("获取渠道信息失败，请联系管理员，渠道ID：%d", channelId),
				"status":      "FAILURE",
				"progress":    "100%",
			})
			if err != nil {
				logger.LogInfo(ctx, fmt.Sprintf("UpdateMidjourneyTask error: %v", err))
			}
			continue
		}
		requestUrl := fmt.Sprintf("%s/mj/task/list-by-condition", *midjourneyChannel.BaseURL)

		body, err := common.Marshal(map[string]any{
			"ids": taskIds,
		})
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Get Task marshal body error: %v", err))
			continue
		}
		timeout := time.Second * 15
		requestCtx, cancel := context.WithTimeout(ctx, timeout)
		req, err := http.NewRequestWithContext(requestCtx, "POST", requestUrl, bytes.NewBuffer(body))
		if err != nil {
			cancel()
			logger.LogError(ctx, fmt.Sprintf("Get Task error: %v", err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("mj-api-secret", midjourneyChannel.Key)
		resp, err := service.GetHttpClient().Do(req)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Get Task Do req error: %v", err))
			cancel()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			logger.LogError(ctx, fmt.Sprintf("Get Task status code: %d", resp.StatusCode))
			resp.Body.Close()
			cancel()
			continue
		}
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Get Mjp Task parse body error: %v", err))
			resp.Body.Close()
			cancel()
			continue
		}
		var responseItems []dto.MidjourneyDto
		err = common.Unmarshal(responseBody, &responseItems)
		if err != nil {
			logger.LogError(ctx, fmt.Sprintf("Get Mjp Task parse body error2: %v, body: %s", err, string(responseBody)))
			resp.Body.Close()
			cancel()
			continue
		}
		resp.Body.Close()
		req.Body.Close()
		cancel()

		for _, responseItem := range responseItems {
			task := taskM[responseItem.MjId]
			if task == nil {
				logger.LogWarn(ctx, fmt.Sprintf("Midjourney task response ignored: unknown mj_id=%s", responseItem.MjId))
				continue
			}

			useTime := (time.Now().UnixNano() / int64(time.Millisecond)) - task.SubmitTime
			// 如果时间超过一小时，且进度不是100%，则认为任务失败
			if useTime > 3600000 && task.Progress != "100%" {
				responseItem.FailReason = "上游任务超时（超过1小时）"
				responseItem.Status = "FAILURE"
			}
			if !checkMjTaskNeedUpdate(task, responseItem) {
				continue
			}
			preStatus := task.Status
			task.Code = 1
			task.Progress = responseItem.Progress
			task.PromptEn = responseItem.PromptEn
			task.State = responseItem.State
			task.SubmitTime = responseItem.SubmitTime
			task.StartTime = responseItem.StartTime
			task.FinishTime = responseItem.FinishTime
			task.ImageUrl = responseItem.ImageUrl
			task.Status = responseItem.Status
			task.FailReason = responseItem.FailReason
			if responseItem.Properties != nil {
				propertiesStr, _ := common.Marshal(responseItem.Properties)
				task.Properties = string(propertiesStr)
			}
			if responseItem.Buttons != nil {
				buttonStr, _ := common.Marshal(responseItem.Buttons)
				task.Buttons = string(buttonStr)
			}
			// 映射 VideoUrl
			task.VideoUrl = responseItem.VideoUrl

			// 映射 VideoUrls - 将数组序列化为 JSON 字符串
			if responseItem.VideoUrls != nil && len(responseItem.VideoUrls) > 0 {
				videoUrlsStr, err := common.Marshal(responseItem.VideoUrls)
				if err != nil {
					logger.LogError(ctx, fmt.Sprintf("序列化 VideoUrls 失败: %v", err))
					task.VideoUrls = "[]" // 失败时设置为空数组
				} else {
					task.VideoUrls = string(videoUrlsStr)
				}
			} else {
				task.VideoUrls = "" // 空值时清空字段
			}

			shouldReturnQuota := false
			if (task.Progress != "100%" && responseItem.FailReason != "") || (task.Progress == "100%" && task.Status == "FAILURE") {
				logger.LogInfo(ctx, task.MjId+" 构建失败，"+task.FailReason)
				task.Progress = "100%"
				if task.Quota != 0 {
					shouldReturnQuota = true
				}
			}
			won, err := task.UpdateWithStatus(preStatus)
			if err != nil {
				logger.LogError(ctx, "UpdateMidjourneyTask task error: "+err.Error())
			} else if won && shouldReturnQuota {
				err = model.IncreaseUserQuota(task.UserId, task.Quota, false)
				if err != nil {
					logger.LogError(ctx, "fail to increase user quota: "+err.Error())
				}
				model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
					UserId:    task.UserId,
					LogType:   model.LogTypeRefund,
					Content:   "",
					ChannelId: task.ChannelId,
					ModelName: service.CovertMjpActionToModelName(task.Action),
					Quota:     task.Quota,
					Other: map[string]interface{}{
						"task_id": task.MjId,
						"reason":  "构图失败",
					},
				})
			}
		}
	}
	if report != nil && (ctx == nil || ctx.Err() == nil) {
		report(totalChannels, totalChannels)
	}
	return summary
}

func checkMjTaskNeedUpdate(oldTask *model.Midjourney, newTask dto.MidjourneyDto) bool {
	if oldTask.Code != 1 {
		return true
	}
	if oldTask.Progress != newTask.Progress {
		return true
	}
	if oldTask.PromptEn != newTask.PromptEn {
		return true
	}
	if oldTask.State != newTask.State {
		return true
	}
	if oldTask.SubmitTime != newTask.SubmitTime {
		return true
	}
	if oldTask.StartTime != newTask.StartTime {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.ImageUrl != newTask.ImageUrl {
		return true
	}
	if oldTask.Status != newTask.Status {
		return true
	}
	if oldTask.FailReason != newTask.FailReason {
		return true
	}
	if oldTask.FinishTime != newTask.FinishTime {
		return true
	}
	if oldTask.Progress != "100%" && newTask.FailReason != "" {
		return true
	}
	// 检查 VideoUrl 是否需要更新
	if oldTask.VideoUrl != newTask.VideoUrl {
		return true
	}
	// 检查 VideoUrls 是否需要更新
	if newTask.VideoUrls != nil && len(newTask.VideoUrls) > 0 {
		newVideoUrlsStr, _ := common.Marshal(newTask.VideoUrls)
		if oldTask.VideoUrls != string(newVideoUrlsStr) {
			return true
		}
	} else if oldTask.VideoUrls != "" {
		// 如果新数据没有 VideoUrls 但旧数据有，需要更新（清空）
		return true
	}

	return false
}

func GetAllMidjourney(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	// 解析其他查询参数
	queryParams := model.TaskQueryParams{
		ChannelID:      c.Query("channel_id"),
		MjID:           c.Query("mj_id"),
		StartTimestamp: c.Query("start_timestamp"),
		EndTimestamp:   c.Query("end_timestamp"),
	}
	scopedUserIDs, scoped, err := model.GetScopedUserIDs(c.GetInt("id"), c.GetInt("role"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if scoped {
		queryParams.UserIDs = scopedUserIDs
	}

	fetchSize := pageInfo.GetEndIdx()
	items := model.GetAllTasks(0, fetchSize, queryParams)
	total := model.CountAllTasks(queryParams)
	generatedImageLogs, generatedImageTotal := model.GetAllGeneratedImageLogs(0, fetchSize, queryParams)
	generatedItems := generatedImageLogsToMidjourney(generatedImageLogs)
	items, skippedExistingItems := filterMidjourneyItemsDuplicatedByGeneratedLogs(items, generatedImageLogs)
	items = append(items, generatedItems...)
	total += generatedImageTotal - int64(skippedExistingItems)
	normalizeMidjourneyLogItems(items)
	sortMidjourneyBySubmitTime(items)
	items = sliceMidjourneyPage(items, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetUserMidjourney(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	userId := c.GetInt("id")

	queryParams := model.TaskQueryParams{
		MjID:           c.Query("mj_id"),
		StartTimestamp: c.Query("start_timestamp"),
		EndTimestamp:   c.Query("end_timestamp"),
	}

	fetchSize := pageInfo.GetEndIdx()
	items := model.GetAllUserTask(userId, 0, fetchSize, queryParams)
	total := model.CountAllUserTask(userId, queryParams)
	generatedImageLogs, generatedImageTotal := model.GetAllUserGeneratedImageLogs(userId, 0, fetchSize, queryParams)
	generatedItems := generatedImageLogsToMidjourney(generatedImageLogs)
	items, skippedExistingItems := filterMidjourneyItemsDuplicatedByGeneratedLogs(items, generatedImageLogs)
	items = append(items, generatedItems...)
	total += generatedImageTotal - int64(skippedExistingItems)
	normalizeMidjourneyLogItems(items)
	sortMidjourneyBySubmitTime(items)
	items = sliceMidjourneyPage(items, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func filterMidjourneyItemsDuplicatedByGeneratedLogs(items []*model.Midjourney, generatedLogs []*model.GeneratedImageLog) ([]*model.Midjourney, int) {
	if len(items) == 0 || len(generatedLogs) == 0 {
		return items, 0
	}

	generatedRequestIDs := make(map[string]bool, len(generatedLogs))
	for _, log := range generatedLogs {
		if log == nil || strings.TrimSpace(log.RequestId) == "" {
			continue
		}
		generatedRequestIDs[strings.TrimSpace(log.RequestId)] = true
	}
	if len(generatedRequestIDs) == 0 {
		return items, 0
	}

	filtered := make([]*model.Midjourney, 0, len(items))
	skipped := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		mjID := strings.TrimSpace(item.MjId)
		requestID := strings.TrimPrefix(mjID, "generated-image-")
		if requestID != mjID && generatedRequestIDs[requestID] {
			skipped++
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, skipped
}

func generatedImageLogsToMidjourney(logs []*model.GeneratedImageLog) []*model.Midjourney {
	items := make([]*model.Midjourney, 0, len(logs))
	for _, log := range logs {
		if log == nil {
			continue
		}
		imageURL := firstGeneratedImageURL(log.Other)
		startTime := log.CreatedAt - int64(log.UseTime)
		if startTime <= 0 || startTime > log.CreatedAt {
			startTime = log.CreatedAt
		}
		items = append(items, &model.Midjourney{
			Id:         log.Id,
			Code:       1,
			UserId:     log.UserId,
			Action:     "IMAGE_GENERATION",
			MjId:       fmt.Sprintf("generated-image-%d", log.Id),
			Prompt:     generatedImagePrompt(log.Other, log.Prompt),
			ModelName:  log.ModelName,
			SubmitTime: log.CreatedAt * 1000,
			StartTime:  startTime * 1000,
			FinishTime: log.CreatedAt * 1000,
			ImageUrl:   imageURL,
			ImageSize:  generatedImageSize(log.Other, log.Prompt),
			Status:     "SUCCESS",
			Progress:   "100%",
			ChannelId:  log.ChannelId,
			Quota:      log.Quota,
		})
	}
	return items
}

func generatedImagePrompt(other string, fallback string) string {
	otherMap, _ := common.StrToMap(other)
	if requestBody, ok := otherMap["request_body"].(map[string]interface{}); ok {
		if body, ok := requestBody["body"].(map[string]interface{}); ok {
			if prompt, ok := body["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
				return prompt
			}
		}
	}
	if prompt, ok := otherMap["prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		return prompt
	}
	if prompt, ok := otherMap["image_prompt"].(string); ok && strings.TrimSpace(prompt) != "" {
		return prompt
	}
	return fallback
}

func generatedImageSize(other string, content string) string {
	otherMap, _ := common.StrToMap(other)
	parts := make([]string, 0, 2)

	if images, ok := otherMap["generated_images"].([]interface{}); ok {
		for _, image := range images {
			imageMap, ok := image.(map[string]interface{})
			if !ok {
				continue
			}
			width, hasWidth := imageMap["width"]
			height, hasHeight := imageMap["height"]
			if hasWidth && hasHeight {
				parts = append(parts, fmt.Sprintf("%.0fx%.0f", generatedImageNumber(width), generatedImageNumber(height)))
			}
			if fileSize := generatedImageFileSize(imageMap["size"]); fileSize != "" {
				parts = append(parts, fileSize)
			}
			if len(parts) > 0 {
				return strings.Join(parts, " / ")
			}
		}
	}

	if size := generatedImageRequestedSize(other); size != "" {
		parts = append(parts, size)
	} else if size := extractImageSizeFromContent(content); size != "" {
		parts = append(parts, size)
	}
	return strings.Join(parts, " / ")
}

func generatedImageRequestedSize(other string) string {
	otherMap, _ := common.StrToMap(other)
	if requestBody, ok := otherMap["request_body"].(map[string]interface{}); ok {
		if body, ok := requestBody["body"].(map[string]interface{}); ok {
			if size, ok := body["size"].(string); ok && strings.TrimSpace(size) != "" {
				return strings.TrimSpace(size)
			}
		}
	}
	if size, ok := otherMap["image_size"].(string); ok && strings.TrimSpace(size) != "" {
		return strings.TrimSpace(size)
	}
	return ""
}

func extractImageSizeFromContent(content string) string {
	fields := strings.Fields(strings.ReplaceAll(content, ",", " "))
	for i, field := range fields {
		if strings.TrimSpace(field) == "大小" && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func generatedImageFileSize(value interface{}) string {
	size := generatedImageNumber(value)
	if size <= 0 {
		return ""
	}
	if size >= 1024*1024 {
		return fmt.Sprintf("%.1f MB", size/(1024*1024))
	}
	if size >= 1024 {
		return fmt.Sprintf("%.1f KB", size/1024)
	}
	return fmt.Sprintf("%.0f B", size)
}

func generatedImageNumber(value interface{}) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case uint:
		return float64(v)
	case uint64:
		return float64(v)
	case uint32:
		return float64(v)
	default:
		return 0
	}
}

func firstGeneratedImageURL(other string) string {
	otherMap, _ := common.StrToMap(other)
	images, ok := otherMap["generated_images"].([]interface{})
	if !ok {
		return ""
	}
	for _, image := range images {
		imageMap, ok := image.(map[string]interface{})
		if !ok {
			continue
		}
		if url, ok := imageMap["url"].(string); ok && url != "" {
			return url
		}
	}
	return ""
}

func normalizeMidjourneyLogItems(items []*model.Midjourney) {
	for i, midjourney := range items {
		if midjourney == nil {
			continue
		}
		if strings.TrimSpace(midjourney.ModelName) == "" {
			midjourney.ModelName = strings.TrimSpace(midjourney.PromptEn)
		}
		if setting.MjForwardUrlEnabled {
			midjourney.ImageUrl = midjourneyDisplayImageURL(midjourney.ImageUrl, midjourney.MjId)
		}
		items[i] = midjourney
	}
}

func sortMidjourneyBySubmitTime(items []*model.Midjourney) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].SubmitTime == items[j].SubmitTime {
			return items[i].Id > items[j].Id
		}
		return items[i].SubmitTime > items[j].SubmitTime
	})
}

func sliceMidjourneyPage(items []*model.Midjourney, startIdx int, pageSize int) []*model.Midjourney {
	if startIdx >= len(items) {
		return []*model.Midjourney{}
	}
	endIdx := startIdx + pageSize
	if endIdx > len(items) {
		endIdx = len(items)
	}
	return items[startIdx:endIdx]
}

func midjourneyDisplayImageURL(imageURL string, mjID string) string {
	if imageURL == "" {
		return ""
	}
	if strings.HasPrefix(imageURL, "/generated-images/") {
		return imageURL
	}
	return system_setting.ServerAddress + "/mj/image/" + mjID
}
