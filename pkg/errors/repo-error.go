package errors

import stderrors "errors"

var (
	// ErrRepoNotFound is returned by repositories when a record does not exist.
	ErrRepoNotFound = stderrors.New("record not found")
	// ErrRepoConflict signals an optimistic-concurrency or uniqueness failure.
	ErrRepoConflict = stderrors.New("version conflict")
)

// IsNotFound reports whether err wraps ErrRepoNotFound.
func IsNotFound(err error) bool { return stderrors.Is(err, ErrRepoNotFound) }

// IsConflict reports whether err wraps ErrRepoConflict.
func IsConflict(err error) bool { return stderrors.Is(err, ErrRepoConflict) }
