package sender

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"gorm.io/gorm"

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
	uuid     uuid.UUID
	token    string
	body     string
	title    string
	imageURL string
	userID   uuid.UUID
	payload  json.RawMessage
}

func (r request) hash() string {
	summary := fmt.Sprintf("%s_%s_%s_%s_%s", r.token, r.title, r.body, r.imageURL, r.userID.String())
	hash := md5.Sum([]byte(summary))
	return hex.EncodeToString(hash[:])
}

func (s *Service) Send(ctx context.Context, req request) error {
	item, err := s.repo.GetByHash(req.hash())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("getByHash: %w", err)
	}
	if item != nil {
		log.Warn().Msgf("duplicate sending push: %s %s", req.userID.String(), req.title)

		return nil
	}

	last, err := s.repo.GetLastSent(req.userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("repo.GetLastSent: %w", err)
	}

	// to avoid spamming
	if last != nil && time.Since(last.CreatedAt) <= time.Minute {
		log.Warn().Msgf("sending less than in minute: %s %s %v", req.userID.String(), req.title, time.Since(last.CreatedAt).Seconds())

		return nil
	}

	client, err := s.makeClient(ctx)
	if err != nil {
		return fmt.Errorf("s.makeClient: %w", err)
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
		Hash:         req.hash(),
	}); err != nil {
		log.Error().Err(err).Msg("create history log")
	}

	return nil
}

func (s *Service) SendCustom(ctx context.Context, req request) error {
	client, err := s.makeClient(ctx)
	if err != nil {
		return fmt.Errorf("s.makeClient: %w", err)
	}

	response, err := client.Send(ctx, &messaging.Message{
		Token: req.token,
		Data: map[string]string{
			"foo":     "bar",
			"payload": string(req.payload),
		},
		Notification: &messaging.Notification{
			Title:    req.title,
			Body:     req.body,
			ImageURL: req.imageURL,
		},
		APNS: &messaging.APNSConfig{
			Payload: &messaging.APNSPayload{
				Aps: &messaging.Aps{
					MutableContent: true,
				},
				CustomData: map[string]interface{}{
					"payload_as_bytes":  req.payload,
					"payload_as_string": string(req.payload),
				},
			},
			FCMOptions: &messaging.APNSFCMOptions{
				ImageURL: req.imageURL,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("send push: %w", err)
	}

	if err = s.repo.Create(&History{
		UserID: req.userID,
		Message: Message{
			ID:       req.uuid,
			Title:    req.title,
			Body:     req.body,
			ImageURL: req.token,
			Payload:  req.payload,
		},
		PushResponse: response,
		Hash:         uuid.NewString(),
	}); err != nil {
		log.Error().Err(err).Msg("create history log")
	}

	return nil
}

func (s *Service) makeClient(ctx context.Context) (*messaging.Client, error) {
	authOpt := option.WithCredentialsJSON(s.cfg)
	fapp, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: s.projectID,
	}, authOpt)
	if err != nil {
		return nil, fmt.Errorf("create firebase app: %w", err)
	}

	client, err := fapp.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("create firebase messagign: %w", err)
	}

	return client, nil
}

func (s *Service) MarkAsClicked(id uuid.UUID) error {
	return s.repo.MarkAsClicked(id)
}
