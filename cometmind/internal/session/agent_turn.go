package session

// AgentTurn identifies which persisted session and model the agent runner should use.
type AgentTurn struct {
	ID         string
	ModelID    string
	ProviderID string
}

// AgentTurnFromSession builds a turn handle from a loaded session row.
func AgentTurnFromSession(sess Session) AgentTurn {
	return AgentTurn{
		ID:         sess.ID,
		ModelID:    sess.ModelID,
		ProviderID: sess.ProviderID,
	}
}
