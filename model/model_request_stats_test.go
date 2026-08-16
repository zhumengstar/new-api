package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetModelRequestStatsPersistsHistoryAndRefreshesToday(t *testing.T) {
	truncateTables(t)
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Now().In(location)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	yesterday := today.AddDate(0, 0, -1).Add(12 * time.Hour).Unix()
	todayTimestamp := today.Add(time.Hour).Unix()

	historical := []*Log{
		{Type: LogTypeConsume, ModelName: "alpha", CreatedAt: yesterday},
		{Type: LogTypeConsume, ModelName: "alpha", CreatedAt: yesterday + 1},
		{Type: LogTypeConsume, ModelName: "beta", CreatedAt: yesterday + 2},
		{Type: LogTypeError, ModelName: "ignored-error", CreatedAt: yesterday + 3},
	}
	require.NoError(t, LOG_DB.Create(&historical).Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ModelName: "beta", CreatedAt: todayTimestamp}).Error)

	total, err := GetModelRequestStats("total")
	require.NoError(t, err)
	require.Len(t, total, 2)
	assert.Equal(t, ModelRequestStat{ModelName: "alpha", RequestCount: 2}, total[0])
	assert.Equal(t, ModelRequestStat{ModelName: "beta", RequestCount: 2}, total[1])

	// Completed dates stay fixed even if the source log table changes later.
	require.NoError(t, LOG_DB.Model(historical[0]).Update("model_name", "beta").Error)
	require.NoError(t, LOG_DB.Create(&Log{Type: LogTypeConsume, ModelName: "beta", CreatedAt: todayTimestamp + 1}).Error)
	total, err = GetModelRequestStats("total")
	require.NoError(t, err)
	assert.Equal(t, ModelRequestStat{ModelName: "beta", RequestCount: 3}, total[0])
	assert.Equal(t, ModelRequestStat{ModelName: "alpha", RequestCount: 2}, total[1])

	todayOnly, err := GetModelRequestStats("today")
	require.NoError(t, err)
	require.Len(t, todayOnly, 1)
	assert.Equal(t, ModelRequestStat{ModelName: "beta", RequestCount: 2}, todayOnly[0])
}
