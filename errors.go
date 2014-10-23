package afind

import (
	"fmt"
)

// Error types

// Repo is already available and cannot be replaced
type RepoExistsError struct {
	key string
}

func newRepoExistsError(key string) *RepoExistsError {
	return &RepoExistsError{key: key}
}

func (e *RepoExistsError) Error() string {
	return "Cannot replace existing repository with key '" + e.key + "'"
}

func IsRepoExistsError(e error) bool {
	if _, ok := e.(*RepoExistsError); ok {
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
	return fmt.Sprintf("Argument '%s' value is invalid: %s", e.arg, e.msg)
}

func IsValueError(e error) bool {
	if _, ok := e.(*ValueError); ok {
		return true
	}
	return false
}

// No specific repos found when requesting some subset of repos
type NoRepoFoundError struct{}

func newNoRepoFoundError() *NoRepoFoundError {
	return &NoRepoFoundError{}
}

func (e NoRepoFoundError) Error() string {
	return "No Repo found"
}

func IsNoRepoFoundError(e error) bool {
	if _, ok := e.(*NoRepoFoundError); ok {
		return true
	}
	return false
}

// No RPC client available for remote searches
type NoRpcClientError struct{}

func newNoRpcClientError() *NoRpcClientError {
	return &NoRpcClientError{}
}

func (e NoRpcClientError) Error() string {
	return "No local RPC client to perform remote requests"
}

func IsNoRpcClientError(e error) bool {
	if _, ok := e.(*NoRpcClientError); ok {
		return true
	}
	return false
}

// A timeout occurred
type TimeoutError struct {
	what string
}

func newTimeoutError(what string) *TimeoutError {
	return &TimeoutError{what: what}
}

func (e TimeoutError) Error() string {
	if e.what != "" {
		return e.what + " timed out"
	}
	return "timed out"
}

func IsTimeoutError(e error) bool {
	if _, ok := e.(*TimeoutError); ok {
		return true
	}
	return false
}

// HTTP error representations for JSON/rest interface

type ErrorHttp struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e ErrorHttp) Error() string {
	return e.Type + ": " + e.Message
}

func newErrorHttp(e error) *ErrorHttp {
	switch e.(type) {
	default:
		return &ErrorHttp{Type: "unexpected", Message: e.Error()}
	case *TimeoutError:
		return &ErrorHttp{Type: "timeout", Message: e.Error()}
	case *NoRpcClientError:
		return &ErrorHttp{Type: "rpc_client_unavailable", Message: e.Error()}
	case *NoRepoFoundError:
		return &ErrorHttp{Type: "no_repo_found", Message: e.Error()}
	case *RepoExistsError:
		return &ErrorHttp{Type: "repo_exists", Message: e.Error()}
	case *ValueError:
		return &ErrorHttp{Type: "value_error", Message: e.Error()}
	}
}
