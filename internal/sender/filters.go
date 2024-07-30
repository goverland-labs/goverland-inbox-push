package sender

import (
	"time"

	"gorm.io/gorm"
)

type Filter func(query *gorm.DB) *gorm.DB

func AvailableForSending() Filter {
	var (
		dummy SendQueue
		_     = dummy.SentAt
	)

	return func(query *gorm.DB) *gorm.DB {
		return query.Where("sent_at is null")
	}
}

func ActionIn(in ...string) Filter {
	var (
		dummy SendQueue
		_     = dummy.Action
	)

	return func(query *gorm.DB) *gorm.DB {
		return query.Where("action in ?", in)
	}
}

func ActionNotIn(in ...string) Filter {
	var (
		dummy SendQueue
		_     = dummy.Action
	)

	return func(query *gorm.DB) *gorm.DB {
		return query.Where("action not in ?", in)
	}
}

func UserIDIn(in ...string) Filter {
	var (
		dummy SendQueue
		_     = dummy.UserID
	)

	return func(query *gorm.DB) *gorm.DB {
		return query.Where("user_id in ?", in)
	}
}

func CreatedAfter(after time.Time) Filter {
	var (
		dummy SendQueue
		_     = dummy.UserID
	)

	return func(query *gorm.DB) *gorm.DB {
		return query.Where("created_at >= ?", after)
	}
}
