// Copyright (c) the go-ruby-rolify/rolify authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rolify

import "testing"

func TestMemoryStoreRolesForUnknownUser(t *testing.T) {
	store := NewMemoryStore()
	if got := store.RolesFor("nobody"); got != nil {
		t.Errorf("RolesFor on an unknown user should be nil, got %v", got)
	}
}

func TestMemoryStoreFindAndCreate(t *testing.T) {
	store := NewMemoryStore()
	if _, ok := store.FindRole("admin", "", ""); ok {
		t.Error("empty store should not find a role")
	}
	r := store.CreateRole("admin", "", "")
	if r.ID != 1 {
		t.Errorf("first role ID should be 1, got %d", r.ID)
	}
	got, ok := store.FindRole("admin", "", "")
	if !ok || got.ID != r.ID {
		t.Errorf("FindRole should return the created role")
	}
	if all := store.AllRoles(); len(all) != 1 || all[0] != r {
		t.Errorf("AllRoles should return the single created role")
	}
}

func TestMemoryStoreAssignAndRemove(t *testing.T) {
	store := NewMemoryStore()
	r := store.CreateRole("admin", "", "")
	if store.RemoveRole("u", r) {
		t.Error("removing before any assignment should be false")
	}
	store.AssignRole("u", r)
	store.AssignRole("u", r) // idempotent
	if got := store.RolesFor("u"); len(got) != 1 {
		t.Errorf("assign should join exactly once, got %d", len(got))
	}
	if !store.RemoveRole("u", r) {
		t.Error("removing an assigned role should be true")
	}
	if store.RemoveRole("u", r) {
		t.Error("removing again should be false")
	}
}
