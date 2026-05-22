package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelSelectTestDB(t *testing.T) {
	t.Helper()
	oldDB := model.DB
	oldLogDB := model.LOG_DB
	oldUsingSQLite := common.UsingSQLite
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	oldRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		model.LOG_DB = oldLogDB
		common.UsingSQLite = oldUsingSQLite
		common.MemoryCacheEnabled = oldMemoryCacheEnabled
		common.RedisEnabled = oldRedisEnabled
	})

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	model.DB = db
	model.LOG_DB = db
	common.UsingSQLite = true
	model.InitColumnNamesForTest()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
}

func seedSelectableChannelWithStatus(t *testing.T, id int, priority int64, weight uint, status int) {
	t.Helper()
	autoBan := 1
	channel := &model.Channel{
		Id:       id,
		Name:     fmt.Sprintf("channel-%d", id),
		Key:      fmt.Sprintf("sk-%d", id),
		Status:   status,
		Models:   "gpt-test",
		Group:    "default",
		Priority: &priority,
		Weight:   &weight,
		AutoBan:  &autoBan,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-test",
		ChannelId: id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)
}

func seedSelectableChannel(t *testing.T, id int, priority int64, weight uint) {
	t.Helper()
	seedSelectableChannelWithStatus(t, id, priority, weight, common.ChannelStatusEnabled)
}

func TestCacheGetRandomSatisfiedChannelExcludesFailedChannelsBeforeFallingBackPriority(t *testing.T) {
	setupChannelSelectTestDB(t)
	seedSelectableChannel(t, 1, 100, 100)
	seedSelectableChannel(t, 2, 100, 100)
	seedSelectableChannel(t, 3, 50, 100)

	param := &RetryParam{
		Ctx:               &gin.Context{},
		TokenGroup:        "default",
		ModelName:         "gpt-test",
		Retry:             common.GetPointer(1),
		TriedChannelIds:   map[int]bool{1: true},
		ExhaustedPriority: map[int]bool{},
	}

	channel, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.Equal(t, "default", group)
	require.NotNil(t, channel)
	require.Equal(t, 2, channel.Id, "retry should try remaining channel at the same highest priority before lower priority")
	require.Equal(t, 0, param.GetRetry(), "same-priority retry should keep the retry index at the highest priority tier")
}

func TestCacheGetRandomSatisfiedChannelFallsBackToNextPriorityWhenPriorityExhausted(t *testing.T) {
	setupChannelSelectTestDB(t)
	seedSelectableChannel(t, 1, 100, 100)
	seedSelectableChannel(t, 2, 100, 100)
	seedSelectableChannel(t, 3, 50, 100)

	param := &RetryParam{
		Ctx:               &gin.Context{},
		TokenGroup:        "default",
		ModelName:         "gpt-test",
		Retry:             common.GetPointer(0),
		TriedChannelIds:   map[int]bool{1: true, 2: true},
		ExhaustedPriority: map[int]bool{},
	}

	channel, group, err := CacheGetRandomSatisfiedChannel(param)
	require.NoError(t, err)
	require.Equal(t, "default", group)
	require.NotNil(t, channel)
	require.Equal(t, 3, channel.Id, "after all highest-priority channels failed, retry should descend to the next priority")
	require.Equal(t, 1, param.GetRetry())
}

func TestCacheGetRandomSatisfiedChannelFallsBackToHighestPriorityAfterAutoReenable(t *testing.T) {
	setupChannelSelectTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = oldMemoryCacheEnabled })

	seedSelectableChannel(t, 1, 100, 100)
	seedSelectableChannel(t, 2, 50, 100)
	model.InitChannelCache()

	model.CacheUpdateChannelStatus(1, common.ChannelStatusAutoDisabled)
	fallback, err := model.GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, fallback)
	require.Equal(t, 2, fallback.Id)

	model.CacheUpdateChannelStatus(1, common.ChannelStatusEnabled)
	recovered, err := model.GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.Equal(t, 1, recovered.Id, "after automatic health check reenables a higher-priority channel, retry=0 should immediately route back to it")
}

func TestCacheUpdateChannelStatusReinsertsChannelDisabledAtCacheInit(t *testing.T) {
	setupChannelSelectTestDB(t)
	oldMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = oldMemoryCacheEnabled })

	seedSelectableChannelWithStatus(t, 1, 100, 100, common.ChannelStatusAutoDisabled)
	model.InitChannelCache()

	missing, err := model.GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.Nil(t, missing)

	model.CacheUpdateChannelStatus(1, common.ChannelStatusEnabled)
	recovered, err := model.GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, recovered)
	require.Equal(t, 1, recovered.Id, "a channel disabled when cache was initialized must be inserted after automatic recovery")

	model.CacheUpdateChannelStatus(1, common.ChannelStatusEnabled)
	recoveredAgain, err := model.GetRandomSatisfiedChannel("default", "gpt-test", 0)
	require.NoError(t, err)
	require.NotNil(t, recoveredAgain)
	require.Equal(t, 1, recoveredAgain.Id, "repeated enable updates must not duplicate the channel in cache")
}

func TestGetRandomSatisfiedChannelWithExclusionNilParam(t *testing.T) {
	setupChannelSelectTestDB(t)
	seedSelectableChannel(t, 1, 100, 100)

	channel, err := getRandomSatisfiedChannelWithExclusion("default", "gpt-test", nil)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 1, channel.Id)
}
