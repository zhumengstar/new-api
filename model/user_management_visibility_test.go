package model

import (
	"testing"
	"time"

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

func TestSearchUsersFuzzyMatchesManagementFields(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{
		Id:              73142,
		Username:        "searchable_user",
		DisplayName:     "Visible Name",
		Email:           "search@example.com",
		Role:            common.RoleCommonUser,
		Status:          common.UserStatusEnabled,
		Quota:           987654,
		Group:           "priority-group",
		Remark:          "customer-note",
		WeChatContact:   "wechat-search-value",
		QQContact:       "qq-search-value",
		RequestCount:    24680,
		AffHistoryQuota: 13579,
	})

	keywords := []string{
		"314", "VISIBLE", "example.com", "priority", "customer-note",
		"wechat-search", "qq-search", "7654", "4680", "3579",
	}
	for _, keyword := range keywords {
		users, total, err := SearchUsers(keyword, "", nil, nil, 0, 100, "", "", 1, common.RoleRootUser)
		require.NoError(t, err, keyword)
		assert.Equal(t, int64(1), total, keyword)
		require.Len(t, users, 1, keyword)
		assert.Equal(t, "searchable_user", users[0].Username, keyword)
	}
}

func TestAdminSearchCannotProbeRootOnlyContacts(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{
		Id:            81,
		Username:      "contact_owner",
		Role:          common.RoleCommonUser,
		Status:        common.UserStatusEnabled,
		WeChatContact: "private-wechat-value",
		QQContact:     "private-qq-value",
	})

	for _, keyword := range []string{"private-wechat", "private-qq"} {
		users, total, err := SearchUsers(keyword, "", nil, nil, 0, 100, "", "", 20, common.RoleAdminUser)
		require.NoError(t, err, keyword)
		assert.Zero(t, total, keyword)
		assert.Empty(t, users, keyword)
	}
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

func TestGetUserOrderSupportsTotalConsumedQuota(t *testing.T) {
	assert.Equal(t, DefaultUserOrder, GetUserOrder("total_consumed_quota", "asc"))
	assert.Equal(t, DefaultUserOrder, GetUserOrder("today_consumed_quota", "desc"))
}

func TestGetRecentDailyIncomeStatsExcludesAdmins(t *testing.T) {
	truncateTables(t)
	now := time.Now()
	insertUserForManagementVisibilityTest(t, &User{Id: 101, Username: "income_user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	insertUserForManagementVisibilityTest(t, &User{Id: 102, Username: "income_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled})
	require.NoError(t, LOG_DB.Create(&Log{UserId: 101, Type: LogTypeConsume, Quota: 125000, CreatedAt: now.Unix()}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 102, Type: LogTypeConsume, Quota: 990000, CreatedAt: now.Unix()}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 101, Type: LogTypeTopup, Quota: 880000, CreatedAt: now.Unix()}).Error)

	stats, err := GetRecentDailyIncomeStats(7)
	require.NoError(t, err)
	require.Len(t, stats, 7)
	assert.Equal(t, now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02"), stats[6].Date)
	assert.Equal(t, int64(125000), stats[6].Quota)
}

func TestGetAllUsersIncludesTotalConsumedQuota(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{Id: 201, Username: "total_consumed_user", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	require.NoError(t, LOG_DB.Create(&Log{UserId: 201, Type: LogTypeConsume, Quota: 125000}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 201, Type: LogTypeConsume, Quota: 250000}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 201, Type: LogTypeTopup, Quota: 990000}).Error)

	users, _, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, "", "", 1, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Equal(t, int64(375000), users[0].TotalConsumedQuota)
}

func TestGetAllUsersSortsByTotalConsumedQuota(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{Id: 301, Username: "lower_consumed", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	insertUserForManagementVisibilityTest(t, &User{Id: 302, Username: "higher_consumed", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	require.NoError(t, LOG_DB.Create(&Log{UserId: 301, Type: LogTypeConsume, Quota: 125000}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 302, Type: LogTypeConsume, Quota: 250000}).Error)

	users, _, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, "total_consumed_quota", "desc", 1, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "higher_consumed", users[0].Username)
	assert.Equal(t, common.UserStatusEnabled, users[0].Status)
	assert.Equal(t, "lower_consumed", users[1].Username)
}

func TestGetAllUsersIncludesAndSortsByTodayConsumedQuota(t *testing.T) {
	truncateTables(t)
	now := time.Now().Unix()
	yesterday := shanghaiTodayStartUnix() - 1
	insertUserForManagementVisibilityTest(t, &User{Id: 311, Username: "higher_historical", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	insertUserForManagementVisibilityTest(t, &User{Id: 312, Username: "higher_today", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	require.NoError(t, LOG_DB.Create(&Log{UserId: 311, Type: LogTypeConsume, Quota: 900000, CreatedAt: yesterday}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 311, Type: LogTypeConsume, Quota: 125000, CreatedAt: now}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 312, Type: LogTypeConsume, Quota: 250000, CreatedAt: now}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 312, Type: LogTypeTopup, Quota: 990000, CreatedAt: now}).Error)

	users, _, err := GetAllUsers(&common.PageInfo{Page: 1, PageSize: 20}, "today_consumed_quota", "desc", 1, common.RoleRootUser)
	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, "higher_today", users[0].Username)
	assert.Equal(t, int64(250000), users[0].TodayConsumedQuota)
	assert.Equal(t, int64(250000), users[0].TotalConsumedQuota)
	assert.Equal(t, "higher_historical", users[1].Username)
	assert.Equal(t, int64(125000), users[1].TodayConsumedQuota)
	assert.Equal(t, int64(1025000), users[1].TotalConsumedQuota)
}

func TestGetUserConsumptionStatsExcludesAdminsFromTotal(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{Id: 401, Username: "total_common", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	insertUserForManagementVisibilityTest(t, &User{Id: 402, Username: "total_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled})
	require.NoError(t, LOG_DB.Create(&Log{UserId: 401, Type: LogTypeConsume, Quota: 125000}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 402, Type: LogTypeConsume, Quota: 990000}).Error)

	stats, err := GetUserConsumptionStats(7)
	require.NoError(t, err)
	assert.Equal(t, int64(125000), stats.TotalQuota)
	assert.Equal(t, int64(125000), stats.TodayQuota)
}

func TestSumCurrentMinuteIncomeExcludesAdminsAndOldLogs(t *testing.T) {
	truncateTables(t)
	insertUserForManagementVisibilityTest(t, &User{Id: 501, Username: "minute_common", Role: common.RoleCommonUser, Status: common.UserStatusEnabled})
	insertUserForManagementVisibilityTest(t, &User{Id: 502, Username: "minute_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled})
	now := time.Now().Unix()
	require.NoError(t, LOG_DB.Create(&Log{UserId: 501, Type: LogTypeConsume, Quota: 125000, CreatedAt: now}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 502, Type: LogTypeConsume, Quota: 990000, CreatedAt: now}).Error)
	require.NoError(t, LOG_DB.Create(&Log{UserId: 501, Type: LogTypeConsume, Quota: 250000, CreatedAt: now - 61}).Error)

	quota, err := SumCurrentMinuteIncome(nil, false)
	require.NoError(t, err)
	assert.Equal(t, int64(125000), quota)
}
