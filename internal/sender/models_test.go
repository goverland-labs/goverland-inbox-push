package sender

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestItem_AllowSending(t *testing.T) {
	for name, tc := range map[string]struct {
		item    Item
		allowed bool
	}{
		"dao item": {
			item: Item{
				DaoID: uuid.New(),
			},
			allowed: false,
		},
		"proposal created": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalCreated,
			},
			allowed: true,
		},
		"proposal vote finishes soon": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalVotingEndsSoon,
			},
			allowed: true,
		},
		"proposal vote finished": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalVotingEnded,
			},
			allowed: true,
		},
		"proposal quorum reached": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalVotingQuorumReached,
			},
			allowed: true,
		},
		"proposal updated": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalUpdated,
			},
			allowed: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := tc.item.AllowSending()
			require.Equal(t, tc.allowed, actual)
		})
	}
}

func TestItem_VotingEndsSoon(t *testing.T) {
	for name, tc := range map[string]struct {
		item     Item
		expected bool
	}{
		"ends soon": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalVotingEndsSoon,
			},
			expected: true,
		},
		"created": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalCreated,
			},
			expected: false,
		},
		"quorum reached": {
			item: Item{
				DaoID:      uuid.New(),
				ProposalID: uuid.New().String(),
				Action:     ProposalVotingQuorumReached,
			},
			expected: false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := tc.item.VotingEndsSoon()
			require.Equal(t, tc.expected, actual)
		})
	}
}
