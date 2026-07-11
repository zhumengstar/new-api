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

func TestGetAllUsersShowsInactiveCommonUsers(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()
	cutoffOlder := now - int64(6*24*60*60)
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
	users, total, err := GetAllUsers(pageInfo, "", "", 1, common.RoleRootUser)

	require.NoError(t, err)
	assert.Equal(t, int64(4), total)
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, user.Username)
	}
	assert.ElementsMatch(t, []string{"recent_user", "inactive_user", "never_login_old", "inactive_admin"}, names)
}

func TestSearchUsersCanFindInactiveCommonUsers(t *testing.T) {
	truncateTables(t)
	old := common.GetTimestamp() - int64(6*24*60*60)
	insertUserForManagementVisibilityTest(t, &User{
		Id:          10,
		Username:    "inactive_search_target",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		CreatedAt:   old,
		LastLoginAt: old,
	})

	users, total, err := SearchUsers("inactive_search_target", "", nil, nil, 0, 20, "", "", 1, common.RoleRootUser)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	assert.Equal(t, "inactive_search_target", users[0].Username)
}

func TestAdminUserManagementShowsAllNonHiddenUsers(t *testing.T) {
	truncateTables(t)
	now := common.GetTimestamp()

	insertUserForManagementVisibilityTest(t, &User{
		Id:          20,
		Username:    "admin_owner",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Quota:       900,
		CreatedAt:   now,
		LastLoginAt: now,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          23,
		Username:    "hidden_user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   99,
		IsHidden:    true,
		CreatedAt:   now,
		LastLoginAt: now,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          21,
		Username:    "invited_by_admin",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   20,
		CreatedAt:   now,
		LastLoginAt: now,
	})
	insertUserForManagementVisibilityTest(t, &User{
		Id:          22,
		Username:    "not_invited_by_admin",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		InviterId:   99,
		CreatedAt:   now,
		LastLoginAt: now,
	})

	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}
	users, total, err := GetAllUsers(pageInfo, "", "", 20, common.RoleAdminUser)

	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	names := make([]string, 0, len(users))
	adminQuota := 0
	for _, user := range users {
		names = append(names, user.Username)
		if user.Id == 20 {
			adminQuota = user.Quota
		}
	}
	assert.ElementsMatch(t, []string{"admin_owner", "invited_by_admin", "not_invited_by_admin"}, names)
	assert.Equal(t, 900, adminQuota)

	searchUsers, searchTotal, err := SearchUsers("not_invited_by_admin", "", nil, nil, 0, 20, "", "", 20, common.RoleAdminUser)
	require.NoError(t, err)
	assert.Equal(t, int64(1), searchTotal)
	require.Len(t, searchUsers, 1)
	assert.Equal(t, "not_invited_by_admin", searchUsers[0].Username)

	hiddenUsers, hiddenTotal, err := SearchUsers("hidden_user", "", nil, nil, 0, 20, "", "", 20, common.RoleAdminUser)
	require.NoError(t, err)
	assert.Equal(t, int64(0), hiddenTotal)
	assert.Empty(t, hiddenUsers)

	selfSearchUsers, selfSearchTotal, err := SearchUsers("admin_owner", "", nil, nil, 0, 20, "", "", 20, common.RoleAdminUser)
	require.NoError(t, err)
	assert.Equal(t, int64(1), selfSearchTotal)
	require.Len(t, selfSearchUsers, 1)
	assert.Equal(t, "admin_owner", selfSearchUsers[0].Username)
}

func TestGetUserOrderSupportsQuota(t *testing.T) {
	assert.Equal(t, "quota asc, id desc", GetUserOrder("quota", "asc"))
	assert.Equal(t, "quota desc, id desc", GetUserOrder("quota", "desc"))
}
