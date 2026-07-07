// Copyright (c) the go-ruby-rolify/rolify authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rolify

import "testing"

// names collects the role names from a slice, preserving order.
func names(rs []*Role) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGlobalRoleMatchesAnyQuery(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("admin", Global())

	for _, s := range []Scope{
		Global(),
		ClassScope("Forum"),
		InstanceScope("Forum", "7"),
		Any(),
	} {
		if !u.HasRole("admin", s) {
			t.Errorf("global admin should match query scope %+v", s)
		}
	}
	// Wrong name never matches.
	if u.HasRole("editor", Global()) {
		t.Error("should not have editor")
	}
}

func TestClassRoleScope(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("moderator", ClassScope("Forum"))

	// Matches the class itself, any instance of it, and :any.
	if !u.HasRole("moderator", ClassScope("Forum")) {
		t.Error("class role should match the class query")
	}
	if !u.HasRole("moderator", InstanceScope("Forum", "42")) {
		t.Error("class role should match any instance of the class")
	}
	if !u.HasRole("moderator", Any()) {
		t.Error("class role should match :any")
	}
	// Does not match a global query, another class, or another instance type.
	if u.HasRole("moderator", Global()) {
		t.Error("class role must not satisfy a global query")
	}
	if u.HasRole("moderator", ClassScope("Group")) {
		t.Error("class role must not match a different class")
	}
	if u.HasRole("moderator", InstanceScope("Group", "1")) {
		t.Error("class role must not match a different class instance")
	}
}

func TestInstanceRoleScope(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("owner", InstanceScope("Forum", "7"))

	if !u.HasRole("owner", InstanceScope("Forum", "7")) {
		t.Error("instance role should match the exact instance")
	}
	if !u.HasRole("owner", Any()) {
		t.Error("instance role should match :any")
	}
	// Not a different instance, not the class query, not global.
	if u.HasRole("owner", InstanceScope("Forum", "8")) {
		t.Error("instance role must not match a different instance")
	}
	if u.HasRole("owner", ClassScope("Forum")) {
		t.Error("instance role must not satisfy a class query")
	}
	if u.HasRole("owner", Global()) {
		t.Error("instance role must not satisfy a global query")
	}
	if u.HasRole("owner", InstanceScope("Group", "7")) {
		t.Error("instance role must not match a different type")
	}
}

func TestAddRoleIdempotent(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store)
	u := rf.User("1")
	r1 := u.AddRole("admin", Global())
	r2 := u.AddRole("admin", Global())
	if r1.ID != r2.ID {
		t.Errorf("duplicate add should reuse the role: %d != %d", r1.ID, r2.ID)
	}
	if got := len(store.AllRoles()); got != 1 {
		t.Errorf("duplicate add must not create a second role: got %d", got)
	}
	if got := len(u.Roles()); got != 1 {
		t.Errorf("duplicate add must not double-join: got %d", got)
	}
}

func TestRemoveRole(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("admin", Global())

	if !u.RemoveRole("admin", Global()) {
		t.Error("removing a held role should report true")
	}
	if u.HasRole("admin", Global()) {
		t.Error("role should be gone after removal")
	}
	// Removing again: the role row still exists but is no longer joined.
	if u.RemoveRole("admin", Global()) {
		t.Error("removing an unheld (but existing) role should report false")
	}
	// Removing a role that was never created at all.
	if u.RemoveRole("ghost", Global()) {
		t.Error("removing a nonexistent role should report false")
	}
}

func TestRemoveRoleHeldByAnotherUser(t *testing.T) {
	rf := New(NewMemoryStore())
	a := rf.User("a")
	b := rf.User("b")
	a.AddRole("admin", Global())
	// b never held it, though the role row exists.
	if b.RemoveRole("admin", Global()) {
		t.Error("user without the join should get false")
	}
	if !a.HasRole("admin", Global()) {
		t.Error("owner should keep the role")
	}
}

func TestHasAllRoles(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("admin", Global())
	u.AddRole("moderator", ClassScope("Forum"))

	if !u.HasAllRoles(
		Query{"admin", Global()},
		Query{"moderator", ClassScope("Forum")},
	) {
		t.Error("should have all listed roles")
	}
	if u.HasAllRoles(
		Query{"admin", Global()},
		Query{"editor", Global()},
	) {
		t.Error("must be false when one role is missing")
	}
	if u.HasAllRoles() {
		t.Error("has_all_roles? with no queries should be false")
	}
}

func TestHasAnyRole(t *testing.T) {
	rf := New(NewMemoryStore())
	u := rf.User("1")
	u.AddRole("admin", Global())

	if !u.HasAnyRole(
		Query{"editor", Global()},
		Query{"admin", Global()},
	) {
		t.Error("should match at least one role")
	}
	if u.HasAnyRole(
		Query{"editor", Global()},
		Query{"writer", Global()},
	) {
		t.Error("must be false when no role matches")
	}
	if u.HasAnyRole() {
		t.Error("has_any_role? with no queries should be false")
	}
}

