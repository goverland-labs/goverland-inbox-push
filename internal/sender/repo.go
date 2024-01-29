package sender

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repo struct {
	conn *gorm.DB
}

func NewRepo(db *gorm.DB) *Repo {
	return &Repo{
		conn: db,
	}
}

func (r *Repo) Create(item *History) error {
	return r.conn.Create(item).Error
}

func (r *Repo) GetByHash(hash string) (*History, error) {
	var h History
	err := r.conn.
		Model(&History{}).
		Where(&History{
			Hash: hash,
		}).
		First(&h).
		Error

	if err != nil {
		return nil, err
	}

	return &h, nil
}

func (r *Repo) GetLastSent(userID uuid.UUID) (*History, error) {
	var h History
	err := r.conn.
		Model(&History{}).
		Where(&History{
			UserID: userID,
		}).
		Last(&h).
		Error

	if err != nil {
		return nil, err
	}

	return &h, nil
}

func (r *Repo) MarkAsClicked(messageUUID uuid.UUID) error {
	var (
		h History
		_ = h.Message.ID
		_ = h.ClickedAt
	)

	return r.conn.Exec(`
		update histories set clicked_at = now() 
		where message->>'id' = ?
	`, messageUUID).Error
}
