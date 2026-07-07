// Copyright (c) the go-ruby-rolify/rolify authors
//
// SPDX-License-Identifier: BSD-3-Clause

package rolify

// Role is rolify's role value model: a Name plus an optional scope expressed as a
// ResourceType and ResourceID. The empty string denotes an absent scope
// component (rolify stores SQL NULL). ID is the store-assigned identity used for
// the users↔roles join; the value the gem cares about is the (Name, ResourceType,
// ResourceID) triple.
//
//   - Global role:   ResourceType == "" and ResourceID == "".
//   - Class role:    ResourceType set, ResourceID == "".
//   - Instance role: ResourceType and ResourceID set.
type Role struct {
	ID           int64
	Name         string
	ResourceType string
	ResourceID   string
}

// Store is the persistence seam. It models rolify's roles table and its
// users↔roles join over the [Role] value model. The in-repo [MemoryStore] backs
// the tests; a host binding wires it to ActiveRecord.
type Store interface {
	// FindRole returns the role matching the exact (name, resourceType,
	// resourceID) triple, and whether it was found.
	FindRole(name, resourceType, resourceID string) (*Role, bool)
	// CreateRole persists a new role for the triple and returns it.
	CreateRole(name, resourceType, resourceID string) *Role
	// AssignRole joins userID to role. It is idempotent: joining an
	// already-joined role is a no-op.
	AssignRole(userID string, role *Role)
	// RemoveRole unjoins userID from role, reporting whether a join existed.
	RemoveRole(userID string, role *Role) bool
	// RolesFor returns every role joined to userID, in a deterministic order.
	RolesFor(userID string) []*Role
	// AllRoles returns every persisted role, in a deterministic order. It backs
	// the resource-side helpers (applied_roles / roles_to_administrate).
	AllRoles() []*Role
}

// MemoryStore is an allocation-only [Store]: a slice of roles plus a
// userID→role-ID join set. It is deterministic (insertion order is preserved on
// every read), so it behaves identically under qemu emulation and wasm.
type MemoryStore struct {
	roles  []*Role
	joins  map[string]map[int64]bool
	nextID int64
}

// NewMemoryStore returns an empty in-memory [Store].
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{joins: map[string]map[int64]bool{}}
}

// FindRole implements [Store].
func (m *MemoryStore) FindRole(name, resourceType, resourceID string) (*Role, bool) {
	for _, r := range m.roles {
		if r.Name == name && r.ResourceType == resourceType && r.ResourceID == resourceID {
			return r, true
		}
	}
	return nil, false
}

// CreateRole implements [Store].
func (m *MemoryStore) CreateRole(name, resourceType, resourceID string) *Role {
	m.nextID++
	r := &Role{ID: m.nextID, Name: name, ResourceType: resourceType, ResourceID: resourceID}
	m.roles = append(m.roles, r)
	return r
}

// AssignRole implements [Store].
func (m *MemoryStore) AssignRole(userID string, role *Role) {
	set := m.joins[userID]
	if set == nil {
		set = map[int64]bool{}
		m.joins[userID] = set
	}
	set[role.ID] = true
}

// RemoveRole implements [Store].
func (m *MemoryStore) RemoveRole(userID string, role *Role) bool {
	set := m.joins[userID]
	if set == nil || !set[role.ID] {
		return false
	}
	delete(set, role.ID)
	return true
}

// RolesFor implements [Store].
func (m *MemoryStore) RolesFor(userID string) []*Role {
	set := m.joins[userID]
	if set == nil {
		return nil
	}
	var out []*Role
	for _, r := range m.roles {
		if set[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// AllRoles implements [Store].
func (m *MemoryStore) AllRoles() []*Role {
	out := make([]*Role, len(m.roles))
	copy(out, m.roles)
	return out
}
