package jobs

import "errors"

var (
	ErrNotFound       = errors.New("job not found")
	ErrConflict       = errors.New("job state conflict")
	ErrNotEditable    = errors.New("job is not editable in current status")
	ErrNotAssigned    = errors.New("job is not assigned to this session")
	ErrAlreadyClaimed = errors.New("job is already claimed")
)
