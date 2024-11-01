package sender

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/goverland-labs/goverland-inbox-api-protocol/protobuf/inboxapi"
	"github.com/stretchr/testify/require"
	"go.openly.dev/pointy"
)

func Test_convertActionToTitle(t *testing.T) {
	for name, tc := range map[string]struct {
		in       Action
		expected string
	}{
		"proposal created": {
			in:       ProposalCreated,
			expected: "New proposal created",
		},
		"quorum reached": {
			in:       ProposalVotingQuorumReached,
			expected: "Quorum reached",
		},
		"vote finished": {
			in:       ProposalVotingEnded,
			expected: "Vote finished",
		},
		"default": {
			in:       ProposalVotingEndsSoon,
			expected: "have update on proposal",
		},
	} {
		t.Run(name, func(t *testing.T) {
			actual := convertActionToTitle(tc.in)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func Test_getAllowedSendActions(t *testing.T) {
	for name, tc := range map[string]struct {
		sp       func(*gomock.Controller) SettingsProvider
		expected Actions
		wantErr  bool
	}{
		"wrong response": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(nil, errors.New("internal error"))

				return sp
			},
			expected: nil,
			wantErr:  true,
		},
		"empty dao settings": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{}, nil)

				return sp
			},
			expected: Actions{},
			wantErr:  false,
		},
		"enabled only vote_finished": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						VoteFinished: pointy.Bool(true),
					},
				}, nil)

				return sp
			},
			expected: Actions{ProposalVotingEnded},
			wantErr:  false,
		},
		"disabled only vote_finished": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						VoteFinished: pointy.Bool(false),
					},
				}, nil)

				return sp
			},
			expected: Actions{},
			wantErr:  false,
		},
		"enabled only vote_finishes_soon": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						VoteFinishesSoon: pointy.Bool(true),
					},
				}, nil)

				return sp
			},
			expected: Actions{ProposalVotingEndsSoon},
			wantErr:  false,
		},
		"disabled only vote_finishes_soon": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						VoteFinishesSoon: pointy.Bool(false),
					},
				}, nil)

				return sp
			},
			expected: Actions{},
			wantErr:  false,
		},
		"enabled only quorum_reached": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						QuorumReached: pointy.Bool(true),
					},
				}, nil)

				return sp
			},
			expected: Actions{ProposalVotingQuorumReached},
			wantErr:  false,
		},
		"disabled only quorum_reached": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						QuorumReached: pointy.Bool(false),
					},
				}, nil)

				return sp
			},
			expected: Actions{},
			wantErr:  false,
		},
		"enabled only proposal_created": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						NewProposalCreated: pointy.Bool(true),
					},
				}, nil)

				return sp
			},
			expected: Actions{ProposalCreated},
			wantErr:  false,
		},
		"disabled only proposal_created": {
			sp: func(ctrl *gomock.Controller) SettingsProvider {
				sp := NewMockSettingsProvider(ctrl)

				sp.EXPECT().GetPushDetails(gomock.Any(), gomock.Any()).Return(&inboxapi.GetPushDetailsResponse{
					Dao: &inboxapi.PushSettingsDao{
						NewProposalCreated: pointy.Bool(false),
					},
				}, nil)

				return sp
			},
			expected: Actions{},
			wantErr:  false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			service := &Service{
				settings: tc.sp(ctrl),
			}

			actual, err := service.getAllowedSendActions(uuid.New())
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}
