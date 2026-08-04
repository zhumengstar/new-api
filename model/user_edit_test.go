package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserEditPersistsAssignedPrivateGroup(t *testing.T) {
	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})

	db, err := gorm.Open(
		sqlite.Open(fmt.Sprintf("file:user-edit-%d?mode=memory&cache=shared", time.Now().UnixNano())),
		&gorm.Config{},
	)
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}))
	DB = db
	common.RedisEnabled = false

	stored := User{
		Username: "group-edit-user",
		Password: "stored-password-hash",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    100,
	}
	require.NoError(t, DB.Create(&stored).Error)

	updated := stored
	updated.Group = "default,private-group"
	updated.Remark = "private access"
	require.NoError(t, updated.Edit(false))

	var actual User
	require.NoError(t, DB.First(&actual, stored.Id).Error)
	require.Equal(t, "default,private-group", actual.Group)
	require.Equal(t, "private access", actual.Remark)
	require.Equal(t, 100, actual.Quota)
	require.Equal(t, "stored-password-hash", actual.Password)
}

func TestUserEditManagedPersistsGroupSettingAndActualQuotaTogether(t *testing.T) {
	truncateTables(t)
	stored := User{
		Username: "managed-root-edit",
		Password: "stored-password-hash",
		Status:   common.UserStatusEnabled,
		Role:     common.RoleCommonUser,
		Group:    "default",
		Quota:    100,
		AffCode:  "managed-root-edit-aff",
	}
	require.NoError(t, DB.Create(&stored).Error)

	updated := stored
	updated.Group = "default,private-group"
	setting := dto.UserSetting{
		UserGroupRatios: map[string]float64{"private-group": 1.25},
	}
	quota := 250
	require.NoError(t, updated.EditManaged(false, &setting, &quota, 0))

	var actual User
	require.NoError(t, DB.First(&actual, stored.Id).Error)
	require.Equal(t, "default,private-group", actual.Group)
	require.Equal(t, 250, actual.Quota)
	require.Equal(t, 1.25, actual.GetSetting().UserGroupRatios["private-group"])
}
