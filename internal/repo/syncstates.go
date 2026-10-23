package repo

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// SyncStateRepository is the data-access contract for sync states.
type SyncStateRepository interface {
	Get(accountID uint, resource string) (*model.SyncState, error)
	Upsert(state *model.SyncState) error
	ListByAccount(accountID uint) ([]model.SyncState, error)
}

type syncStateRepository struct {
	db *gorm.DB
}

func (r *syncStateRepository) Get(accountID uint, resource string) (*model.SyncState, error) {
	var state model.SyncState
	if err := r.db.Where("account_id = ? AND resource = ?", accountID, resource).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func (r *syncStateRepository) Upsert(state *model.SyncState) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "account_id"}, {Name: "resource"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cursor", "last_run_at", "last_error", "updated_at",
		}),
	}).Create(state).Error
}

func (r *syncStateRepository) ListByAccount(accountID uint) ([]model.SyncState, error) {
	var states []model.SyncState
	if err := r.db.Where("account_id = ?", accountID).Order("id ASC").Find(&states).Error; err != nil {
		return nil, err
	}
	return states, nil
}
