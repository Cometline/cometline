package gateway

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/id"
)

const jobProposalTTL = 30 * time.Minute

// JobProposalPayload is parsed from propose_job tool output.
type JobProposalPayload struct {
	Status           string `json:"status"`
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	DefaultWorkspace string `json:"default_workspace"`
}

// PendingJobProposal tracks a Discord job confirmation flow.
type PendingJobProposal struct {
	ID                string
	Description       string
	DefinitionOfDone  string
	DefaultWorkspace  string
	SelectedWorkspace string
	UserID            string
	ChannelID         string
	ThreadID          string
	SessionID         string
	SourceChannelID   string
	CreatedAt         time.Time
}

// JobProposalStore holds in-flight Discord job proposals.
type JobProposalStore struct {
	mu        sync.Mutex
	byID      map[string]*PendingJobProposal
	byChannel map[string]string
}

func NewJobProposalStore() *JobProposalStore {
	return &JobProposalStore{
		byID:      make(map[string]*PendingJobProposal),
		byChannel: make(map[string]string),
	}
}

func ParseJobProposalOutput(output string) (*JobProposalPayload, bool) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, false
	}
	var payload JobProposalPayload
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return nil, false
	}
	if payload.Status != "awaiting_workspace" {
		return nil, false
	}
	if strings.TrimSpace(payload.Description) == "" {
		return nil, false
	}
	return &payload, true
}

func (s *JobProposalStore) Put(msg InboundMessage, payload JobProposalPayload, sessionID, sourceChannelID, sessionWorkspace string) *PendingJobProposal {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now())

	proposalID := id.New()
	defaultWS := strings.TrimSpace(payload.DefaultWorkspace)
	if defaultWS == "" {
		defaultWS = strings.TrimSpace(sessionWorkspace)
	}
	channelKey := deliveryChannelID(msg)
	if prevID, ok := s.byChannel[channelKey]; ok {
		delete(s.byID, prevID)
	}

	p := &PendingJobProposal{
		ID:               proposalID,
		Description:      strings.TrimSpace(payload.Description),
		DefinitionOfDone: strings.TrimSpace(payload.DefinitionOfDone),
		DefaultWorkspace: defaultWS,
		SelectedWorkspace: defaultWS,
		UserID:           msg.UserID,
		ChannelID:        msg.ChannelID,
		ThreadID:         msg.ThreadID,
		SessionID:        sessionID,
		SourceChannelID:  sourceChannelID,
		CreatedAt:        time.Now(),
	}
	s.byID[proposalID] = p
	s.byChannel[channelKey] = proposalID
	return p
}

func (s *JobProposalStore) Get(proposalID string) (*PendingJobProposal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(time.Now())
	p, ok := s.byID[proposalID]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

func (s *JobProposalStore) SetWorkspace(proposalID, workspace string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byID[proposalID]
	if !ok {
		return false
	}
	p.SelectedWorkspace = strings.TrimSpace(workspace)
	return true
}

func (s *JobProposalStore) Remove(proposalID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.byID[proposalID]
	if !ok {
		return
	}
	delete(s.byID, proposalID)
	channelKey := deliveryChannelID(InboundMessage{ChannelID: p.ChannelID, ThreadID: p.ThreadID})
	if s.byChannel[channelKey] == proposalID {
		delete(s.byChannel, channelKey)
	}
}

func (s *JobProposalStore) pruneLocked(now time.Time) {
	for id, p := range s.byID {
		if now.Sub(p.CreatedAt) > jobProposalTTL {
			delete(s.byID, id)
			channelKey := deliveryChannelID(InboundMessage{ChannelID: p.ChannelID, ThreadID: p.ThreadID})
			if s.byChannel[channelKey] == id {
				delete(s.byChannel, channelKey)
			}
		}
	}
}

func JobProposalSelectCustomID(proposalID string) string {
	return "job_ps:" + proposalID
}

func JobProposalConfirmCustomID(proposalID string) string {
	return "job_pc:" + proposalID
}

func JobProposalCancelCustomID(proposalID string) string {
	return "job_px:" + proposalID
}

func ParseJobProposalCustomID(customID string) (action, proposalID string, ok bool) {
	customID = strings.TrimSpace(customID)
	switch {
	case strings.HasPrefix(customID, "job_ps:"):
		return "select", strings.TrimPrefix(customID, "job_ps:"), true
	case strings.HasPrefix(customID, "job_pc:"):
		return "confirm", strings.TrimPrefix(customID, "job_pc:"), true
	case strings.HasPrefix(customID, "job_px:"):
		return "cancel", strings.TrimPrefix(customID, "job_px:"), true
	default:
		return "", "", false
	}
}
