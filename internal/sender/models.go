package sender

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Message struct {
	ID       uuid.UUID
	Title    string          `json:"title"`
	Body     string          `json:"body"`
	ImageURL string          `json:"image_url"`
	Payload  json.RawMessage `json:"payload"`
}

type History struct {
	gorm.Model

	UserID       uuid.UUID
	Message      Message `gorm:"serializer:json"`
	PushResponse string
	Hash         string
	ClickedAt    time.Time
}
