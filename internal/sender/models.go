package sender

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	DaoCreated                  Action = "dao.created"
	DaoUpdated                  Action = "dao.updated"
	ProposalCreated             Action = "proposal.created"
	ProposalUpdated             Action = "proposal.updated"
	ProposalVotingStartsSoon    Action = "proposal.voting.starts_soon"
	ProposalVotingEndsSoon      Action = "proposal.voting.ends_soon"
	ProposalVotingStarted       Action = "proposal.voting.started"
	ProposalVotingQuorumReached Action = "proposal.voting.quorum_reached"
	ProposalVotingEnded         Action = "proposal.voting.ended"
)

const (
	templateIDVoteFinishesSoon  templateID = 1
	templateIDOneDaoOneProposal templateID = 2
	templateIDOneDaoFewProposal templateID = 3
	templateIDTwoDao            templateID = 4
	templateIDFewDao            templateID = 5
)

type Type string

type Action string

type templateID int

type request struct {
	token     string
	body      string
	title     string
	imageURL  string
	userID    uuid.UUID
	payload   json.RawMessage
	proposals []string
	template  templateID
}

type Message struct {
	ID         uuid.UUID       `json:"id"`
	Title      string          `json:"title"`
	Body       string          `json:"body"`
	ImageURL   string          `json:"image_url"`
	Payload    json.RawMessage `json:"payload"`
	TemplateID templateID      `json:"template_id"`
}

type History struct {
	gorm.Model

	UserID       uuid.UUID
	Message      Message `gorm:"serializer:json"`
	PushResponse string
	Hash         string
	ClickedAt    time.Time
}

type Item struct {
	DaoID      uuid.UUID `json:"dao_id"`
	ProposalID string    `json:"proposal_id"`
	Action     Action    `json:"action"`
}

func (i Item) DAO() bool {
	return i.ProposalID == ""
}

func (i Item) AllowSending() bool {
	if i.DAO() {
		return false
	}

	switch i.Action {
	case ProposalCreated,
		ProposalVotingQuorumReached,
		ProposalVotingEndsSoon,
		ProposalVotingEnded:
		return true
	}

	return false
}

func (i Item) VotingEndsSoon() bool {
	return i.Action == ProposalVotingEndsSoon
}

type SendQueue struct {
	gorm.Model

	UserID     uuid.UUID
	DaoID      uuid.UUID
	ProposalID string
	Action     Action
	SentAt     *time.Time
}

func (SendQueue) TableName() string {
	return "send_queue"
}
