package sender

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *Repo) QueueByFilters(_ context.Context, filters []Filter) ([]SendQueue, error) {
	query := r.conn.Model(&SendQueue{})
	for _, f := range filters {
		f(query)
	}

	var list []SendQueue
	err := query.Find(&list).Error

	return list, err
}

func (r *Repo) CreateSendQueueRequest(_ context.Context, item *SendQueue) error {
	return r.conn.
		Model(&SendQueue{}).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "dao_id"},
				{Name: "proposal_id"},
				{Name: "action"},
			},
			DoNothing: true,
		}).
		Create(item).
		Error
}

func (r *Repo) MarkAsSent(_ context.Context, ids []uint) error {
	if len(ids) == 0 {
		return nil
	}

	var (
		dummy SendQueue
		_     = dummy.ID
		_     = dummy.SentAt
	)

	return r.conn.
		Model(&SendQueue{}).
		Where("id IN ?", ids).
		Update("sent_at", time.Now()).
		Error
}
