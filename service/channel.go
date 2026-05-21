package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

const (
	channelConsecutiveErrorCountKey = "consecutive_error_count"
	channelConsecutiveErrorLastKey  = "consecutive_error_last"
	channelConsecutiveErrorLimit    = 10
)

func getChannelConsecutiveErrorCount(otherInfo map[string]interface{}) int {
	if otherInfo == nil {
		return 0
	}
	switch v := otherInfo[channelConsecutiveErrorCountKey].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		count, _ := v.Int64()
		return int(count)
	default:
		return 0
	}
}

func RecordChannelFailureAndMaybeDisable(channelError types.ChannelError, err *types.NewAPIError) bool {
	if !channelError.AutoBan || !ShouldDisableChannel(err) {
		return false
	}

	channel, getErr := model.GetChannelById(channelError.ChannelId, true)
	if getErr != nil {
		common.SysLog(fmt.Sprintf("failed to record channel consecutive error: channel_id=%d, error=%v", channelError.ChannelId, getErr))
		return false
	}

	info := channel.GetOtherInfo()
	count := getChannelConsecutiveErrorCount(info) + 1
	info[channelConsecutiveErrorCountKey] = count
	info[channelConsecutiveErrorLastKey] = err.ErrorWithStatusCode()
	channel.SetOtherInfo(info)
	if saveErr := channel.SaveWithoutKey(); saveErr != nil {
		common.SysLog(fmt.Sprintf("failed to save channel consecutive error count: channel_id=%d, count=%d, error=%v", channelError.ChannelId, count, saveErr))
		return false
	}

	if count < channelConsecutiveErrorLimit {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）连续错误 %d/%d，暂不禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, count, channelConsecutiveErrorLimit, err.ErrorWithStatusCode()))
		return false
	}

	DisableChannel(channelError, fmt.Sprintf("连续错误 %d 次，最后错误：%s", count, err.ErrorWithStatusCode()))
	return true
}

func ClearChannelConsecutiveErrors(channelId int) {
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return
	}
	info := channel.GetOtherInfo()
	if _, ok := info[channelConsecutiveErrorCountKey]; !ok {
		if _, ok := info[channelConsecutiveErrorLastKey]; !ok {
			return
		}
	}
	delete(info, channelConsecutiveErrorCountKey)
	delete(info, channelConsecutiveErrorLastKey)
	channel.SetOtherInfo(info)
	if saveErr := channel.SaveWithoutKey(); saveErr != nil {
		common.SysLog(fmt.Sprintf("failed to clear channel consecutive error count: channel_id=%d, error=%v", channelId, saveErr))
	}
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	return search
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
