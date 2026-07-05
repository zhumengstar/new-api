package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range ratio_setting.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}

func GetGroupDetails(c *gin.Context) {
	groupNames := make([]string, 0)
	groupRatios := ratio_setting.GetGroupRatioCopy()
	userUsableGroups := setting.GetUserUsableGroupsCopy()
	userId := c.GetInt("id")
	userGroup, _ := model.GetUserGroup(userId, false)
	groupMeta := make(map[string]map[string]interface{}, len(groupRatios))
	for groupName, ratio := range groupRatios {
		groupNames = append(groupNames, groupName)
		_, isPublic := userUsableGroups[groupName]
		groupMeta[groupName] = map[string]interface{}{
			"ratio":       ratio,
			"admin_ratio": service.GetUserGroupRatioForUser(userId, userGroup, groupName),
			"is_public":   isPublic,
			"desc":        setting.GetUsableGroupDescription(groupName),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"groups": groupNames,
			"meta":   groupMeta,
		},
	})
}

func GetUserGroups(c *gin.Context) {
	usableGroups := make(map[string]map[string]interface{})
	userGroup := ""
	userId := c.GetInt("id")
	userGroup, _ = model.GetUserGroup(userId, false)
	userUsableGroups := service.GetUserUsableGroups(userGroup)
	for groupName, _ := range ratio_setting.GetGroupRatioCopy() {
		// UserUsableGroups contains the groups that the user can use
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]interface{}{
				"ratio": service.GetUserGroupRatioForUser(userId, userGroup, groupName),
				"desc":  desc,
			}
		}
	}
	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]interface{}{
			"ratio": "自动",
			"desc":  setting.GetUsableGroupDescription("auto"),
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    usableGroups,
	})
}