func TestAddRoleScopeTargets(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store)
	u := rf.User("1")
	u.AddRole("g", Global())
	u.AddRole("c", ClassScope("Forum"))
	u.AddRole("i", InstanceScope("Forum", "3"))
	u.AddRole("w", Any()) // :any assignment is treated as global

	byName := map[string]*Role{}
	for _, r := range store.AllRoles() {
		byName[r.Name] = r
	}
	check := func(name, wantType, wantID string) {
		r := byName[name]
		if r.ResourceType != wantType || r.ResourceID != wantID {
			t.Errorf("%s: got (%q,%q) want (%q,%q)", name, r.ResourceType, r.ResourceID, wantType, wantID)
		}
	}
	check("g", "", "")
	check("c", "Forum", "")
	check("i", "Forum", "3")
	check("w", "", "")
}

func TestStrictMode(t *testing.T) {
	rf := New(NewMemoryStore(), Strict(true))
	u := rf.User("1")
	u.AddRole("admin", Global())
	u.AddRole("moderator", ClassScope("Forum"))

	// Global and :any queries still use inheriting semantics.
	if !u.HasRole("admin", Global()) {
		t.Error("strict: global query should match global role")
	}
	if !u.HasRole("admin", Any()) {
		t.Error("strict: :any should still match")
	}
	// Strict class/instance: no inheritance from a broader role.
	if u.HasRole("admin", ClassScope("Forum")) {
		t.Error("strict: global role must not satisfy a class query")
	}
	if u.HasRole("admin", InstanceScope("Forum", "1")) {
		t.Error("strict: global role must not satisfy an instance query")
	}
	if u.HasRole("moderator", InstanceScope("Forum", "1")) {
		t.Error("strict: class role must not satisfy an instance query")
	}
	// Exact strict matches still succeed.
	if !u.HasRole("moderator", ClassScope("Forum")) {
		t.Error("strict: exact class query should match class role")
	}
	u.AddRole("owner", InstanceScope("Forum", "9"))
	if !u.HasRole("owner", InstanceScope("Forum", "9")) {
		t.Error("strict: exact instance query should match instance role")
	}
	// Strict name mismatch and type mismatch.
	if u.HasRole("owner", InstanceScope("Forum", "10")) {
		t.Error("strict: wrong id must not match")
	}
	if u.HasRole("ghost", ClassScope("Forum")) {
		t.Error("strict: wrong name must not match")
	}
	if u.HasRole("moderator", ClassScope("Group")) {
		t.Error("strict: wrong class must not match")
	}
}

func TestRoleCache(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store, Cache(true))
	u := rf.User("1")
	u.AddRole("admin", Global())

	// Warm the cache.
	if !u.HasRole("admin", Global()) {
		t.Error("cached read should see the role")
	}
	if got := len(u.Roles()); got != 1 {
		t.Errorf("cached Roles() got %d", got)
	}
	// A mutation invalidates the cache, so a fresh add is visible.
	u.AddRole("editor", Global())
	if !u.HasRole("editor", Global()) {
		t.Error("cache should reload after a mutation")
	}
	// Removal also invalidates.
	u.RemoveRole("editor", Global())
	if u.HasRole("editor", Global()) {
		t.Error("cache should reflect removal")
	}
}

func TestHasCachedRole(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store) // cache option OFF; has_cached_role? still caches its own snapshot
	u := rf.User("1")
	u.AddRole("admin", Global())

	if !u.HasCachedRole("admin", Global()) {
		t.Error("cached role check should find admin")
	}
	if u.HasCachedRole("editor", Global()) {
		t.Error("cached role check should not find editor")
	}
	// Mutate the store directly, behind the user's back: the warm snapshot must
	// not see it until invalidated.
	other := rf.User("1")
	other.AddRole("editor", Global())
	if u.HasCachedRole("editor", Global()) {
		t.Error("stale cached snapshot must not see out-of-band changes")
	}
	// After the same user mutates (invalidating), the snapshot refreshes.
	u.AddRole("writer", Global())
	if !u.HasCachedRole("editor", Global()) {
		t.Error("after invalidation the cached snapshot should refresh")
	}
}

func TestAppliedRoles(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store)
	u := rf.User("1")
	u.AddRole("super", Global())
	u.AddRole("mod", ClassScope("Forum"))
	u.AddRole("owner", InstanceScope("Forum", "7"))
	u.AddRole("owner", InstanceScope("Forum", "8")) // different instance, excluded
	u.AddRole("mod", ClassScope("Group"))           // different class, excluded

	got := names(rf.AppliedRoles("Forum", "7"))
	want := []string{"super", "mod", "owner"}
	if !equal(got, want) {
		t.Errorf("AppliedRoles = %v, want %v", got, want)
	}
}

func TestRolesToAdministrate(t *testing.T) {
	store := NewMemoryStore()
	rf := New(store)
	u := rf.User("1")
	u.AddRole("super", Global())                    // global excluded
	u.AddRole("mod", ClassScope("Forum"))           // class included
	u.AddRole("owner", InstanceScope("Forum", "7")) // this instance included
	u.AddRole("owner", InstanceScope("Forum", "8")) // other instance excluded
	u.AddRole("mod", ClassScope("Group"))           // other class excluded

	got := names(rf.RolesToAdministrate("Forum", "7"))
	want := []string{"mod", "owner"}
	if !equal(got, want) {
		t.Errorf("RolesToAdministrate = %v, want %v", got, want)
	}
}
