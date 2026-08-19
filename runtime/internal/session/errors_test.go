package session

import (
	"errors"
	"io/fs"
	"testing"
)

func TestIsNotFound(t *testing.T) {
	if IsNotFound(nil) {
		t.Fatal("nil should not match")
	}
	if !IsNotFound(ErrSessionLogNotFound) {
		t.Fatal("sentinel should match")
	}
	if !IsNotFound(errors.Join(ErrSessionLogNotFound, fs.ErrNotExist)) {
		t.Fatal("joined sentinel should match")
	}
	if !IsNotFound(fs.ErrNotExist) {
		t.Fatal("fs.ErrNotExist should match")
	}
	if IsNotFound(errors.New("read run state: file not found")) {
		t.Fatal("plain string error should not match without wrap")
	}
}

func TestIsNotFoundWrapped(t *testing.T) {
	wrapped := errors.Join(ErrSessionLogNotFound, fs.ErrNotExist)
	if !IsNotFound(wrapped) {
		t.Fatal("wrapped not found should match")
	}
}
