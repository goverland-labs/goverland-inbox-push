package sender

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/goverland-labs/goverland-inbox-api-protocol/protobuf/inboxapi"
	"github.com/goverland-labs/goverland-platform-events/events/inbox"
	"github.com/rs/zerolog/log"
)

func (c *Consumer) handleFeed() inbox.FeedHandler {
	return func(payload inbox.FeedPayload) error {
		converted := convertPayloadToInternal(payload)

		log.Info().Msgf("start processing proposal %s with action %s", converted.ProposalID, converted.Action)

		if err := c.service.ProcessFeedItem(context.TODO(), converted); err != nil {
			log.Error().
				Err(err).
				Msgf("process proposal %s with action %s", converted.ProposalID, converted.Action)

			return err
		}

		log.Info().Msgf("processed proposal %s with action %s", converted.ProposalID, converted.Action)

		return nil
	}
}

func (s *Service) ProcessFeedItem(ctx context.Context, item Item) error {
	if !item.AllowSending() {
		log.Info().Msgf("skip processing due to invalid type/action: %s with action %s", item.ProposalID, item.Action)

		return nil
	}

	resp, err := s.subscriptions.FindSubscribers(ctx, &inboxapi.FindSubscribersRequest{
		DaoId: item.DaoID.String(),
	})
	if err != nil {
		return fmt.Errorf("find subscribers by dao id %s: %w", item.DaoID.String(), err)
	}

	log.Info().Msgf("for dao %s founded %d subscribers", item.DaoID.String(), len(resp.Users))

	for _, sub := range resp.Users {
		subscriberID, err := uuid.Parse(sub.GetUserId())
		if err != nil {
			return fmt.Errorf("unable to parse subscriber id '%s': %w", sub.GetUserId(), err)
		}

		// check that the user has allowed to receive push notifications
		if list, err := s.GetTokens(ctx, subscriberID); err != nil || len(list) == 0 {
			log.Info().Msgf("skip processing %s to user %s due to missing tokens", item.ProposalID, subscriberID.String())

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

		log.Info().Msgf("proposal %s processed for %s subscriber", item.ProposalID, subscriberID.String())

		collectStats("queue", "add", err)
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
