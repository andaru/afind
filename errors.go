package afind

import (
	"fmt"
)

// Error types

// Index is already available
type IndexAvailableError struct {
	key string
}

func newIndexAvailableError(key string) *IndexAvailableError {
	return &IndexAvailableError{key: key}
}

func (e *IndexAvailableError) Error() string {
	return "Cannot replace existing repository with key '" + e.key + "'"
}

func IsIndexAvailableError(e error) bool {
	if _, ok := e.(*IndexAvailableError); ok {
		return true
	}
	return false
}

// There was an error regarding the value of some argunent
type ValueError struct {
	arg string
	msg string
}

func newValueError(arg, msg string) *ValueError {
	return &ValueError{arg: arg, msg: msg}
}

func (e ValueError) Error() string {
	return fmt.Sprintf(`Value for argument '%s' is invalid: %s`,
		e.arg, e.msg)
}

func IsValueError(e error) bool {
	if _, ok := e.(*ValueError); ok {
		return true
	}
	return false
}

// No repositories were available to search in
type NoRepoAvailableError struct{}

func newNoRepoAvailableError() *NoRepoAvailableError {
	return &NoRepoAvailableError{}
}

func (e NoRepoAvailableError) Error() string {
	return "No Repo available to search"
}

func IsNoRepoAvailableError(e error) bool {
	if _, ok := e.(*NoRepoAvailableError); ok {
		return true
	}
	return false
}
