package errs

// Error types

const (
	MessageSvcFromContext = `Could not get Service from provided context`
)

// Repo is already available and cannot be replaced
type RepoExistsError struct {
	key string
}

func NewRepoExistsError(key string) *RepoExistsError {
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

func NewValueError(arg, msg string) *ValueError {
	return &ValueError{arg: arg, msg: msg}
}

func (e ValueError) Error() string {
	return "Argument '" + e.arg + "' value is invalid: " + e.msg
}

func IsValueError(e error) bool {
	if _, ok := e.(*ValueError); ok {
		return true
	}
	return false
}

// No specific repos found when requesting some subset of repos
type RepoUnavailableError struct{}

func NewRepoUnavailableError() *RepoUnavailableError {
	return &RepoUnavailableError{}
}

func (e RepoUnavailableError) Error() string {
	return "Repo not available"
}

func IsRepoUnavailableError(e error) bool {
	if _, ok := e.(*RepoUnavailableError); ok {
		return true
	}
	return false
}

// No RPC client available for remote searches
type NoRpcClientError struct{}

func NewNoRpcClientError() *NoRpcClientError {
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

func NewTimeoutError(what string) *TimeoutError {
	return &TimeoutError{what: what}
}

func (e TimeoutError) Error() string {
	s := "timed out"
	if e.what != "" {
		s += " waiting for " + e.what
	}
	return s
}

func IsTimeoutError(e error) bool {
	if _, ok := e.(*TimeoutError); ok {
		return true
	}
	return false
}

// An unexpected internal error occured
type InternalError string

func NewInternalError(s string) *InternalError {
	err := InternalError(s)
	return &err
}

func (e InternalError) Error() string {
	return string(e)
}

func IsInternalError(e error) bool {
	if _, ok := e.(*InternalError); ok {
		return true
	}
	return false
}

// An unexpected internal error occured
type InvalidRequestError string

func NewInvalidRequestError(s string) *InvalidRequestError {
	err := InvalidRequestError(s)
	return &err
}

func (e InvalidRequestError) Error() string {
	return string(e)
}

func IsInvalidRequestError(e error) bool {
	if _, ok := e.(*InvalidRequestError); ok {
		return true
	}
	return false
}

// Structured errors for JSON/etc interfaces
// where 'error' does not marshal.
type StructError struct {
	T string `json:"type"`
	M string `json:"message,omitempty"`
}

func (e StructError) Error() string {
	s := ""
	if e.T != "" {
		s += e.T
	}
	if e.M != "" {
		if s != "" {
			s += ": "
		}
		s += e.M
	}
	return s
}

func (e StructError) Type() string {
	return e.T
}

func (e StructError) Message() string {
	return e.M
}

func NewStructError(e error) *StructError {
	switch e.(type) {
	default:
		return &StructError{"unknown_error", e.Error()}
	case *InternalError:
		return &StructError{"internal_error", e.Error()}
	case *InvalidRequestError:
		return &StructError{"invalid_request", e.Error()}
	case *TimeoutError:
		return &StructError{"timeout", e.Error()}
	case *NoRpcClientError:
		return &StructError{"rpc_client_unavailable", e.Error()}
	case *RepoUnavailableError:
		return &StructError{"no_repo_found", e.Error()}
	case *RepoExistsError:
		return &StructError{"repo_exists", e.Error()}
	case *ValueError:
		return &StructError{"value_error", e.Error()}
	}
}
