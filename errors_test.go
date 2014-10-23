package afind

import (
	"errors"
	"strings"
	"testing"
)

func TestErrorHttpType(t *testing.T) {
	check := func(e error, expType string) {
		err := newErrorHttp(e)
		if err.Type != expType {
			t.Errorf("got error type %v, want %v", err.Type, expType)
		}
	}
	check(newTimeoutError(""), "timeout")
	check(newNoRepoFoundError(), "no_repo_found")
	check(newNoRpcClientError(), "rpc_client_unavailable")
	check(newRepoExistsError("repo_key"), "repo_exists")
	check(newValueError("argument", "msg"), "value_error")
	check(errors.New("yeehaw"), "unexpected")
}

func TestErrorString(t *testing.T) {
	check := func(e error, substr string) {
		err := newErrorHttp(e)
		if !strings.Contains(err.Error(), substr) {
			t.Errorf("want substring %v, got string %v",
				substr, err.Error())
		}
	}
	check(newNoRepoFoundError(), "no_repo_found: No Repo found")
	check(newValueError("argument", "msg"),
		"value_error: Argument 'argument' value is invalid: msg")
}

func TestErrorIs(t *testing.T) {
	basicerr := errors.New("")
	var err error

	err = newRepoExistsError("key")
	if !IsRepoExistsError(err) {
		t.Error("got unexpected error type")
	}
	if IsRepoExistsError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = newValueError("arg", "msg")
	if !IsValueError(err) {
		t.Error("got unexpected error type")
	}
	if IsValueError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = newNoRpcClientError()
	if !IsNoRpcClientError(err) {
		t.Error("got unexpected error type")
	}
	if IsNoRpcClientError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = newNoRepoFoundError()
	if !IsNoRepoFoundError(err) {
		t.Error("got unexpected error type")
	}
	if IsNoRepoFoundError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}

	err = newTimeoutError("thing")
	if !IsTimeoutError(err) {
		t.Error("got unexpected error type")
	}
	if IsTimeoutError(basicerr) {
		t.Error("got afind error type for an errors.New()")
	}
}
