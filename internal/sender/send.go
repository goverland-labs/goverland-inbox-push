package sender

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	goverlandcorewebsdk "github.com/goverland-labs/core-web-sdk"
	"github.com/goverland-labs/core-web-sdk/dao"
	"github.com/goverland-labs/core-web-sdk/proposal"
	"github.com/goverland-labs/inbox-api/protobuf/inboxapi"
	"github.com/rs/zerolog/log"
)

func (s *Service) sendBatch(ctx context.Context) error {
	// get list from queue
	list, err := s.repo.QueueByFilters(ctx, []Filter{
		AvailableForSending(),
		ActionNotIn(string(ProposalVotingEndsSoon)),
	})
	if err != nil {
		return fmt.Errorf("s.repo.GetInQueue: %w", err)
	}

	if len(list) == 0 {
		return nil
	}

	batches := make(map[uuid.UUID][]SendQueue)
	for _, item := range list {
		byUser := batches[item.UserID]
		byUser = append(byUser, item)
		batches[item.UserID] = byUser
	}

	sent := make([]uint, 0, len(list))
	defer func(ids []uint) {
		if err := s.repo.MarkAsSent(context.TODO(), sent); err != nil {
			log.Error().Err(err).Msg("mark as sent")
		}
	}(sent)

	for userID, details := range batches {
		// let's check if we can send a push
		res, err := s.usrs.AllowSendingPush(ctx, &inboxapi.AllowSendingPushRequest{UserId: userID.String()})
		if err != nil {
			return fmt.Errorf("s.usrs.AllowSendingPush: %w", err)
		}
		if !res.Allow {
			continue
		}

		req, err := s.prepareReq(ctx, userID, details)
		if err != nil {
			return fmt.Errorf("s.prepareReq: %w", err)
		}
		if err := s.SendV2(ctx, req); err != nil {
			return fmt.Errorf("s.SendV2: %w", err)
		}

		for _, info := range details {
			sent = append(sent, info.ID)
		}
	}

	return nil
}

func (s *Service) prepareReq(ctx context.Context, userID uuid.UUID, details []SendQueue) (request, error) {
	if len(details) == 0 {
		return request{}, fmt.Errorf("empty details")
	}

	req := request{
		userID: userID,
	}

	daoByID := map[uuid.UUID]struct{}{}
	daos := make([]uuid.UUID, 0)
	proposals := make([]string, 0, len(details))
	for _, info := range details {
		if _, ok := daoByID[info.DaoID]; !ok {
			daoByID[info.DaoID] = struct{}{}
			daos = append(daos, info.DaoID)
		}

		proposals = append(proposals, info.ProposalID)
		req.proposals = append(req.proposals, info.ProposalID)
	}

	if len(daos) >= 2 {
		req.title = "Goverland"

		maxCnt := 2
		names := make([]string, maxCnt)
		for idx := range daos[:maxCnt] {
			dd, err := s.getDao(ctx, daos[idx])
			if err != nil {
				return req, fmt.Errorf("s.getDao: %w", err)
			}

			names[idx] = dd.Name
		}
		if len(daos) > 2 {
			req.body = fmt.Sprintf("%s, %s, and more have updates on proposals.", names[0], names[1])
		} else {
			req.body = fmt.Sprintf("%s and %s have updates on proposals.", names[0], names[1])
		}

		return req, nil
	}

	dd, err := s.getDao(ctx, daos[0])
	if err != nil {
		return req, fmt.Errorf("s.getDao: %w", err)
	}
	req.imageURL = generateDaoIcon(dd.Alias)

	if len(proposals) > 1 {
		req.title = dd.Name
		req.body = fmt.Sprintf("Updates on %d proposals", len(proposals))

		return req, nil
	}

	pr, err := s.getProposal(ctx, proposals[0])
	if err != nil {
		return req, fmt.Errorf("s.getProposal: %w", err)
	}

	req.title = fmt.Sprintf("%s: %s", dd.Name, convertActionToTitle(details[0].Action))
	req.body = pr.Title

	return req, nil
}

func convertActionToTitle(action Action) string {
	switch action {
	case ProposalCreated:
		return "New proposal created"
	case ProposalVotingQuorumReached:
		return "Quorum reached"
	case ProposalVotingEnded:
		return "Vote finished"
	default:
		return "have update on proposal"
	}
}

func (s *Service) sendVotingEndsSoon(ctx context.Context) error {
	list, err := s.repo.QueueByFilters(ctx, []Filter{
		AvailableForSending(),
		ActionIn(string(ProposalVotingEndsSoon)),
	})
	if err != nil {
		return fmt.Errorf("s.repo.GetInQueue: %w", err)
	}

	if len(list) == 0 {
		return nil
	}

	sent := make([]uint, 0, len(list))
	defer func(ids []uint) {
		if err := s.repo.MarkAsSent(context.TODO(), sent); err != nil {
			log.Error().Err(err).Msg("mark as sent")
		}
	}(sent)

	for _, item := range list {
		usr, err := s.usrs.GetUserProfile(ctx, &inboxapi.GetUserProfileRequest{UserId: item.UserID.String()})
		if err != nil {
			return fmt.Errorf("s.usrs.GetUserProfile: %w", err)
		}

		res, err := s.core.GetUserVotes(ctx, usr.GetUser().GetAddress(), goverlandcorewebsdk.GetUserVotesRequest{
			ProposalIDs: []string{item.ProposalID},
			Limit:       1,
		})
		if err != nil {
			return fmt.Errorf("s.core.UserVoted: %w", err)
		}

		// user has voted
		if len(res.Items) != 0 {
			sent = append(sent, item.ID)
		}

		dd, err := s.getDao(ctx, item.DaoID)
		if err != nil {
			return fmt.Errorf("s.getDao: %w", err)
		}

		pr, err := s.getProposal(ctx, item.ProposalID)
		if err != nil {
			return fmt.Errorf("s.getProposal: %w", err)
		}

		err = s.SendV2(ctx, request{
			title:     fmt.Sprintf("%s: Vote finishes soon", dd.Name),
			body:      pr.Title,
			imageURL:  generateDaoIcon(dd.Alias),
			userID:    item.UserID,
			proposals: []string{item.ProposalID},
		})
		if err != nil {
			return fmt.Errorf("s.SendV2: %w", err)
		}

		sent = append(sent, item.ID)
	}

	return nil
}

// todo: refactor it
func (s *Service) getDao(ctx context.Context, id uuid.UUID) (*dao.Dao, error) {
	key := fmt.Sprintf("dao-%s", id.String())
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.cache[key]
	if ok && time.Now().Before(val.expireAt) {
		return val.data.(*dao.Dao), nil
	}

	dao, err := s.core.GetDao(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("s.core.GetDao: %s: %w", id, err)
	}

	s.cache[key] = cacheItem{
		expireAt: time.Now().Add(time.Hour),
		data:     dao,
	}

	return dao, nil
}

// todo: refactor it
func (s *Service) getProposal(ctx context.Context, id string) (*proposal.Proposal, error) {
	key := fmt.Sprintf("pr-%s", id)
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.cache[key]
	if ok && time.Now().Before(val.expireAt) {
		return val.data.(*proposal.Proposal), nil
	}

	pr, err := s.core.GetProposal(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("s.core.GetProposal: %s: %w", id, err)
	}

	s.cache[key] = cacheItem{
		expireAt: time.Now().Add(time.Hour),
		data:     pr,
	}

	return pr, nil
}

func generateDaoIcon(alias string) string {
	return fmt.Sprintf("https://cdn.stamp.fyi/avatar/%s?s=180", alias, 180)
}
