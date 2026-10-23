package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// ABTestRepository is the data-access contract for A/B tests.
type ABTestRepository interface {
	Create(test *model.ABTest) error
	GetByID(id uint) (*model.ABTest, error)
	ListByVideo(videoID uint) ([]model.ABTest, error)
	ListActive() ([]model.ABTest, error)
	AddSnapshot(snapshot *model.ABTestSnapshot) error
	ListSnapshots(testID uint) ([]model.ABTestSnapshot, error)
	Update(test *model.ABTest) error
}

type abTestRepository struct {
	db *gorm.DB
}

func (r *abTestRepository) Create(test *model.ABTest) error {
	return r.db.Create(test).Error
}

func (r *abTestRepository) GetByID(id uint) (*model.ABTest, error) {
	var test model.ABTest
	if err := r.db.First(&test, id).Error; err != nil {
		return nil, err
	}
	return &test, nil
}

func (r *abTestRepository) ListByVideo(videoID uint) ([]model.ABTest, error) {
	var tests []model.ABTest
	if err := r.db.Where("video_id = ?", videoID).Order("id ASC").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

func (r *abTestRepository) ListActive() ([]model.ABTest, error) {
	var tests []model.ABTest
	if err := r.db.Where("is_active = ?", true).Order("id ASC").Find(&tests).Error; err != nil {
		return nil, err
	}
	return tests, nil
}

func (r *abTestRepository) AddSnapshot(snapshot *model.ABTestSnapshot) error {
	return r.db.Create(snapshot).Error
}

func (r *abTestRepository) ListSnapshots(testID uint) ([]model.ABTestSnapshot, error) {
	var snapshots []model.ABTestSnapshot
	if err := r.db.Where("test_id = ?", testID).Order("captured_at ASC").Find(&snapshots).Error; err != nil {
		return nil, err
	}
	return snapshots, nil
}

func (r *abTestRepository) Update(test *model.ABTest) error {
	return r.db.Save(test).Error
}
