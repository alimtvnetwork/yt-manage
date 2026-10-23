package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// ScheduleRepository is the data-access contract for schedule rules.
type ScheduleRepository interface {
	Create(rule *model.ScheduleRule) error
	GetByID(id uint) (*model.ScheduleRule, error)
	ListByChannel(channelID uint) ([]model.ScheduleRule, error)
	ListActive() ([]model.ScheduleRule, error)
	Update(rule *model.ScheduleRule) error
	Delete(id uint) error
}

type scheduleRepository struct {
	db *gorm.DB
}

func (r *scheduleRepository) Create(rule *model.ScheduleRule) error {
	return r.db.Create(rule).Error
}

func (r *scheduleRepository) GetByID(id uint) (*model.ScheduleRule, error) {
	var rule model.ScheduleRule
	if err := r.db.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *scheduleRepository) ListByChannel(channelID uint) ([]model.ScheduleRule, error) {
	var rules []model.ScheduleRule
	if err := r.db.Where("channel_id = ?", channelID).Order("id ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *scheduleRepository) ListActive() ([]model.ScheduleRule, error) {
	var rules []model.ScheduleRule
	if err := r.db.Where("is_active = ?", true).Order("id ASC").Find(&rules).Error; err != nil {
		return nil, err
	}
	return rules, nil
}

func (r *scheduleRepository) Update(rule *model.ScheduleRule) error {
	return r.db.Save(rule).Error
}

func (r *scheduleRepository) Delete(id uint) error {
	return r.db.Delete(&model.ScheduleRule{}, id).Error
}
