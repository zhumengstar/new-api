package model

import (
	"errors"
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserVirtualQuota struct {
	Id        int   `json:"id" gorm:"primaryKey"`
	UserId    int   `json:"user_id" gorm:"uniqueIndex;not null"`
	AdminId   int   `json:"admin_id" gorm:"index;not null"`
	Quota     int   `json:"quota" gorm:"not null;default:0"`
	UsedQuota int   `json:"used_quota" gorm:"not null;default:0"`
	CreatedAt int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt int64 `json:"updated_at" gorm:"bigint"`
}

func (UserVirtualQuota) TableName() string {
	return "user_virtual_quotas"
}

func (v *UserVirtualQuota) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if v.CreatedAt == 0 {
		v.CreatedAt = now
	}
	v.UpdatedAt = now
	return nil
}

func (v *UserVirtualQuota) BeforeUpdate(tx *gorm.DB) error {
	v.UpdatedAt = common.GetTimestamp()
	return nil
}

func (v UserVirtualQuota) RemainingQuota() int {
	remaining := v.Quota - v.UsedQuota
	if remaining < 0 {
		return 0
	}
	return remaining
}

func GetUserVirtualQuota(userId int) (*UserVirtualQuota, error) {
	var quota UserVirtualQuota
	err := DB.Where("user_id = ?", userId).First(&quota).Error
	if err != nil {
		return nil, err
	}
	return &quota, nil
}

func GetUserVirtualQuotaMap(userIds []int) (map[int]UserVirtualQuota, error) {
	quotas := make(map[int]UserVirtualQuota)
	if len(userIds) == 0 {
		return quotas, nil
	}
	var rows []UserVirtualQuota
	if err := DB.Where("user_id IN ?", userIds).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		quotas[row.UserId] = row
	}
	return quotas, nil
}

func ApplyAdminVirtualQuotaView(adminId int, users []*User) error {
	if adminId <= 0 || len(users) == 0 {
		return nil
	}
	userIds := make([]int, 0, len(users))
	for _, user := range users {
		if user != nil && user.InviterId == adminId {
			userIds = append(userIds, user.Id)
		}
	}
	quotaMap, err := GetUserVirtualQuotaMap(userIds)
	if err != nil {
		return err
	}
	for _, user := range users {
		if user == nil {
			continue
		}
		if user.InviterId != adminId {
			continue
		}
		virtualQuota, ok := quotaMap[user.Id]
		if !ok {
			user.Quota = 0
			user.UsedQuota = 0
			continue
		}
		user.Quota = virtualQuota.RemainingQuota()
		user.UsedQuota = virtualQuota.UsedQuota
	}
	return nil
}

func ApplyAdminVirtualQuotaToUser(adminId int, user *User) error {
	if adminId <= 0 || user == nil || user.InviterId != adminId {
		return nil
	}
	virtualQuota, err := GetUserVirtualQuota(user.Id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user.Quota = 0
		user.UsedQuota = 0
		return nil
	}
	if err != nil {
		return err
	}
	user.Quota = virtualQuota.RemainingQuota()
	user.UsedQuota = virtualQuota.UsedQuota
	return nil
}

func SumAdminVirtualRemainingQuota(adminId int, excludeUserId int) (int, error) {
	var rows []UserVirtualQuota
	query := DB.Where("admin_id = ?", adminId)
	if excludeUserId > 0 {
		query = query.Where("user_id <> ?", excludeUserId)
	}
	if err := query.Find(&rows).Error; err != nil {
		return 0, err
	}
	total := 0
	for _, row := range rows {
		total += row.RemainingQuota()
	}
	return total, nil
}

