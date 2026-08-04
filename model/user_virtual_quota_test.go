package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetUserVirtualQuotaRespectsAdminAvailableQuota(t *testing.T) {
	truncateTables(t)
	admin := &User{Id: 1001, Username: "virtual_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 1000, AffCode: "virtual_admin_aff"}
	childA := &User{Id: 1002, Username: "virtual_child_a", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, AffCode: "virtual_child_a_aff"}
	childB := &User{Id: 1003, Username: "virtual_child_b", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, AffCode: "virtual_child_b_aff"}
	require.NoError(t, DB.Create(admin).Error)
	require.NoError(t, DB.Create(childA).Error)
	require.NoError(t, DB.Create(childB).Error)

	require.NoError(t, SetUserVirtualQuota(admin.Id, childA.Id, 700))
	err := SetUserVirtualQuota(admin.Id, childB.Id, 400)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds admin available quota")
}

func TestSetUserVirtualQuotaCannotDropBelowUsedQuota(t *testing.T) {
	truncateTables(t)
	admin := &User{Id: 1011, Username: "used_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 1000, AffCode: "used_admin_aff"}
	child := &User{Id: 1012, Username: "used_child", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, AffCode: "used_child_aff"}
	require.NoError(t, DB.Create(admin).Error)
	require.NoError(t, DB.Create(child).Error)
	require.NoError(t, SetUserVirtualQuota(admin.Id, child.Id, 800))
	require.NoError(t, ConsumeVirtualQuota(admin.Id, child.Id, 300, 150))

	err := SetUserVirtualQuota(admin.Id, child.Id, 200)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be less than used quota")
}

func TestConsumeVirtualQuotaDeductsVirtualAndAdminActualQuota(t *testing.T) {
	truncateTables(t)
	admin := &User{Id: 1021, Username: "consume_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 1000, AffCode: "consume_admin_aff"}
	child := &User{Id: 1022, Username: "consume_child", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, AffCode: "consume_child_aff"}
	require.NoError(t, DB.Create(admin).Error)
	require.NoError(t, DB.Create(child).Error)
	require.NoError(t, SetUserVirtualQuota(admin.Id, child.Id, 800))

	require.NoError(t, ConsumeVirtualQuota(admin.Id, child.Id, 300, 150))

	virtualQuota, err := GetUserVirtualQuota(child.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, virtualQuota.UsedQuota)
	assert.Equal(t, 500, virtualQuota.RemainingQuota())

	adminAfter, err := GetUserById(admin.Id, true)
	require.NoError(t, err)
	assert.Equal(t, 850, adminAfter.Quota)
	assert.Equal(t, 150, adminAfter.UsedQuota)
}

func TestEditManagedUpdatesVirtualRemainingQuotaWithGroupAndSetting(t *testing.T) {
	truncateTables(t)
	admin := &User{Id: 1031, Username: "edit_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 1000, AffCode: "edit_admin_aff"}
	child := &User{Id: 1032, Username: "edit_child", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, Group: "default", AffCode: "edit_child_aff"}
	require.NoError(t, DB.Create(admin).Error)
	require.NoError(t, DB.Create(child).Error)
	require.NoError(t, SetUserVirtualQuota(admin.Id, child.Id, 800))
	require.NoError(t, ConsumeVirtualQuota(admin.Id, child.Id, 300, 150))

	updated := *child
	updated.Group = "default,private-group"
	setting := dto.UserSetting{UserGroupRatios: map[string]float64{"private-group": 1.5}}
	remainingQuota := 400
	require.NoError(t, updated.EditManaged(false, &setting, &remainingQuota, admin.Id))

	var actual User
	require.NoError(t, DB.First(&actual, child.Id).Error)
	assert.Equal(t, "default,private-group", actual.Group)
	assert.Equal(t, 1.5, actual.GetSetting().UserGroupRatios["private-group"])
	virtualQuota, err := GetUserVirtualQuota(child.Id)
	require.NoError(t, err)
	assert.Equal(t, 300, virtualQuota.UsedQuota)
	assert.Equal(t, 400, virtualQuota.RemainingQuota())
	assert.Equal(t, 700, virtualQuota.Quota)
}

func TestEditManagedRollsBackFieldsWhenVirtualQuotaFails(t *testing.T) {
	truncateTables(t)
	admin := &User{Id: 1041, Username: "rollback_admin", Role: common.RoleAdminUser, Status: common.UserStatusEnabled, Quota: 100, AffCode: "rollback_admin_aff"}
	childA := &User{Id: 1042, Username: "rollback_child_a", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, Group: "default", Setting: "{}", AffCode: "rollback_child_a_aff"}
	childB := &User{Id: 1043, Username: "rollback_child_b", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: admin.Id, Group: "default", AffCode: "rollback_child_b_aff"}
	require.NoError(t, DB.Create(admin).Error)
	require.NoError(t, DB.Create(childA).Error)
	require.NoError(t, DB.Create(childB).Error)
	require.NoError(t, SetUserVirtualQuota(admin.Id, childB.Id, 100))

	updated := *childA
	updated.Group = "default,private-group"
	setting := dto.UserSetting{UserGroupRatios: map[string]float64{"private-group": 2}}
	remainingQuota := 1
	err := updated.EditManaged(false, &setting, &remainingQuota, admin.Id)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds admin available quota")

	var actual User
	require.NoError(t, DB.First(&actual, childA.Id).Error)
	assert.Equal(t, "default", actual.Group)
	assert.Equal(t, "{}", actual.Setting)
	_, err = GetUserVirtualQuota(childA.Id)
	require.Error(t, err)
}
