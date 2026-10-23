package repo

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// SettingRepository is the data-access contract for settings.
type SettingRepository interface {
	Get(key string) (string, bool, error)
	Set(key string, value string) error
	List() ([]model.Setting, error)
}

type settingRepository struct {
	db *gorm.DB
}

func (r *settingRepository) Get(key string) (string, bool, error) {
	var setting model.Setting
	err := r.db.Where("key = ?", key).First(&setting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return setting.Value, true, nil
}

func (r *settingRepository) Set(key string, value string) error {
	setting := model.Setting{Key: key, Value: value}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&setting).Error
}

func (r *settingRepository) List() ([]model.Setting, error) {
	var settings []model.Setting
	if err := r.db.Order("key ASC").Find(&settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}
