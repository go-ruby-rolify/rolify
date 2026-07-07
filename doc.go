// Copyright (c) the go-ruby-rolify/rolify authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package rolify is a pure-Go (CGO-free) reimplementation of the deterministic
// core of Ruby's [rolify] gem — role management with global, class-scoped, and
// instance-scoped roles. It reproduces the role model, the user-side API
// (add_role / has_role? / remove_role / roles / has_all_roles? / has_any_role?),
// the resource-side helpers (applied_roles / roles_to_administrate), the faithful
// scope-matching semantics, the :any wildcard, and the strict_rolify / role_cache
// options — without any Ruby runtime.
//
// It is the role engine for
// [go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a
// standalone, reusable module.
//
// # What it is — and isn't
//
// Everything rolify does above the database is deterministic and needs no
// interpreter, so it lives here as pure Go: creating roles, joining them to
// users, and — crucially — the scope-matching rules that decide whether a stored
// role satisfies a query. Persistence itself is a host seam: the [Store]
// interface (FindRole / CreateRole / AssignRole / RemoveRole / RolesFor /
// AllRoles) is the only piece that touches storage. The default in-repo store is
// [MemoryStore], an allocation-only implementation used by the tests; a future
// rbgo binding wires the seam to ActiveRecord, mirroring the gem, whose only
// persistent concern is the roles table and the users↔roles join.
//
// # Scope semantics
//
// A [Role] carries a Name and an optional scope expressed as a ResourceType and
// ResourceID:
//
//   - Global role: ResourceType == "" and ResourceID == "". Matches any query.
//   - Class role: ResourceType set, ResourceID == "". Matches any instance of
//     that class and the class itself.
//   - Instance role: ResourceType and ResourceID set. Matches exactly.
//
// A query names a role and a [Scope]: [Global], [ClassScope], [InstanceScope], or
// the [Any] wildcard (Ruby's has_role? :admin, :any) which matches a role of any
// scope. The strict_rolify option ([Strict]) disables scope inheritance so a
// global or class role no longer satisfies a narrower class/instance query.
//
// # Flow
//
//	rf := rolify.New(rolify.NewMemoryStore())
//	user := rf.User("42")
//
//	user.AddRole("admin", rolify.Global())                 // add_role :admin
//	user.AddRole("moderator", rolify.ClassScope("Forum"))  // add_role :moderator, Forum
//	user.AddRole("owner", rolify.InstanceScope("Forum", "7"))
//
//	user.HasRole("admin", rolify.Global())                    // true
//	user.HasRole("moderator", rolify.InstanceScope("Forum", "1")) // true (class role)
//	user.HasRole("owner", rolify.Any())                       // true
//	user.RemoveRole("admin", rolify.Global())
//
// # Value model
//
// A host (go-embedded-ruby / rbgo) maps its Ruby Role / user / resource objects
// to and from these shapes:
//
//	user.add_role :admin              -> User.AddRole("admin", Global())
//	user.add_role :mod, Forum         -> User.AddRole("mod", ClassScope("Forum"))
//	user.add_role :mod, @forum        -> User.AddRole("mod", InstanceScope("Forum", id))
//	user.has_role? :admin             -> User.HasRole("admin", Global())
//	user.has_role? :admin, :any       -> User.HasRole("admin", Any())
//	user.has_cached_role? :admin      -> User.HasCachedRole("admin", Global())
//	user.remove_role :admin           -> User.RemoveRole("admin", Global())
//	user.roles                        -> User.Roles()
//	user.has_all_roles?(...)          -> User.HasAllRoles(...)
//	user.has_any_role?(...)           -> User.HasAnyRole(...)
//	@forum.applied_roles              -> Rolify.AppliedRoles("Forum", id)
//	@forum.roles_to_administrate      -> Rolify.RolesToAdministrate("Forum", id)
//
// [rolify]: https://github.com/RolifyCommunity/rolify
package rolify
