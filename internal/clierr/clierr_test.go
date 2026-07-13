package clierr

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := &Error{Code: CodeUser, Message: "bad input"}
	if e.Error() != "bad input" {
		t.Errorf("expected 'bad input', got %q", e.Error())
	}
}

func TestError_WithCause(t *testing.T) {
	cause := errors.New("disk full")
	e := &Error{Code: CodeSystem, Message: "write failed", Cause: cause}
	if e.Error() != "write failed: disk full" {
		t.Errorf("expected 'write failed: disk full', got %q", e.Error())
	}
}

func TestError_Unwrap(t *testing.T) {
	cause := errors.New("disk full")
	e := &Error{Code: CodeSystem, Message: "write failed", Cause: cause}
	if errors.Unwrap(e) != cause {
		t.Error("Unwrap should return the cause")
	}
}

func TestError_ExitCode(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{
		{CodeUser, ExitUser},
		{CodeSystem, ExitSystem},
		{CodeBundle, ExitBundle},
		{"unknown", ExitUser},
	}

	for _, tt := range tests {
		e := &Error{Code: tt.code}
		if e.ExitCode() != tt.want {
			t.Errorf("code %s: expected exit code %d, got %d", tt.code, tt.want, e.ExitCode())
		}
	}
}

func TestNew(t *testing.T) {
	e := New(CodeUser, "test")
	if e.Code != CodeUser {
		t.Errorf("expected CodeUser, got %s", e.Code)
	}
	if e.Message != "test" {
		t.Errorf("expected 'test', got %s", e.Message)
	}
	if e.Cause != nil {
		t.Errorf("expected nil cause, got %v", e.Cause)
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("io failure")
	e := Wrap(CodeSystem, "read failed", cause)
	if e.Code != CodeSystem {
		t.Errorf("expected CodeSystem, got %s", e.Code)
	}
	if e.Message != "read failed" {
		t.Errorf("expected 'read failed', got %s", e.Message)
	}
	if e.Cause != cause {
		t.Error("expected cause to match")
	}
}

func TestError_IsChain(t *testing.T) {
	cause := errors.New("disk full")
	e := Wrap(CodeSystem, "write failed", cause)

	// errors.Is should traverse
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find cause in chain")
	}
}

func TestAs_Error(t *testing.T) {
	cause := errors.New("io failure")
	e := Wrap(CodeSystem, "read failed", cause)

	var ce *Error
	if !errors.As(e, &ce) {
		t.Error("errors.As should extract *Error")
	}
	if ce.Code != CodeSystem {
		t.Errorf("expected CodeSystem, got %s", ce.Code)
	}
}
