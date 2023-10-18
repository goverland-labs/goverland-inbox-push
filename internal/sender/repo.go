package sender

import "gorm.io/gorm"

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
