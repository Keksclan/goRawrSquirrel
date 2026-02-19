# Versioning & Release Strategy

This document describes the versioning policy, release strategy, and API
stability guarantees for **goRawrSquirrel**.

---

## 1. Current Version Target

The current development target is **v0.1.0**.

This release represents the first publicly usable version of the toolkit and
includes the foundational middleware primitives (recovery, request-ID, rate
limiting, authentication, IP blocking, caching) along with the `NewServer`
functional-options API.

> **Note:** While the project is below v1.0.0, breaking changes may occur
> between minor versions. Pin your dependency to an exact minor version if
> stability is important to you.

---

## 2. What Qualifies for v1.0.0

The project will be promoted to **v1.0.0** once **all** of the following
criteria are met:

- The public API surface (`NewServer`, all `With*` options, exported types and
  interfaces) has been stable across at least **two consecutive minor
  releases** without breaking changes.
- OpenTelemetry tracing integration is complete and tested.
- Retry / back-off helpers and circuit-breaker middleware are implemented.
- Comprehensive documentation and usage examples cover every public feature.
- CI pipeline includes linting, unit tests, integration tests, and race
  detection on all supported Go versions.
- At least one production deployment has validated the middleware stack under
  realistic load.

---

## 3. Semantic Versioning Rules

goRawrSquirrel follows [Semantic Versioning 2.0.0](https://semver.org/):

```
vMAJOR.MINOR.PATCH
```

| Component | Incremented when…                                                        |
|-----------|--------------------------------------------------------------------------|
| **MAJOR** | A backwards-incompatible change is made to the public API.               |
| **MINOR** | New functionality is added in a backwards-compatible manner.             |
| **PATCH** | Backwards-compatible bug fixes, documentation updates, or performance improvements are released. |

### Pre-v1 Addendum

While the major version is **0** (i.e., `v0.x.y`):

- **MINOR** bumps may include breaking changes.
- **PATCH** bumps remain backwards-compatible bug fixes only.

---

## 4. Deprecation Policy

When a public symbol, option, or behaviour is scheduled for removal:

1. **Announce** — The upcoming deprecation is noted in the CHANGELOG and the
   symbol's GoDoc comment is updated with a `Deprecated:` annotation that
   names the replacement (if any).
2. **Grace period** — The deprecated symbol remains functional for **at least
   one minor release cycle** (two cycles after v1.0.0).
3. **Remove** — The symbol is removed in the next eligible MAJOR (post-v1) or
   MINOR (pre-v1) release after the grace period expires.

Example deprecation comment:

```go
// WithOldOption configures X.
//
// Deprecated: Use [WithNewOption] instead. Will be removed in v0.3.0.
func WithOldOption() ServerOption { … }
```

---

## 5. Public API Stability Rules

The **public API** consists of every exported identifier in non-`internal`
packages:

- `NewServer` and all `ServerOption` constructors (`With*` functions).
- Exported types, interfaces, and constants in `auth`, `cache`, `contextx`,
  `interceptors`, `policy`, `ratelimit`, and `security`.
- The `examples/` directory serves as documentation only and is **not**
  covered by stability guarantees.

### Stability Guarantees (post-v1)

| Change kind                        | Allowed in PATCH | Allowed in MINOR | Allowed in MAJOR |
|------------------------------------|:----------------:|:----------------:|:----------------:|
| Bug fix (no API change)            | ✅               | ✅               | ✅               |
| New exported symbol                | ❌               | ✅               | ✅               |
| New field in an options struct     | ❌               | ✅               | ✅               |
| Change function signature          | ❌               | ❌               | ✅               |
| Remove exported symbol             | ❌               | ❌               | ✅               |
| Change middleware default priority | ❌               | ❌               | ✅               |

### Pre-v1

All of the above changes are permitted in MINOR bumps. PATCH releases still
guarantee backwards compatibility.

---

## 6. Internal Packages Disclaimer

Everything under the `internal/` directory tree (e.g., `internal/core`) is
**private implementation detail**. These packages:

- Are **not** part of the public API.
- May change or be removed **at any time**, in any release, without prior
  notice or deprecation.
- **Must not** be imported by external modules. The Go toolchain enforces this
  restriction at build time.

If you find yourself needing functionality that currently lives in `internal/`,
please open an issue to discuss promoting it to the public surface.
