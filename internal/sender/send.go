package sender

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	goverlandcorewebsdk "github.com/goverland-labs/goverland-core-sdk-go"
	"github.com/goverland-labs/goverland-core-sdk-go/dao"
	"github.com/goverland-labs/goverland-core-sdk-go/proposal"
	"github.com/goverland-labs/goverland-inbox-api-protocol/protobuf/inboxapi"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *Service) sendBatch(ctx context.Context) error {
	// get list from queue
	list, err := s.repo.QueueByFilters(ctx, []Filter{
		AvailableForSending(),
		ActionNotIn(
			string(ProposalVotingEndsSoon),
			string(DelegateCreateProposal),
			string(DelegateVotingVoted),
			string(DelegateVotingSkipVote),
		),
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
	defer func() {
		log.Info().Msgf("marking as sent ids: %v", sent)

		if err := s.repo.MarkAsSent(context.TODO(), sent); err != nil {
			log.Error().Err(err).Msg("mark as sent")
		}
	}()

	for userID, details := range batches {
		//let's check if we can send a push
		res, err := s.usrs.AllowSendingPush(ctx, &inboxapi.AllowSendingPushRequest{UserId: userID.String()})
		if err != nil {
			return fmt.Errorf("s.usrs.AllowSendingPush: %w", err)
		}
		if !res.Allow {
			log.Info().Msgf("user is not allow to recieve push: %s", userID.String())

			continue
		}

		allowedActions, err := s.getAllowedSendActions(userID)
		if err != nil {
			return fmt.Errorf("s.getAllowedSendActions: %w", err)
		}

		log.Info().Msgf("user %s allowed actions: %v", userID.String(), allowedActions)

		// filter only supported actions by user cfg
		supported := make([]SendQueue, 0, len(details))
		for _, info := range details {
			if !allowedActions.Contains(info.Action) {
				sent = append(sent, info.ID)
				continue
			}

			supported = append(supported, info)
		}

		log.Info().Msgf("user %s supported to recieve: %v", userID.String(), supported)

		// if no supported, do not send anything
		if len(supported) == 0 {
			continue
		}

		req, err := s.prepareBatchReq(ctx, userID, supported)
		if err != nil {
			return fmt.Errorf("s.prepareBatchReq: %w", err)
		}
		if err := s.Send(ctx, req); err != nil {
			return fmt.Errorf("s.Send: %w", err)
		}

		collectStats("send", "batch", err)

		for _, info := range details {
			sent = append(sent, info.ID)
		}
	}

	return nil
}

func (s *Service) prepareBatchReq(ctx context.Context, userID uuid.UUID, details []SendQueue) (request, error) {
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
		req.template = templateIDFewDao

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
			req.template = templateIDTwoDao
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
		req.template = templateIDOneDaoFewProposal
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
	req.template = templateIDOneDaoOneProposal

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

	// group by user_id
	batches := make(map[uuid.UUID][]SendQueue)
	for _, item := range list {
		byUser := batches[item.UserID]
		byUser = append(byUser, item)
		batches[item.UserID] = byUser
	}

	// prepareVotingEndsSoonReq
	for userID, details := range batches {
		req, err := s.prepareVotingEndsSoonReq(ctx, userID, details)
		if err != nil {
			return fmt.Errorf("s.prepareVotingEndsSoonReq: %s: %w", userID, err)
		}
		if req != nil {
			err = s.Send(ctx, *req)
			if err != nil {
				return fmt.Errorf("s.Send: %w", err)
			}

			collectStats("send", "voting_ends_soon", err)
		}

		for _, info := range details {
			sent = append(sent, info.ID)
		}
	}

	return nil
}

func (s *Service) sendDelegates(ctx context.Context) error {
	list, err := s.repo.QueueByFilters(ctx, []Filter{
		AvailableForSending(),
		ActionIn(
			string(DelegateCreateProposal),
			string(DelegateVotingVoted),
			string(DelegateVotingSkipVote),
		),
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

	for _, info := range list {
		req, err := s.prepareDelegationPush(ctx, info)
		if err != nil {
			return fmt.Errorf("s.prepareDelegationPush: %d: %w", info.ID, err)
		}

		err = s.Send(ctx, req)
		if err != nil {
			return fmt.Errorf("s.Send: %w", err)
		}

		collectStats("send", string(info.Action), err)

		sent = append(sent, info.ID)
	}

	return nil
}

func (s *Service) prepareVotingEndsSoonReq(ctx context.Context, userID uuid.UUID, details []SendQueue) (*request, error) {
	if len(details) == 0 {
		return nil, fmt.Errorf("empty details")
	}

	filtered, err := s.getNotVotedDetails(ctx, userID, details)
	if err != nil {
		return nil, fmt.Errorf("s.getNotVotedDetails: %w", err)
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	req := request{
		userID:   userID,
		template: templateIDVoteFinishesSoon,
	}

	daoByID := map[uuid.UUID]struct{}{}
	daos := make([]uuid.UUID, 0)
	proposals := make([]string, 0, len(filtered))
	for _, info := range filtered {
		if _, ok := daoByID[info.DaoID]; !ok {
			daoByID[info.DaoID] = struct{}{}
			daos = append(daos, info.DaoID)
		}

		proposals = append(proposals, info.ProposalID)
		req.proposals = append(req.proposals, info.ProposalID)
	}

	if len(daos) >= 2 {
		req.title = "Votes finish soon"

		names := make([]string, 0, len(daos))
		for _, daoID := range daos {
			dd, err := s.getDao(ctx, daoID)
			if err != nil {
				return nil, fmt.Errorf("s.getDao: %w", err)
			}

			names = append(names, dd.Name)
		}

		req.body = fmt.Sprintf("%d active proposals in %s will finish soon.", len(req.proposals), prepareVotingEndsSoonNames(names))

		return &req, nil
	}

	dd, err := s.getDao(ctx, daos[0])
	if err != nil {
		return nil, fmt.Errorf("s.getDao: %w", err)
	}
	req.imageURL = generateDaoIcon(dd.Alias)
	req.title = fmt.Sprintf("%s: Votes finish soon", dd.Name)

	if len(proposals) > 1 {
		req.body = fmt.Sprintf("%d active proposals in %s will finish soon.", len(req.proposals), dd.Name)

		return &req, nil
	}

	pr, err := s.getProposal(ctx, proposals[0])
	if err != nil {
		return nil, fmt.Errorf("s.getProposal: %w", err)
	}
	req.body = pr.Title

	return &req, nil
}

func (s *Service) prepareDelegationPush(ctx context.Context, info SendQueue) (request, error) {
	req := request{
		userID:    info.UserID,
		proposals: []string{info.ProposalID},
	}

	dd, err := s.getDao(ctx, info.DaoID)
	if err != nil {
		return request{}, fmt.Errorf("s.getDao: %w", err)
	}

	req.imageURL = generateDaoIcon(dd.Alias)

	pr, err := s.getProposal(ctx, info.ProposalID)
	if err != nil {
		return request{}, fmt.Errorf("s.getProposal: %w", err)
	}

	req.title = dd.Name

	switch info.Action {
	case DelegateCreateProposal:
		req.template = templateIDDelegateCreateProposal
		req.body = fmt.Sprintf("Your delegate created a proposal: %s", pr.Title)
	case DelegateVotingVoted:
		req.template = templateIDDelegateVotingVoted
		req.body = fmt.Sprintf("Your delegate voted on a proposal: %s", pr.Title)
	case DelegateVotingSkipVote:
		req.template = templateIDDelegateVotingSkipVote
		req.body = fmt.Sprintf("Your delegate skipped the vote: %s", pr.Title)
	}

	return req, nil
}

func prepareVotingEndsSoonNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return fmt.Sprintf("%s and %s", names[0], names[1])
	default:
		return fmt.Sprintf("%s, %s and %s", names[0], names[1], names[2])
	}
}

func (s *Service) getNotVotedDetails(ctx context.Context, userID uuid.UUID, details []SendQueue) ([]SendQueue, error) {
	usr, err := s.usrs.GetUserProfile(ctx, &inboxapi.GetUserProfileRequest{UserId: userID.String()})
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("s.usrs.GetUserProfile: %w", err)
	}

	skippedIDs := make([]string, 0, len(details))
	if usr.GetUser().GetAddress() != "" {
		list := make([]string, 0, len(details))
		for _, item := range details {
			list = append(list, item.ProposalID)
		}

		res, err := s.core.GetUserVotes(ctx, usr.GetUser().GetAddress(), goverlandcorewebsdk.GetUserVotesRequest{
			ProposalIDs: list,
			Limit:       len(list),
		})

		if err != nil {
			return nil, fmt.Errorf("s.core.GetUserVotes: %w", err)
		}

		for _, pr := range res.Items {
			skippedIDs = append(skippedIDs, pr.ProposalID)
		}
	}

	response := make([]SendQueue, 0, len(details))
	for _, info := range details {
		if slices.Contains(skippedIDs, info.ProposalID) {
			continue
		}

		response = append(response, info)
	}

	return response, nil
}

func (s *Service) getAllowedSendActions(userID uuid.UUID) (Actions, error) {
	result := make(Actions, 0, 10)
	details, err := s.settings.GetPushDetails(context.Background(), &inboxapi.GetPushDetailsRequest{UserId: userID.String()})
	if err != nil {
		return nil, fmt.Errorf("s.settings.GetPushDetails: %w", err)
	}

	daoSettings := details.GetDao()
	if daoSettings == nil {
		return result, nil
	}

	if daoSettings.GetVoteFinished() {
		result = append(result, ProposalVotingEnded)
	}

	if daoSettings.GetVoteFinishesSoon() {
		result = append(result, ProposalVotingEndsSoon)
	}

	if daoSettings.GetQuorumReached() {
		result = append(result, ProposalVotingQuorumReached)
	}

	if daoSettings.GetNewProposalCreated() {
		result = append(result, ProposalCreated)
	}

	return result, nil
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

	dao, err := s.core.GetDao(ctx, id.String())
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
	return fmt.Sprintf("https://cdn.stamp.fyi/space/%s?s=%d", alias, 180)
}
