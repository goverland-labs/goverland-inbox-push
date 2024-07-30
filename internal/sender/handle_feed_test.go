package sender

import (
	"testing"

	"github.com/google/uuid"
	"github.com/goverland-labs/goverland-platform-events/events/inbox"
	"github.com/stretchr/testify/assert"
)

func TestConvertPayloadActionToInternal(t *testing.T) {
	for name, tc := range map[string]struct {
		in       inbox.TimelineAction
		expected Action
	}{
		"dao_created": {
			in:       inbox.DaoCreated,
			expected: DaoCreated,
		},
		"dao_updated": {
			in:       inbox.DaoUpdated,
			expected: DaoUpdated,
		},
		"proposal_created": {
			in:       inbox.ProposalCreated,
			expected: ProposalCreated,
		},
		"proposal_updated": {
			in:       inbox.ProposalUpdated,
			expected: ProposalUpdated,
		},
		"proposal_voting_ended": {
			in:       inbox.ProposalVotingEnded,
			expected: ProposalVotingEnded,
		},
		"proposal_voting_started": {
			in:       inbox.ProposalVotingStarted,
			expected: ProposalVotingStarted,
		},
		"proposal_quorum_reached": {
			in:       inbox.ProposalVotingQuorumReached,
			expected: ProposalVotingQuorumReached,
		},
		"proposal_voting_starts_soon": {
			in:       inbox.ProposalVotingStartsSoon,
			expected: ProposalVotingStartsSoon,
		},
		"proposal_voting_ends_soon": {
			in:       inbox.ProposalVotingEndsSoon,
			expected: ProposalVotingEndsSoon,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := convertPayloadActionToInternal(tc.in)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestConvertPayloadToInternal(t *testing.T) {
	in := inbox.FeedPayload{
		DaoID:      uuid.New(),
		ProposalID: uuid.New().String(),
		Action:     inbox.ProposalCreated,
	}

	actual := convertPayloadToInternal(in)
	assert.Equal(t, convertPayloadActionToInternal(in.Action), actual.Action)
	assert.Equal(t, in.ProposalID, actual.ProposalID)
	assert.Equal(t, in.DaoID, actual.DaoID)
}
