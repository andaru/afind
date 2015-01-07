package errs

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStructErrorType(t *testing.T) {
	check := func(e error, expType string) {
		err := NewStructError(e)
		if err.T != expType {
			t.Errorf("got error type %v, want %v", err.T, expType)
		}
	}
	check(NewInternalError("thing"), "internal_error")
	check(NewInvalidRequestError("thing"), "invalid_request")
	check(NewTimeoutError("thing"), "timeout")
	check(NewTimeoutError(""), "timeout")
	check(NewRepoUnavailableError(), "no_repo_found")
	check(NewNoRpcClientError(), "rpc_client_unavailable")
	check(NewRepoExistsError("repo_key"), "repo_exists")
	check(NewValueError("argument", "msg"), "value_error")
	check(errors.New("yeehaw"), "unknown_error")
}

func TestErrorString(t *testing.T) {
	check := func(e error, substr string) {
		err := NewStructError(e)
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("want substring %v, got string %v",
				substr, err.Error())
		}
	}
	check(NewRepoUnavailableError(), "no_repo_found: Repo not available")
	check(NewValueError("argument", "msg"),
		"value_error: Argument 'argument' value is invalid: msg")
	check(errors.New("foo"), "unknown_error: foo")
}

func TestErrorIs(t *testing.T) {
	basicerr := errors.New("")
	var err error

	err = NewRepoExistsError("key")
	if !IsRepoExistsError(err) {
		t.Error("got unexpected error type")
	}
	if IsRepoExistsError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewValueError("arg", "msg")
	if !IsValueError(err) {
		t.Error("got unexpected error type")
	}
	if IsValueError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewNoRpcClientError()
	if !IsNoRpcClientError(err) {
		t.Error("got unexpected error type")
	}
	if IsNoRpcClientError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewRepoUnavailableError()
	if !IsRepoUnavailableError(err) {
		t.Error("got unexpected error type")
	}
	if IsRepoUnavailableError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewTimeoutError("thing")
	if !IsTimeoutError(err) {
		t.Error("got unexpected error type")
	}
	if IsTimeoutError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewInternalError("thing")
	if !IsInternalError(err) {
		t.Error("got unexpected error type")
	}
	if IsInternalError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewInvalidRequestError("thing")
	if !IsInvalidRequestError(err) {
		t.Error("got unexpected error type")
	}
	if IsInvalidRequestError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

}

func TestStructErrorAsJson(t *testing.T) {
	check := func(e *StructError, expected string) {
		if b, err := json.Marshal(e); err != nil {
			t.Errorf("want no error, got %v", err)
		} else if string(b) != expected {
			t.Errorf("want %s, got %s", expected, string(b))
		}
	}

	check(NewStructError(NewInvalidRequestError("bar")),
		`{"type":"invalid_request","message":"bar"}`)
	check(NewStructError(NewInternalError("foo")),
		`{"type":"internal_error","message":"foo"}`)
	check(NewStructError(NewRepoUnavailableError()),
		`{"type":"no_repo_found","message":"Repo not available"}`)
	check(NewStructError(NewNoRpcClientError()),
		`{"type":"rpc_client_unavailable","message":"No local RPC client to perform remote requests"}`)
	check(NewStructError(NewRepoExistsError("key")),
		`{"type":"repo_exists","message":"Cannot replace existing repository with key 'key'"}`)
	check(NewStructError(NewTimeoutError("foo")),
		`{"type":"timeout","message":"timed out waiting for foo"}`)
	check(NewStructError(NewValueError("a", "b")),
		`{"type":"value_error","message":"Argument 'a' value is invalid: b"}`)
}
