<p align="center"><img src="https://go-ruby-rolify.github.io/logo.png" alt="go-ruby-rolify/rolify" width="720"></p>

# rolify — go-ruby-rolify

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-rolify.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the deterministic core of Ruby's
[`rolify`](https://github.com/RolifyCommunity/rolify) gem** — role management with
global, class-scoped, and instance-scoped roles. It reproduces the role model, the
user-side API (`add_role` / `has_role?` / `remove_role` / `roles` /
`has_all_roles?` / `has_any_role?`), the resource-side helpers (`applied_roles` /
`roles_to_administrate`), the faithful scope-matching semantics, the `:any`
wildcard, and the `strict_rolify` / `role_cache` options — **without any Ruby
runtime**.

It is the role engine for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a **standalone,
reusable** module.

> **What it is — and isn't.** Everything `rolify` does *above* the database is
> deterministic and needs **no interpreter**, so it lives here as pure Go: creating
> roles, joining them to users, and — crucially — the scope-matching rules that
> decide whether a stored role satisfies a query. **Persistence itself is a host
> seam**: the [`Store`](store.go) interface (`FindRole` / `CreateRole` /
> `AssignRole` / `RemoveRole` / `RolesFor` / `AllRoles`) is the only piece that
> touches storage. The default in-repo `MemoryStore` is an allocation-only
> implementation the tests drive; a future rbgo binding wires the seam to
> **ActiveRecord**, mirroring the gem, whose only persistent concern is the roles
> table and the users↔roles join.

## Features

Faithful port of the `rolify` role engine:

- **Role model** — `Role{Name, ResourceType, ResourceID}`: global (both empty),
  class-scoped (type set, id empty), or instance-scoped (both set).
- **User API** — `AddRole` (idempotent), `HasRole`, `HasCachedRole`, `RemoveRole`,
  `Roles`, `HasAllRoles`, `HasAnyRole`.
- **Scopes** — `Global()`, `ClassScope(type)`, `InstanceScope(type, id)`, and the
  `Any()` wildcard (Ruby's `has_role? :admin, :any`).
- **Faithful matching** — a **global** role matches any query; a **class** role
  matches any instance of that class and the class itself; an **instance** role
  matches exactly; `:any` matches a role of any scope.
- **Resource side** — `AppliedRoles(type, id)` (instance + class + global roles on
  a resource) and `RolesToAdministrate(type, id)` (roles scoped to the resource or
  its class).
- **Options** — `Strict(true)` (`strict_rolify`: no scope inheritance for
  class/instance queries) and `Cache(true)` (`role_cache`: load a user's roles
  once, invalidate on mutation).
- **Persistence seam** — `Store`; the default `MemoryStore` opens no storage of its
  own, and a host binding wires it to ActiveRecord.

CGO-free, dependency-free (stdlib only), **100% test coverage**, `gofmt` +
`go vet` clean, and green across the six 64-bit Go targets (amd64, arm64,
riscv64, loong64, ppc64le, **s390x** — big-endian) plus `js/wasm` and
`wasip1/wasm`.

## Install

```sh
go get github.com/go-ruby-rolify/rolify
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-rolify/rolify"
)

func main() {
	rf := rolify.New(rolify.NewMemoryStore())
	user := rf.User("42")

	user.AddRole("admin", rolify.Global())                    // add_role :admin
	user.AddRole("moderator", rolify.ClassScope("Forum"))     // add_role :moderator, Forum
	user.AddRole("owner", rolify.InstanceScope("Forum", "7")) // add_role :owner, @forum

	fmt.Println(user.HasRole("admin", rolify.Global()))                      // true
	fmt.Println(user.HasRole("moderator", rolify.InstanceScope("Forum", "1"))) // true (class role)
	fmt.Println(user.HasRole("owner", rolify.Any()))                        // true
	fmt.Println(user.HasRole("owner", rolify.InstanceScope("Forum", "8")))    // false

	user.RemoveRole("admin", rolify.Global())

	// Resource side.
	rf.AppliedRoles("Forum", "7")        // instance + class + global roles on @forum
	rf.RolesToAdministrate("Forum", "7") // roles scoped to @forum or the Forum class
}
```

### Strict mode & caching

```go
rf := rolify.New(rolify.NewMemoryStore(), rolify.Strict(true), rolify.Cache(true))
user := rf.User("42")
user.AddRole("admin", rolify.Global())

// strict_rolify: a global role no longer satisfies a narrower query.
user.HasRole("admin", rolify.Global())               // true
user.HasRole("admin", rolify.ClassScope("Forum"))    // false (strict)
```

### Injecting a store (hosts)

```go
type ActiveRecordStore struct{ /* db handle */ }
// ... implement rolify.Store: FindRole / CreateRole / AssignRole / RemoveRole /
//     RolesFor / AllRoles over your users↔roles tables ...

rf := rolify.New(&ActiveRecordStore{ /* ... */ })
```

## Value model

| gem                                    | this package                                      |
| -------------------------------------- | ------------------------------------------------- |
| `user.add_role :admin`                 | `user.AddRole("admin", Global())`                 |
| `user.add_role :mod, Forum`            | `user.AddRole("mod", ClassScope("Forum"))`        |
| `user.add_role :mod, @forum`           | `user.AddRole("mod", InstanceScope("Forum", id))` |
| `user.has_role? :admin`                | `user.HasRole("admin", Global())`                 |
| `user.has_role? :admin, :any`          | `user.HasRole("admin", Any())`                    |
| `user.has_cached_role? :admin`         | `user.HasCachedRole("admin", Global())`           |
| `user.remove_role :admin`              | `user.RemoveRole("admin", Global())`              |
| `user.roles`                           | `user.Roles()`                                    |
| `user.has_all_roles?(…)`               | `user.HasAllRoles(…)`                             |
| `user.has_any_role?(…)`                | `user.HasAnyRole(…)`                              |
| `resourcify` + `@forum.applied_roles`  | `rf.AppliedRoles("Forum", id)`                    |
| `@forum.roles_to_administrate`         | `rf.RolesToAdministrate("Forum", id)`             |
| `strict_rolify` / `role_cache`         | `Strict(true)` / `Cache(true)`                    |
| the `roles` table + users↔roles join   | `Store` (host seam; `MemoryStore` in tests)       |

## Scope semantics

A role names a permission and carries an optional scope:

- **Global** (`ResourceType == "" && ResourceID == ""`) — matches **any** query.
- **Class** (`ResourceType` set, `ResourceID == ""`) — matches any **instance** of
  that class **and** the class itself.
- **Instance** (`ResourceType` and `ResourceID` set) — matches **exactly** that
  instance.

A query names a role and a `Scope` (`Global` / `ClassScope` / `InstanceScope` /
`Any`). `Any()` is the `:any` wildcard: it matches a role of any scope. Under
`Strict(true)` a class/instance query is matched **exactly** — a broader (global or
class) role no longer satisfies it.

## Tests & coverage

The suite is deterministic and storage-local: the in-repo `MemoryStore` backs every
test, so the cross-arch qemu lanes and the Windows lane all hold coverage at
**100%** — including global vs. class vs. instance matching, the `:any` wildcard,
idempotent duplicate adds, removing a nonexistent/unheld role, `has_all_roles?` /
`has_any_role?`, strict mode, and the role cache.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-rolify/rolify authors.
