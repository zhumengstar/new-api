package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertUserForManagementVisibilityTest(t *testing.T, user *User) {
	t.Helper()
	if user.AffCode == "" {
		user.AffCode = user.Username + "_aff"
	}
	require.NoError(t, DB.Create(user).Error)
}

func TestGetAllUsersHidesInactiveCommonUsersOnly(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	cutoffOlder := now - int64((userManagementInactiveHiddenDays+1)*24*60*60)
	recent := now - int64(24*60*60)

	insertUserForManagementVisibilityTest(t, &User{
		Id:          1,
		Username:    "recent_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   recent,
		LastLoginAt: recent,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          2,
		Username:    "inactive_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   cutoffOlder,
		LastLoginAt: cutoffOlder,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          3,
		Username:    "never_login_old",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   cutoffOlder,
		LastLoginAt: 0,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          4,
		Username:    "inactive_admin",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   cutoffOlder,
		LastLoginAt: cutoffOlder,
	})

	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}
	users, total, err := GetAllUsers(pageInfo, "", "")

	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, user.Username)
	}
	assert.ElementsMatch(t, []string{"recent_user", "inactive_admin"}, names)
}

func TestSearchUsersCanFindInactiveCommonUsers(t *testing.T) {
	truncateTables(t)
	old := common.GetTimestamp() - int64((userManagementInactiveHiddenDays+1)*24*60*60)
	insertUserForManagementVisibilityTest(t, &User{
		Id:          10,
		Username:    "inactive_search_target",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   old,
		LastLoginAt: old,
	})

	users, total, err := SearchUsers("inactive_search_target", "", nil, nil, 0, 20, "", "")

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	assert.Equal(t, "inactive_search_target", users[0].Username)
}
