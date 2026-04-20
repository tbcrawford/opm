package store

import "testing"

var deleteProfileOverride func(s *Store, name string, force bool) error
var renameProfileAndRetargetOverride func(s *Store, oldName, newName string) (RenameProfileResult, error)

func TestHookDeleteProfile(t *testing.T, fn func(s *Store, name string, force bool) error) func() {
	t.Helper()
	prev := deleteProfileOverride
	deleteProfileOverride = fn
	return func() {
		deleteProfileOverride = prev
	}
}

func TestHookRenameProfileAndRetarget(t *testing.T, fn func(s *Store, oldName, newName string) (RenameProfileResult, error)) func() {
	t.Helper()
	prev := renameProfileAndRetargetOverride
	renameProfileAndRetargetOverride = fn
	return func() {
		renameProfileAndRetargetOverride = prev
	}
}
