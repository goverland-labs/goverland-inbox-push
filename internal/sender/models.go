package sender

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	ImageURL string `json:"image_url"`
}

type History struct {
	gorm.Model

	UserID       uuid.UUID
	Message      Message `gorm:"serializer:json"`
	PushResponse string
}
