package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// PeerRepository is the data-access contract for peers.
type PeerRepository interface {
	Create(peer *model.Peer) error
	GetByID(id uint) (*model.Peer, error)
	GetByName(name string) (*model.Peer, error)
	List() ([]model.Peer, error)
	Update(peer *model.Peer) error
	Delete(id uint) error
}

type peerRepository struct {
	db *gorm.DB
}

func (r *peerRepository) Create(peer *model.Peer) error {
	return r.db.Create(peer).Error
}

func (r *peerRepository) GetByID(id uint) (*model.Peer, error) {
	var peer model.Peer
	if err := r.db.First(&peer, id).Error; err != nil {
		return nil, err
	}
	return &peer, nil
}

func (r *peerRepository) GetByName(name string) (*model.Peer, error) {
	var peer model.Peer
	if err := r.db.Where("name = ?", name).First(&peer).Error; err != nil {
		return nil, err
	}
	return &peer, nil
}

func (r *peerRepository) List() ([]model.Peer, error) {
	var peers []model.Peer
	if err := r.db.Order("id ASC").Find(&peers).Error; err != nil {
		return nil, err
	}
	return peers, nil
}

func (r *peerRepository) Update(peer *model.Peer) error {
	return r.db.Save(peer).Error
}

func (r *peerRepository) Delete(id uint) error {
	return r.db.Delete(&model.Peer{}, id).Error
}
