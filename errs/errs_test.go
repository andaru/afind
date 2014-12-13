package errs

import (
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
	check(NewTimeoutError("thing"), "timeout")
	check(NewTimeoutError(""), "timeout")
	check(NewNoRepoFoundError(), "no_repo_found")
	check(NewNoRpcClientError(), "rpc_client_unavailable")
	check(NewRepoExistsError("repo_key"), "repo_exists")
	check(NewValueError("argument", "msg"), "value_error")
	check(errors.New("yeehaw"), "other")
}

func TestErrorString(t *testing.T) {
	check := func(e error, substr string) {
		err := NewStructError(e)
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("want substring %v, got string %v",
				substr, err.Error())
		}
	}
	check(NewNoRepoFoundError(), "no_repo_found: No Repo found")
	check(NewValueError("argument", "msg"),
		"value_error: Argument 'argument' value is invalid: msg")
	check(errors.New("foo"), "other: foo")
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

	err = NewNoRepoFoundError()
	if !IsNoRepoFoundError(err) {
		t.Error("got unexpected error type")
	}
	if IsNoRepoFoundError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = NewTimeoutError("thing")
	if !IsTimeoutError(err) {
		t.Error("got unexpected error type")
	}
	if IsTimeoutError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}
}
