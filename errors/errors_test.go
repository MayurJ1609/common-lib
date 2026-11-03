package errors

import (
	stderrors "errors"
	"testing"
)

func TestErrorFormattingAndUnwrap(t *testing.T) {
	cause := stderrors.New("cause")

	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{name: "without cause", err: &Error{Message: "invalid input"}, want: "invalid input"},
		{name: "with cause", err: &Error{Message: "operation failed", Cause: cause}, want: "operation failed: cause"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}

	err := &Error{Message: "operation failed", Cause: cause}
	if got := err.Unwrap(); got != cause {
		t.Fatalf("Unwrap() = %v, want original cause", got)
	}
	if got := (&Error{}).Unwrap(); got != nil {
		t.Fatalf("Unwrap() = %v, want nil", got)
	}
}

func TestNewWrapAndHasCode(t *testing.T) {
	created := New(CodeNotFound, "missing")
	if !HasCode(created, CodeNotFound) || HasCode(created, CodeInternal) {
		t.Fatal("HasCode did not identify the code on a new error")
	}

	cause := stderrors.New("database unavailable")
	wrapped := Wrap(CodeInternal, "load record", cause)
	if !HasCode(wrapped, CodeInternal) || !stderrors.Is(wrapped, cause) {
		t.Fatal("Wrap did not preserve its code and cause")
	}
	if Wrap(CodeInternal, "unused", nil) != nil {
		t.Fatal("Wrap(nil) should return nil")
	}
	if HasCode(cause, CodeInternal) || HasCode(nil, CodeInternal) {
		t.Fatal("HasCode matched an unrelated or nil error")
	}
}
