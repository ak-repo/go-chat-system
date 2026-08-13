package errs

import (
	"errors"
	"reflect"
	"testing"
)

func TestWrapPreservesErrorsIs(t *testing.T) {
	err := Wrap("service.UserService.Register", Wrap("repository.UserRepository.CreateUser", ErrConflict))

	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected wrapped error to match sentinel")
	}
}

func TestTraceReturnsOperationsOutermostFirst(t *testing.T) {
	err := Wrap("service.UserService.Register", Wrap("repository.UserRepository.CreateUser", ErrDatabase))

	got := Trace(err)
	want := []string{"service.UserService.Register", "repository.UserRepository.CreateUser"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Trace() = %v, want %v", got, want)
	}
}

func TestTraceReturnsEmptyForUnwrappedError(t *testing.T) {
	if got := Trace(ErrInternal); len(got) != 0 {
		t.Fatalf("Trace() = %v, want empty", got)
	}
}
