package sender

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/goverland-labs/platform-events/events/inbox"
	"github.com/rs/zerolog/log"
)

func (c *Consumer) handleFeed() inbox.FeedHandler {
	return func(payload inbox.FeedPayload) error {
		converted := convertPayloadToInternal(payload)

		if err := c.service.ProcessFeedItem(context.TODO(), converted); err != nil {
			log.Error().Err(err).Msgf("process item: %s", converted.ProposalID)
			return err
		}

		return nil
	}
}

func (s *Service) ProcessFeedItem(ctx context.Context, item Item) error {
	if !item.AllowSending() {
		return nil
	}

	resp, err := s.subscriptions.FindSubscribers(ctx, &inboxapi.FindSubscribersRequest{
		DaoId: item.DaoID.String(),
	})
	if err != nil {
		return err
	}

	for _, sub := range resp.Users {
		subscriberID, err := uuid.Parse(sub.GetUserId())
		if err != nil {
			return fmt.Errorf("unable to parse subscriber id '%s': %w", sub.GetUserId(), err)
		}

		// check that the user has allowed to receive push notifications
		_, err = s.GetToken(ctx, subscriberID)
		if err != nil {
			return nil
		}

		err = s.repo.CreateSendQueueRequest(ctx, &SendQueue{
			UserID:     subscriberID,
			DaoID:      item.DaoID,
			ProposalID: item.ProposalID,
			Action:     item.Action,
		})
		if err != nil {
			return fmt.Errorf("s.repo.CreateSendQueueRequest: %w", err)
		}
	}

	return nil
}

func convertPayloadToInternal(payload inbox.FeedPayload) Item {
	return Item{
		DaoID:      payload.DaoID,
		ProposalID: payload.ProposalID,
		Action:     convertPayloadActionToInternal(payload.Action),
	}
}

func convertPayloadActionToInternal(action inbox.TimelineAction) Action {
	converted, ok := payloadActionMap[action]

	if !ok {
		log.Warn().Any("action", action).Msg("unknown payload timeline action")
	}

	return converted
}

var payloadActionMap = map[inbox.TimelineAction]Action{
	inbox.DaoCreated:                  DaoCreated,
	inbox.DaoUpdated:                  DaoUpdated,
	inbox.ProposalCreated:             ProposalCreated,
	inbox.ProposalUpdated:             ProposalUpdated,
	inbox.ProposalVotingStartsSoon:    ProposalVotingStartsSoon,
	inbox.ProposalVotingEndsSoon:      ProposalVotingEndsSoon,
	inbox.ProposalVotingStarted:       ProposalVotingStarted,
	inbox.ProposalVotingQuorumReached: ProposalVotingQuorumReached,
	inbox.ProposalVotingEnded:         ProposalVotingEnded,
}
