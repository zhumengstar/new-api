package model

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ModelRequestDailyStat struct {
	Date         string `json:"date" gorm:"type:varchar(10);primaryKey"`
	ModelName    string `json:"model_name" gorm:"type:varchar(191);primaryKey"`
	RequestCount int64  `json:"request_count" gorm:"not null;default:0"`
	UpdatedAt    int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (ModelRequestDailyStat) TableName() string {
	return "model_request_daily_stats"
}

type ModelRequestStatsCheckpoint struct {
	Id               int    `gorm:"primaryKey"`
	CompletedThrough string `gorm:"type:varchar(10);not null"`
	UpdatedAt        int64  `gorm:"autoUpdateTime"`
}

func (ModelRequestStatsCheckpoint) TableName() string {
	return "model_request_stats_checkpoints"
}

type ModelRequestStat struct {
	ModelName    string `json:"model_name"`
	RequestCount int64  `json:"request_count"`
}

var modelRequestStatsMu sync.Mutex

func GetModelRequestStats(period string) ([]ModelRequestStat, error) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	todayStats, err := aggregateModelRequests(today.Unix(), today.AddDate(0, 0, 1).Unix())
	if err != nil {
		return nil, err
	}
	if period == "today" {
		sortModelRequestStats(todayStats)
		return todayStats, nil
	}

	if err := persistCompletedModelRequestStats(today); err != nil {
		return nil, err
	}

	historical := make([]ModelRequestStat, 0)
	if err := DB.Model(&ModelRequestDailyStat{}).
		Select("model_name, COALESCE(SUM(request_count), 0) AS request_count").
		Group("model_name").
		Scan(&historical).Error; err != nil {
		return nil, err
	}

	counts := make(map[string]int64, len(historical)+len(todayStats))
	for _, stat := range historical {
		counts[stat.ModelName] += stat.RequestCount
	}
	for _, stat := range todayStats {
		counts[stat.ModelName] += stat.RequestCount
	}
	result := make([]ModelRequestStat, 0, len(counts))
	for modelName, count := range counts {
		result = append(result, ModelRequestStat{ModelName: modelName, RequestCount: count})
	}
	sortModelRequestStats(result)
	return result, nil
}

func persistCompletedModelRequestStats(today time.Time) error {
	modelRequestStatsMu.Lock()
	defer modelRequestStatsMu.Unlock()

	checkpoint := ModelRequestStatsCheckpoint{Id: 1}
	checkpointResult := DB.First(&checkpoint)
	if checkpointResult.Error != nil && !errors.Is(checkpointResult.Error, gorm.ErrRecordNotFound) {
		return checkpointResult.Error
	}

	var start time.Time
	if checkpointResult.Error == nil && checkpoint.CompletedThrough != "" {
		completedThrough, err := time.ParseInLocation("2006-01-02", checkpoint.CompletedThrough, today.Location())
		if err != nil {
			return err
		}
		start = completedThrough.AddDate(0, 0, 1)
	} else {
		var earliest int64
		if err := LOG_DB.Model(&Log{}).
			Select("COALESCE(MIN(created_at), 0)").
			Where("type = ? AND model_name <> ''", LogTypeConsume).
			Scan(&earliest).Error; err != nil {
			return err
		}
		if earliest == 0 {
			return nil
		}
		earliestTime := time.Unix(earliest, 0).In(today.Location())
		start = time.Date(earliestTime.Year(), earliestTime.Month(), earliestTime.Day(), 0, 0, 0, 0, today.Location())
	}
	if !start.Before(today) {
		return nil
	}

	rows, err := aggregateModelRequestsByDay(start.Unix(), today.Unix())
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if len(rows) > 0 {
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "date"}, {Name: "model_name"}},
				DoUpdates: clause.AssignmentColumns([]string{"request_count", "updated_at"}),
			}).CreateInBatches(rows, 500).Error; err != nil {
				return err
			}
		}
		checkpoint = ModelRequestStatsCheckpoint{
			Id:               1,
			CompletedThrough: today.AddDate(0, 0, -1).Format("2006-01-02"),
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"completed_through", "updated_at"}),
		}).Create(&checkpoint).Error
	})
}

func aggregateModelRequests(startUnix int64, endUnix int64) ([]ModelRequestStat, error) {
	rows := make([]ModelRequestStat, 0)
	err := LOG_DB.Table("logs").
		Select("model_name, COUNT(*) AS request_count").
		Where("type = ? AND created_at >= ? AND created_at < ? AND model_name <> ''", LogTypeConsume, startUnix, endUnix).
		Group("model_name").
		Scan(&rows).Error
	return rows, err
}

func aggregateModelRequestsByDay(startUnix int64, endUnix int64) ([]ModelRequestDailyStat, error) {
	dateExpr := "DATE_FORMAT(FROM_UNIXTIME(created_at), '%Y-%m-%d')"
	switch {
	case common.UsingLogDatabase(common.DatabaseTypePostgreSQL):
		dateExpr = "TO_CHAR(TO_TIMESTAMP(created_at) AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')"
	case common.UsingLogDatabase(common.DatabaseTypeSQLite):
		dateExpr = "strftime('%Y-%m-%d', created_at, 'unixepoch', '+8 hours')"
	}
	rows := make([]ModelRequestDailyStat, 0)
	err := LOG_DB.Table("logs").
		Select(dateExpr+" AS date, model_name, COUNT(*) AS request_count").
		Where("type = ? AND created_at >= ? AND created_at < ? AND model_name <> ''", LogTypeConsume, startUnix, endUnix).
		Group(dateExpr + ", model_name").
		Scan(&rows).Error
	return rows, err
}

func sortModelRequestStats(stats []ModelRequestStat) {
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].RequestCount == stats[j].RequestCount {
			return stats[i].ModelName < stats[j].ModelName
		}
		return stats[i].RequestCount > stats[j].RequestCount
	})
}
