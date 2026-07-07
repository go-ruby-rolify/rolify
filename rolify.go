// Copyright (c) the go-ruby-rolify/rolify authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rolify

// scopeKind classifies a query scope. The zero value is a global query.
type scopeKind int

const (
	scopeGlobal   scopeKind = iota // add_role :admin        / has_role? :admin
	scopeClass                     // add_role :mod, Forum   / has_role? :mod, Forum
	scopeInstance                  // add_role :mod, @forum  / has_role? :mod, @forum
	scopeAny                       // has_role? :admin, :any
)

// Scope names the target of a role query or assignment. Build one with [Global],
// [ClassScope], [InstanceScope], or [Any]. The zero Scope is [Global].
type Scope struct {
	kind scopeKind
	Type string
	ID   string
}

// Global is the unscoped query/assignment (Ruby: add_role :admin / has_role? :admin).
func Global() Scope { return Scope{kind: scopeGlobal} }

// ClassScope scopes to a resource class (Ruby: add_role :mod, Forum).
func ClassScope(resourceType string) Scope {
	return Scope{kind: scopeClass, Type: resourceType}
}

// InstanceScope scopes to a single resource instance (Ruby: add_role :mod, @forum).
func InstanceScope(resourceType, resourceID string) Scope {
	return Scope{kind: scopeInstance, Type: resourceType, ID: resourceID}
}

// Any is the wildcard query (Ruby: has_role? :admin, :any); it matches a role of
// any scope. Using it for an assignment is treated as [Global].
func Any() Scope { return Scope{kind: scopeAny} }

// target returns the (ResourceType, ResourceID) a role assignment should carry.
func (s Scope) target() (string, string) {
	switch s.kind {
	case scopeClass:
		return s.Type, ""
	case scopeInstance:
		return s.Type, s.ID
	default: // scopeGlobal, scopeAny
		return "", ""
	}
}

// roleMatches reports whether stored role r satisfies a query for name in scope s
// under rolify's default (inheriting) semantics.
func roleMatches(r *Role, name string, s Scope) bool {
	if r.Name != name {
		return false
	}
	// A global role matches any query.
	if r.ResourceType == "" && r.ResourceID == "" {
		return true
	}
	switch s.kind {
	case scopeAny:
		return true
	case scopeClass:
		// Class query: only a class role of the same type qualifies.
		return r.ResourceType == s.Type && r.ResourceID == ""
	case scopeInstance:
		// Instance query: a class role of the same type (id == "") or the exact
		// instance qualifies.
		return r.ResourceType == s.Type && (r.ResourceID == "" || r.ResourceID == s.ID)
	default: // scopeGlobal
		// Global query: only a global role qualifies (handled above).
		return false
	}
}

// strictMatch reports whether stored role r satisfies a class/instance query for
// name in scope s under strict_rolify: no scope inheritance, exact match only.
func strictMatch(r *Role, name string, s Scope) bool {
	if r.Name != name {
		return false
	}
	switch s.kind {
	case scopeClass:
		return r.ResourceType == s.Type && r.ResourceID == ""
	default: // scopeInstance
		return r.ResourceType == s.Type && r.ResourceID == s.ID
	}
}

// Rolify is the role engine bound to a [Store] and a set of options.
type Rolify struct {
	store  Store
	strict bool
	cache  bool
}

// Option configures a [Rolify] engine.
type Option func(*Rolify)

// Strict sets rolify's strict_rolify option. In strict mode a class/instance
// query is matched exactly: a broader (global or class) role no longer satisfies
// a narrower query.
func Strict(on bool) Option { return func(r *Rolify) { r.strict = on } }

// Cache sets rolify's role_cache option. When on, a [User] loads its roles once
// and reuses that snapshot until a mutation (AddRole / RemoveRole) invalidates it.
func Cache(on bool) Option { return func(r *Rolify) { r.cache = on } }

// New returns a role engine over store, applying opts.
func New(store Store, opts ...Option) *Rolify {
	rf := &Rolify{store: store}
	for _, opt := range opts {
		opt(rf)
	}
	return rf
}

// User returns a handle for the user identified by id. The handle carries the
// role cache (when the [Cache] option is set).
func (rf *Rolify) User(id string) *User {
	return &User{rf: rf, id: id}
}

// User is a rolify role holder: the object the gem mixes add_role / has_role? /
// remove_role into.
type User struct {
	rf     *Rolify
	id     string
	cache  []*Role
	cached bool
}

