package sender

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	coresdk "github.com/goverland-labs/core-web-sdk"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/goverland-labs/inbox-push/internal/config"
)

type SubscriptionsFinder interface {
	FindSubscribers(ctx context.Context, in *inboxapi.FindSubscribersRequest, opts ...grpc.CallOption) (*inboxapi.UserList, error)
	ListSubscriptions(ctx context.Context, in *inboxapi.ListSubscriptionRequest, opts ...grpc.CallOption) (*inboxapi.ListSubscriptionResponse, error)
}

type UsersFinder interface {
	GetUserProfile(ctx context.Context, req *inboxapi.GetUserProfileRequest, opts ...grpc.CallOption) (*inboxapi.UserProfile, error)
	AllowSendingPush(ctx context.Context, req *inboxapi.AllowSendingPushRequest, opts ...grpc.CallOption) (*inboxapi.AllowSendingPushResponse, error)
}

type cacheItem struct {
	expireAt time.Time
	data     any
}

type Service struct {
	repo          *Repo
	subscriptions SubscriptionsFinder
	usrs          UsersFinder
	client        inboxapi.SettingsClient
	core          *coresdk.Client

	cache map[string]cacheItem
	mu    sync.Mutex

	cfg       []byte
	projectID string
}

func NewService(
	r *Repo,
	cfg config.Push,
	client inboxapi.SettingsClient,
	subs SubscriptionsFinder,
	usrs UsersFinder,
	coreSDK *coresdk.Client,
) (*Service, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	return &Service{
		repo:          r,
		subscriptions: subs,
		usrs:          usrs,
		client:        client,
		cfg:           data,
		projectID:     cfg.ProjectID,
		core:          coreSDK,
		cache:         make(map[string]cacheItem),
	}, nil
}

func (s *Service) GetToken(ctx context.Context, userID uuid.UUID) (string, error) {
	response, err := s.client.GetPushToken(ctx, &inboxapi.GetPushTokenRequest{UserId: userID.String()})
	if err != nil {
		return "", fmt.Errorf("get push token by user_id: %s: %w", userID, err)
	}

	return response.GetToken(), nil
}

func (r request) hash() string {
	summary := fmt.Sprintf(
		"%s_%s_%s_%s_%s",
		r.userID.String(),
		r.title,
		r.body,
		r.imageURL,
		time.Now().Format("2006-01-02"),
	)
	hash := md5.Sum([]byte(summary))
	return hex.EncodeToString(hash[:])
}

// Deprecated: use SendV2 function instead, will be removed in next releases
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
		log.Error().Err(err).Msg("send push")

		return nil
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

// Deprecated: use SendV2 function instead, will be removed in next releases
func (s *Service) SendCustom(ctx context.Context, req request) error {
	client, err := s.makeClient(ctx)
	if err != nil {
		return fmt.Errorf("s.makeClient: %w", err)
	}

	msgID := uuid.New()
	response, err := client.Send(ctx, &messaging.Message{
		Token: req.token,
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
					"id":        msgID,
					"proposals": req.payload,
				},
			},
			FCMOptions: &messaging.APNSFCMOptions{
				ImageURL: req.imageURL,
			},
		},
	})
	if err != nil {
		log.Error().Err(err).Msg("send custom push")

		return nil
	}

	if err = s.repo.Create(&History{
		UserID: req.userID,
		Message: Message{
			ID:       msgID,
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

func (s *Service) SendV2(ctx context.Context, req request) error {
	token, err := s.GetToken(context.TODO(), req.userID)
	if err != nil {
		log.Warn().Err(err).Msgf("get token for user %s", req.userID.String())

		return nil
	}

	item, err := s.repo.GetByHash(req.hash())
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("getByHash: %w", err)
	}
	if item != nil {
		log.Warn().Msgf("duplicate sending push: %s %s", req.userID.String(), req.title)

		return nil
	}

	client, err := s.makeClient(ctx)
	if err != nil {
		return fmt.Errorf("s.makeClient: %w", err)
	}

	msgID := uuid.New()
	response, err := client.Send(ctx, &messaging.Message{
		Token: token,
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
					"id":        msgID,
					"proposals": req.proposals,
				},
			},
			FCMOptions: &messaging.APNSFCMOptions{
				ImageURL: req.imageURL,
			},
		},
	})
	if err != nil {
		log.Error().
			Err(err).
			Msg("send push by external client")

		return nil
	}

	payload, _ := json.Marshal(req.proposals)
	if err = s.repo.Create(&History{
		UserID: req.userID,
		Message: Message{
			ID:         msgID,
			Title:      req.title,
			Body:       req.body,
			ImageURL:   req.imageURL,
			Payload:    payload,
			TemplateID: req.template,
		},
		PushResponse: response,
		Hash:         req.hash(),
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
