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
