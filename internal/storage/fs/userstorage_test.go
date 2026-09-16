package fs

import (
	"errors"
	"testing"

	"github.com/ddvk/rmfakecloud/internal/config"
	"github.com/ddvk/rmfakecloud/internal/model"
	"github.com/ddvk/rmfakecloud/internal/storage"
)

func testStorage(t *testing.T) *FileSystemStorage {
	t.Helper()
	return NewStorage(&config.Config{DataDir: t.TempDir()})
}

// Callers that provision a user on first login have to tell "this user does not
// exist yet" apart from "the disk is unreadable". Without a sentinel both arrive
// as an opaque error and the safe reading of one is the dangerous reading of the
// other.
func TestGetUserReportsAMissingUserAsSuch(t *testing.T) {
	fs := testStorage(t)

	_, err := fs.GetUser("nobody")

	if !errors.Is(err, storage.ErrUserNotFound) {
		t.Errorf("got %v, want ErrUserNotFound", err)
	}
}

func TestGetUserDoesNotReportAnExistingUserAsMissing(t *testing.T) {
	fs := testStorage(t)
	user, err := model.NewUser("alice", "hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.RegisterUser(user); err != nil {
		t.Fatal(err)
	}

	got, err := fs.GetUser("alice")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "alice" {
		t.Errorf("got %q", got.ID)
	}
}

func TestGetUserRejectsAnEmptyID(t *testing.T) {
	fs := testStorage(t)

	if _, err := fs.GetUser(""); err == nil {
		t.Error("an empty uid must be an error")
	}
}
