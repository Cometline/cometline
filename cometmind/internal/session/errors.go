package session

import "errors"

var (
	// ErrSessionNotFound is returned when a session id does not exist.
	ErrSessionNotFound = errors.New("session not found")
	// ErrWorkspaceNotFound is returned when a workspace id or path does not exist.
	ErrWorkspaceNotFound = errors.New("workspace not found")
	// ErrActiveDelegation blocks workspace changes while a child delegation is in flight.
	ErrActiveDelegation = errors.New("cannot change workspace while delegation is active")
)
