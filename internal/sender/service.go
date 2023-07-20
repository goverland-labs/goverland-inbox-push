package sender

import (
	"context"
	"encoding/json"
	"fmt"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"

	"github.com/goverland-labs/inbox-push/internal/config"
)

type Service struct {
	repo   *Repo
	client inboxapi.SettingsClient

	cfg       []byte
	projectID string
}

func NewService(r *Repo, cfg config.Push, client inboxapi.SettingsClient) (*Service, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return &Service{
		repo:      r,
		client:    client,
		cfg:       data,
		projectID: cfg.ProjectID,
	}, nil
}

func (s *Service) GetToken(ctx context.Context, userID uuid.UUID) (string, error) {
	response, err := s.client.GetPushToken(ctx, &inboxapi.GetPushTokenRequest{UserId: userID.String()})
	if err != nil {
		return "", fmt.Errorf("get push token by user_id: %s: %w", userID, err)
	}

	return response.GetToken(), nil
}

type request struct {
	token    string
	body     string
	title    string
	imageURL string
	userID   uuid.UUID
}

func (s *Service) Send(ctx context.Context, req request) error {
	authOpt := option.WithCredentialsJSON(s.cfg)
	fapp, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: s.projectID,
	}, authOpt)
	if err != nil {
		return fmt.Errorf("create firebase app: %w", err)
	}

	client, err := fapp.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("create firebase messagign: %w", err)
	}

	response, err := client.Send(ctx, &messaging.Message{
		Notification: &messaging.Notification{
			Title:    req.title,
			Body:     req.body,
			ImageURL: req.imageURL,
		},
		Token: req.token,
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}

	if err = s.repo.Create(&History{
		UserID: req.userID,
		Message: Message{
			Title:    req.title,
			Body:     req.body,
			ImageURL: req.token,
		},
		PushResponse: response,
	}); err != nil {
		log.Error().Err(err).Msg("create history log")
	}

	return nil
}
