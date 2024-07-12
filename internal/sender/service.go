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
	"github.com/goverland-labs/core-web-sdk/dao"
	"github.com/goverland-labs/core-web-sdk/proposal"
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

type SettingsProvider interface {
	GetPushDetails(ctx context.Context, in *inboxapi.GetPushDetailsRequest, opts ...grpc.CallOption) (*inboxapi.GetPushDetailsResponse, error)
	GetPushToken(ctx context.Context, in *inboxapi.GetPushTokenRequest, opts ...grpc.CallOption) (*inboxapi.PushTokenResponse, error)
	GetPushTokenList(ctx context.Context, in *inboxapi.GetPushTokenListRequest, opts ...grpc.CallOption) (*inboxapi.PushTokenListResponse, error)
}

type MessageSender interface {
	Send(ctx context.Context, message *messaging.Message) (string, error)
}

type CoreDataProvider interface {
	GetUserVotes(ctx context.Context, address string, params coresdk.GetUserVotesRequest) (*proposal.VoteList, error)
	GetDao(ctx context.Context, id uuid.UUID) (*dao.Dao, error)
	GetProposal(ctx context.Context, id string) (*proposal.Proposal, error)
}

type DataManipulator interface {
	Create(item *History) error
	GetByHash(hash string) (*History, error)
	MarkAsClicked(messageUUID uuid.UUID) error
	QueueByFilters(_ context.Context, filters []Filter) ([]SendQueue, error)
	CreateSendQueueRequest(_ context.Context, item *SendQueue) error
	MarkAsSent(_ context.Context, ids []uint) error
}

type cacheItem struct {
	expireAt time.Time
	data     any
}

type Service struct {
	repo          *Repo
	subscriptions SubscriptionsFinder
	usrs          UsersFinder
	settings      SettingsProvider
	core          CoreDataProvider
	sender        MessageSender

	cache map[string]cacheItem
	mu    sync.Mutex

	cfg       []byte
	projectID string
}

func NewService(
	r *Repo,
	cfg config.Push,
	subs SubscriptionsFinder,
	usrs UsersFinder,
	sp SettingsProvider,
	coreSDK *coresdk.Client,
) (*Service, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	// todo: check if it can live days...
	sender, err := makeSender(context.Background(), data, cfg.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to make sender: %w", err)
	}

	return &Service{
		repo:          r,
		subscriptions: subs,
		usrs:          usrs,
		settings:      sp,
		cfg:           data,
		projectID:     cfg.ProjectID,
		sender:        sender,
		core:          coreSDK,
		cache:         make(map[string]cacheItem),
	}, nil
}

func (s *Service) GetToken(ctx context.Context, userID uuid.UUID) (string, error) {
	response, err := s.settings.GetPushToken(ctx, &inboxapi.GetPushTokenRequest{UserId: userID.String()})
	if err != nil {
		return "", fmt.Errorf("get push token by user_id: %s: %w", userID, err)
	}

	return response.GetToken(), nil
}

func (s *Service) GetTokens(ctx context.Context, userID uuid.UUID) ([]TokenDetails, error) {
	response, err := s.settings.GetPushTokenList(ctx, &inboxapi.GetPushTokenListRequest{UserId: userID.String()})
	if err != nil {
		return nil, fmt.Errorf("get push tokens by user_id: %s: %w", userID, err)
	}

	tokens := make([]TokenDetails, 0, len(response.GetTokens()))
	for _, info := range response.GetTokens() {
		tokens = append(tokens, TokenDetails{
			Token:      info.GetToken(),
			DeviceUUID: info.GetDeviceUuid(),
		})
	}

	return tokens, nil
}

func (r request) hash() string {
	summary := fmt.Sprintf(
		"%s_%s_%s_%s_%s_%s",
		r.userID.String(),
		r.deviceUUID,
		r.title,
		r.body,
		r.imageURL,
		time.Now().Format("2006-01-02"),
	)
	hash := md5.Sum([]byte(summary))
	return hex.EncodeToString(hash[:])
}

func (s *Service) Send(ctx context.Context, req request) error {
	list, err := s.GetTokens(context.TODO(), req.userID)
	if err != nil {
		log.Warn().Err(err).Msgf("get token for user %s", req.userID.String())

		return nil
	}

	msgID := uuid.New()
	for _, info := range list {
		req.deviceUUID = info.DeviceUUID
		item, err := s.repo.GetByHash(req.hash())
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("getByHash: %w", err)
		}
		if item != nil {
			log.Warn().Msgf("duplicate sending push: %s %s", req.userID.String(), req.title)

			continue
		}

		response, err := s.sender.Send(ctx, &messaging.Message{
			Token: info.Token,
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
				DeviceUUID: info.DeviceUUID,
			},
			PushResponse: response,
			Hash:         req.hash(),
		}); err != nil {
			log.Error().Err(err).Msg("create history log")
		}
	}

	return nil
}

func makeSender(ctx context.Context, cfg []byte, projectID string) (MessageSender, error) {
	authOpt := option.WithCredentialsJSON(cfg)
	fapp, err := firebase.NewApp(context.Background(), &firebase.Config{
		ProjectID: projectID,
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