func SetUserVirtualQuota(adminId int, userId int, quota int) error {
	if adminId <= 0 || userId <= 0 {
		return errors.New("invalid admin or user id")
	}
	if quota < 0 {
		return errors.New("virtual quota must be non-negative")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var admin User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", adminId).First(&admin).Error; err != nil {
			return err
		}
		var user User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		if user.InviterId != adminId {
			return errors.New("user is not invited by this admin")
		}
		var current UserVirtualQuota
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userId).First(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		usedQuota := 0
		if err == nil {
			usedQuota = current.UsedQuota
		}
		if quota < usedQuota {
			return fmt.Errorf("virtual quota cannot be less than used quota: %d", usedQuota)
		}
		var rows []UserVirtualQuota
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("admin_id = ? AND user_id <> ?", adminId, userId).
			Find(&rows).Error; err != nil {
			return err
		}
		totalRemaining := quota - usedQuota
		for _, row := range rows {
			totalRemaining += row.RemainingQuota()
		}
		if totalRemaining > admin.Quota {
			return fmt.Errorf("virtual quota exceeds admin available quota: admin quota=%d, allocated remaining=%d", admin.Quota, totalRemaining)
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(&UserVirtualQuota{AdminId: adminId, UserId: userId, Quota: quota}).Error
		}
		return tx.Model(&UserVirtualQuota{}).Where("id = ?", current.Id).Updates(map[string]interface{}{
			"admin_id": adminId,
			"quota":    quota,
		}).Error
	})
}

func ConsumeVirtualQuota(adminId int, userId int, virtualDelta int, adminDelta int) error {
	if adminId <= 0 || userId <= 0 {
		return errors.New("invalid admin or user id")
	}
	if virtualDelta == 0 && adminDelta == 0 {
		return nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var virtualQuota UserVirtualQuota
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("admin_id = ? AND user_id = ?", adminId, userId).
			First(&virtualQuota).Error; err != nil {
			return err
		}
		if virtualDelta > 0 && virtualQuota.RemainingQuota() < virtualDelta {
			return fmt.Errorf("virtual quota insufficient: remaining=%d, required=%d", virtualQuota.RemainingQuota(), virtualDelta)
		}
		if virtualDelta < 0 && virtualQuota.UsedQuota+virtualDelta < 0 {
			return fmt.Errorf("virtual quota refund exceeds used quota: used=%d, refund=%d", virtualQuota.UsedQuota, -virtualDelta)
		}
		var admin User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", adminId).First(&admin).Error; err != nil {
			return err
		}
		if adminDelta > 0 && admin.Quota < adminDelta {
			return fmt.Errorf("admin quota insufficient: remaining=%d, required=%d", admin.Quota, adminDelta)
		}
		if adminDelta < 0 && admin.UsedQuota+adminDelta < 0 {
			adminDelta = -admin.UsedQuota
		}
		if virtualDelta != 0 {
			if err := tx.Model(&UserVirtualQuota{}).Where("id = ?", virtualQuota.Id).Update("used_quota", gorm.Expr("used_quota + ?", virtualDelta)).Error; err != nil {
				return err
			}
		}
		if adminDelta != 0 {
			if err := tx.Model(&User{}).Where("id = ?", adminId).Updates(map[string]interface{}{
				"quota":      gorm.Expr("quota - ?", adminDelta),
				"used_quota": gorm.Expr("used_quota + ?", adminDelta),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := InvalidateUserCache(adminId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate admin cache after virtual quota consume: %s", err.Error()))
	}
	if err := InvalidateUserCache(userId); err != nil {
		common.SysLog(fmt.Sprintf("failed to invalidate user cache after virtual quota consume: %s", err.Error()))
	}
	return nil
}

func ScaleVirtualQuota(quota int, adminRatio float64, userRatio float64) int {
	if quota == 0 {
		return 0
	}
	if adminRatio <= 0 || userRatio <= 0 {
		return quota
	}
	scaled := int(math.Round(float64(quota) * adminRatio / userRatio))
	if quota > 0 && scaled <= 0 {
		return 1
	}
	if quota < 0 && scaled >= 0 {
		return -1
	}
	return scaled
}
