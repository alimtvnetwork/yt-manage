package repo

import (
	"gorm.io/gorm"

	"github.com/alimtvnetwork/yt-manage/internal/model"
)

// AccountRepository is the data-access contract for accounts.
type AccountRepository interface {
	Create(account *model.Account) error
	GetByID(id uint) (*model.Account, error)
	GetByGoogleSubject(subject string) (*model.Account, error)
	List() ([]model.Account, error)
	Update(account *model.Account) error
	Delete(id uint) error
	SaveToken(id uint, ciphertext []byte, nonce []byte) error
}

type accountRepository struct {
	db *gorm.DB
}

func (r *accountRepository) Create(account *model.Account) error {
	return r.db.Create(account).Error
}

func (r *accountRepository) GetByID(id uint) (*model.Account, error) {
	var account model.Account
	if err := r.db.First(&account, id).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) GetByGoogleSubject(subject string) (*model.Account, error) {
	var account model.Account
	if err := r.db.Where("google_subject = ?", subject).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *accountRepository) List() ([]model.Account, error) {
	var accounts []model.Account
	if err := r.db.Order("id ASC").Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *accountRepository) Update(account *model.Account) error {
	return r.db.Save(account).Error
}

func (r *accountRepository) Delete(id uint) error {
	return r.db.Delete(&model.Account{}, id).Error
}

func (r *accountRepository) SaveToken(id uint, ciphertext []byte, nonce []byte) error {
	return r.db.Model(&model.Account{}).Where("id = ?", id).Updates(map[string]interface{}{
		"token_ciphertext": ciphertext,
		"token_nonce":      nonce,
	}).Error
}
