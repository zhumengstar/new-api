package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupChannelHealthTestDB(t *testing.T) {
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
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}))
}

func seedHealthChannel(t *testing.T, status int) *model.Channel {
	t.Helper()
	autoBan := 1
	channel := &model.Channel{
		Id:      1,
		Name:    "health-test-channel",
		Key:     "sk-test",
		Status:  status,
		AutoBan: &autoBan,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	return channel
}

func reloadHealthChannel(t *testing.T) *model.Channel {
	t.Helper()
	channel, err := model.GetChannelById(1, true)
	require.NoError(t, err)
	return channel
}

func TestChannelFailureRecorderDisablesOnlyAfterThreeConsecutiveErrors(t *testing.T) {
	setupChannelHealthTestDB(t)
	seedHealthChannel(t, common.ChannelStatusEnabled)

	oldDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = oldDisable })

	channelError := *types.NewChannelError(1, 1, "health-test-channel", false, "sk-test", true)
	apiErr := types.NewError(fmt.Errorf("channel unavailable"), types.ErrorCodeChannelNoAvailableKey)

	for i := 1; i <= 2; i++ {
		assert.False(t, RecordChannelFailureAndMaybeDisable(channelError, apiErr), "attempt %d should not disable", i)
		channel := reloadHealthChannel(t)
		assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
		assert.EqualValues(t, i, channel.GetOtherInfo()["consecutive_error_count"])
	}

	assert.True(t, RecordChannelFailureAndMaybeDisable(channelError, apiErr), "3rd consecutive error should disable")
	channel := reloadHealthChannel(t)
	assert.Equal(t, common.ChannelStatusAutoDisabled, channel.Status)
	assert.EqualValues(t, 3, channel.GetOtherInfo()["consecutive_error_count"])
}

func TestChannelFailureRecorderClearsCountOnSuccess(t *testing.T) {
	setupChannelHealthTestDB(t)
	seedHealthChannel(t, common.ChannelStatusEnabled)

	oldDisable := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = oldDisable })

	channelError := *types.NewChannelError(1, 1, "health-test-channel", false, "sk-test", true)
	apiErr := types.NewError(fmt.Errorf("channel unavailable"), types.ErrorCodeChannelNoAvailableKey)

	for i := 0; i < 2; i++ {
		RecordChannelFailureAndMaybeDisable(channelError, apiErr)
	}
	ClearChannelConsecutiveErrors(1)

	channel := reloadHealthChannel(t)
	_, exists := channel.GetOtherInfo()["consecutive_error_count"]
	assert.False(t, exists)
	assert.Equal(t, common.ChannelStatusEnabled, channel.Status)
}

func TestShouldEnableChannelOnlyAutoDisabledOnSuccessfulCheck(t *testing.T) {
	old := common.AutomaticEnableChannelEnabled
	common.AutomaticEnableChannelEnabled = true
	t.Cleanup(func() { common.AutomaticEnableChannelEnabled = old })

	assert.True(t, ShouldEnableChannel(nil, common.ChannelStatusAutoDisabled))
	assert.False(t, ShouldEnableChannel(nil, common.ChannelStatusManuallyDisabled))
	assert.False(t, ShouldEnableChannel(types.NewOpenAIError(fmt.Errorf("failed"), types.ErrorCodeBadResponse, 500), common.ChannelStatusAutoDisabled))
}
