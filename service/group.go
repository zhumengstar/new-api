package service

import (
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	userGroups := ParseUserGroups(userGroup)
	if len(userGroups) > 0 {
		for _, singleUserGroup := range userGroups {
			specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(singleUserGroup)
			if !b {
				continue
			}
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		for _, singleUserGroup := range userGroups {
			// 如果userGroup不在UserUsableGroups中，返回UserUsableGroups + userGroup
			if _, ok := groupsCopy[singleUserGroup]; !ok {
				groupsCopy[singleUserGroup] = "用户分组"
			}
		}
	}
	return groupsCopy
}

func ParseUserGroups(userGroup string) []string {
	userGroup = strings.TrimSpace(userGroup)
	if userGroup == "" {
		return nil
	}
	seen := make(map[string]struct{})
	groups := make([]string, 0)
	for _, group := range strings.Split(userGroup, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	return groups
}

func JoinUserGroups(groups []string) string {
	seen := make(map[string]struct{})
	normalized := make([]string, 0, len(groups))
	for _, group := range groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		normalized = append(normalized, group)
	}
	return strings.Join(normalized, ",")
}

func GetPrimaryUserGroup(userGroup string) string {
	groups := ParseUserGroups(userGroup)
	if len(groups) == 0 {
		return ""
	}
	return groups[0]
}

func GetUserGroupRatio(userGroup, group string) float64 {
	var selectedRatio float64
	hasSelectedRatio := false
	for _, singleUserGroup := range ParseUserGroups(userGroup) {
		ratio, ok := ratio_setting.GetGroupGroupRatio(singleUserGroup, group)
		if ok && (!hasSelectedRatio || ratio < selectedRatio) {
			selectedRatio = ratio
			hasSelectedRatio = true
		}
	}
	if hasSelectedRatio {
		return selectedRatio
	}
	return ratio_setting.GetGroupRatio(group)
}

func GetUserGroupRatioForUser(userId int, userGroup, group string) float64 {
	if userId > 0 {
		userSetting, err := model.GetUserSetting(userId, true)
		if err == nil && userSetting.UserGroupRatios != nil {
			if ratio, ok := userSetting.UserGroupRatios[group]; ok {
				return ratio
			}
		}
	}
	return GetUserGroupRatio(userGroup, group)
}

func GetUserGroupRatioWithSetting(userSetting dto.UserSetting, userGroup, group string) float64 {
	if userSetting.UserGroupRatios != nil {
		if ratio, ok := userSetting.UserGroupRatios[group]; ok {
			return ratio
		}
	}
	return GetUserGroupRatio(userGroup, group)
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}
