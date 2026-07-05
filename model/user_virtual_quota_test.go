package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
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
