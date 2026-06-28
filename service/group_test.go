package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseUserGroups(t *testing.T) {
	got := ParseUserGroups(" default, vip ,default,, svip ")
	want := []string{"default", "vip", "svip"}
	require.Equal(t, want, got)
}

func TestJoinUserGroups(t *testing.T) {
	got := JoinUserGroups([]string{" default ", "vip", "", "default", " svip "})
	want := "default,vip,svip"
	require.Equal(t, want, got)
}

func TestGetUserUsableGroupsIncludesMultipleUserGroups(t *testing.T) {
	groups := GetUserUsableGroups("default,vip")
	require.Contains(t, groups, "default")
	require.Contains(t, groups, "vip")
	require.Contains(t, groups, "append_1")
}