// currentRoles returns the user's roles, honouring the cache option.
func (u *User) currentRoles() []*Role {
	if u.rf.cache {
		if !u.cached {
			u.cache = u.rf.store.RolesFor(u.id)
			u.cached = true
		}
		return u.cache
	}
	return u.rf.store.RolesFor(u.id)
}

// invalidate drops any cached roles after a mutation.
func (u *User) invalidate() {
	u.cached = false
	u.cache = nil
}

// AddRole grants name in scope s (Ruby: add_role). It is idempotent: an existing
// role is reused and re-granting it changes nothing. It returns the role.
func (u *User) AddRole(name string, s Scope) *Role {
	t, id := s.target()
	r, ok := u.rf.store.FindRole(name, t, id)
	if !ok {
		r = u.rf.store.CreateRole(name, t, id)
	}
	u.rf.store.AssignRole(u.id, r)
	u.invalidate()
	return r
}

// HasRole reports whether the user has name in scope s (Ruby: has_role?). Under
// strict_rolify a class/instance query is matched exactly; otherwise scope
// inheritance applies (a global role satisfies any query, a class role satisfies
// its instances).
func (u *User) HasRole(name string, s Scope) bool {
	strict := u.rf.strict && (s.kind == scopeClass || s.kind == scopeInstance)
	for _, r := range u.currentRoles() {
		if strict {
			if strictMatch(r, name, s) {
				return true
			}
			continue
		}
		if roleMatches(r, name, s) {
			return true
		}
	}
	return false
}

// HasCachedRole reports whether the user has name in scope s using only the
// cached role snapshot (Ruby: has_cached_role?). The snapshot is loaded once on
// first use and never re-queries the store until a mutation invalidates it, so it
// answers without touching persistence even when the [Cache] option is off.
func (u *User) HasCachedRole(name string, s Scope) bool {
	if !u.cached {
		u.cache = u.rf.store.RolesFor(u.id)
		u.cached = true
	}
	for _, r := range u.cache {
		if roleMatches(r, name, s) {
			return true
		}
	}
	return false
}

// RemoveRole revokes name in scope s (Ruby: remove_role). Removing a role the
// user does not have — or one that does not exist — is a no-op; the returned bool
// reports whether a grant was removed.
func (u *User) RemoveRole(name string, s Scope) bool {
	t, id := s.target()
	r, ok := u.rf.store.FindRole(name, t, id)
	if !ok {
		return false
	}
	removed := u.rf.store.RemoveRole(u.id, r)
	u.invalidate()
	return removed
}

// Roles returns every role granted to the user (Ruby: user.roles).
func (u *User) Roles() []*Role { return u.currentRoles() }

// Query pairs a role name with a scope for the multi-role predicates.
type Query struct {
	Name  string
	Scope Scope
}

// HasAllRoles reports whether the user has every listed role (Ruby:
// has_all_roles?). With no queries it reports false.
func (u *User) HasAllRoles(queries ...Query) bool {
	if len(queries) == 0 {
		return false
	}
	for _, q := range queries {
		if !u.HasRole(q.Name, q.Scope) {
			return false
		}
	}
	return true
}

// HasAnyRole reports whether the user has at least one listed role (Ruby:
// has_any_role?).
func (u *User) HasAnyRole(queries ...Query) bool {
	for _, q := range queries {
		if u.HasRole(q.Name, q.Scope) {
			return true
		}
	}
	return false
}

// AppliedRoles returns the roles that apply to the resource instance identified
// by (resourceType, resourceID): instance-scoped roles on it, class-scoped roles
// on its class, and global roles (Ruby: @resource.applied_roles).
func (rf *Rolify) AppliedRoles(resourceType, resourceID string) []*Role {
	var out []*Role
	for _, r := range rf.store.AllRoles() {
		switch {
		case r.ResourceType == "" && r.ResourceID == "": // global
			out = append(out, r)
		case r.ResourceType == resourceType && r.ResourceID == "": // class
			out = append(out, r)
		case r.ResourceType == resourceType && r.ResourceID == resourceID: // instance
			out = append(out, r)
		}
	}
	return out
}

// RolesToAdministrate returns the roles that administrate the resource instance
// identified by (resourceType, resourceID): the roles scoped to this resource or
// to its class, excluding global roles (Ruby: @resource.roles_to_administrate).
func (rf *Rolify) RolesToAdministrate(resourceType, resourceID string) []*Role {
	var out []*Role
	for _, r := range rf.store.AllRoles() {
		if r.ResourceType != resourceType {
			continue
		}
		if r.ResourceID == "" || r.ResourceID == resourceID {
			out = append(out, r)
		}
	}
	return out
}
