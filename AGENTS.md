# AGENTS.md — go-rpc

Guidance for agentic coding assistants working in this repository.

## Project Overview

`go-rpc` is a single-package Go library (`package rpc`) that implements a JSON-RPC 2.0 server
served over HTTP. The module path is `github.com/bubunyo/go-rpc`. All library source lives at
the module root; `example/` contains a standalone runnable demo (`package main`).

- **Go version**: 1.19 (no generics, no `any`-constraint features beyond the `any` type alias)
- **Only external dependency**: `github.com/stretchr/testify` (test-only)
- **No Makefile**, no task runner — use plain `go` commands

---

## Build, Lint, and Test Commands

### Build / Vet
```bash
go build ./...          # compile all packages
go vet ./...            # static analysis — run before committing
```

### Run All Tests
```bash
go test ./...
```

### Run a Single Test Function
```bash
go test -run TestRpcServer_MethodNotFound .
go test -run TestRpcServer_ValidRequestParams/nil_params .    # single sub-test
```

### Run Tests with Verbose Output
```bash
go test -v -run TestRpcServer ./...
```

### Clear Test Cache
```bash
go clean -testcache
```

### Linting (CI uses golangci-lint v1.45.2)
```bash
golangci-lint run                                       # default linters
golangci-lint run --no-config --disable-all --enable goconst   # goconst only
```

There is no local `.golangci.yml`; CI uses the default linter set with `only-new-issues: true`.
If golangci-lint is not installed locally, `go vet ./...` is the minimum required check.

---

## Repository Layout

```
go-rpc/
├── rpc.go               # Core library
├── error.go             # Error type and sentinel errors
├── error_test.go        # White-box tests (package rpc)
├── rpc_test.go          # Black-box tests (package rpc_test)
├── test_rpc_service.go  # Placeholder — package declaration only
├── example/
│   └── example.go       # Standalone usage demo (package main)
├── .github/
│   └── workflows/
│       ├── test-unit.yml       # CI: go test ./...
│       └── golangci-lint.yml   # CI: golangci-lint
├── go.mod
├── go.sum
└── README.MD
```

---

## Code Style Guidelines

### Formatting
- All code must be `gofmt`-formatted. No exceptions.
- Line length is not formally enforced; follow idiomatic Go (keep lines readable).
- Use tabs for indentation (enforced by `gofmt`).

### Imports
- Group imports into **two blocks** separated by a blank line: stdlib first, then third-party.
  ```go
  import (
      "context"
      "encoding/json"
      "net/http"

      "github.com/stretchr/testify/assert"
  )
  ```
- All files must follow the stdlib-first convention. Fix any violations you encounter.
- Use `goimports` or `gofmt` to auto-organise imports.

### Naming Conventions
- Exported identifiers: `PascalCase` — `Service`, `Request`, `Response`, `NewServer`, `Opts`
- Unexported identifiers: `camelCase` — `methodResp`, `methodMap`
- Constants: `PascalCase` — `Version`, `MaxBytesRead`, `ExecutionTimeout`
- Sentinel errors: `PascalCase` noun phrases — `ParseError`, `MethodNotFound`, `InvalidRequest`
- JSON struct tags: `lowercase_snake_case` — `json:"jsonrpc"`, `json:"id,omitempty"`
- Acronyms follow Go convention: `RPC`, `JSON`, `HTTP` (all caps in exported names)

### Types and Declarations
- Prefer `any` over `interface{}` (the project already uses `any` throughout — Go 1.18+ alias).
- Group related type declarations in a `type ( ... )` block.
- Use type aliases (`type Foo = Bar`) to expose clean API surface without re-exporting internals.
- Define interfaces small and focused (single-method interfaces preferred).
- Avoid generics; this codebase targets Go 1.19 and does not use type parameters.

### Error Handling
- Define custom errors as value types that implement the `error` interface:
  ```go
  type Error struct {
      Code    int
      Message string
  }
  func (e Error) Error() string { return fmt.Sprintf("%d: %s", e.Code, e.Message) }
  ```
- Declare sentinel errors in a `var ( ... )` block at package level using a constructor:
  ```go
  var (
      ParseError   = NewError(-32700, "Parse error")
      MethodNotFound = NewError(-32601, "Method not found")
  )
  ```
- Wrap errors with context using `fmt.Errorf("%w: %s", SentinelErr, detail)`.
- Type-assert on the library's `Error` type with `switch err.(type)` to distinguish internal
  errors from generic `error` values.
- Only discard errors (with `_ =`) when the failure is genuinely inconsequential (e.g., writing
  to a `ResponseRecorder` in tests, encoding to an in-memory buffer). Document why when not
  obvious.

### Concurrency
- Use `sync.WaitGroup` for fan-out over a bounded set of goroutines (batch requests).
- Use a goroutine + channel + `time.NewTimer` + `select` for per-operation timeouts rather than
  `context.WithTimeout`, matching the existing pattern in `rpc.go`.
- When adding concurrency, ensure goroutines cannot leak; always drain or close channels.

### HTTP / JSON
- Use `httptest.NewRecorder()` for in-process HTTP handler tests — do not start a real server.
- Disable HTML escaping on JSON output: use `json.NewEncoder` + `SetEscapeHTML(false)`.
- Decode request bodies with a size limit via `http.MaxBytesReader` (already enforced by
  `MaxBytesRead` constant, currently 1 MB).

### Comments and Documentation
- All exported symbols must have a doc comment beginning with the symbol name.
- Inline comments on struct fields are preferred over separate paragraphs for short descriptions.
- Cite the JSON-RPC 2.0 spec URL when documenting spec-defined behaviour.

---

## Testing Guidelines

- **Same-package (white-box) tests**: file suffix `_test.go`, `package rpc` — used for internal
  types and unexported behaviour (see `error_test.go`).
- **External (black-box) tests**: `package rpc_test` — used for public API validation
  (see `rpc_test.go`). Prefer this for new tests of exported functionality.
- Use `github.com/stretchr/testify/assert` for non-fatal assertions and
  `github.com/stretchr/testify/require` for fatal assertions (stop test immediately on failure).
- Use **table-driven tests** (`for _, tc := range cases`) for any test that checks multiple
  input/output combinations.
- Define test helper types (e.g., `EchoService`) inline in the test file, not in production code.
- Use `t.Helper()` in test helper functions so failures point to the call site.
- Test helper functions follow the signature `func helperName(t *testing.T, ...)`.

---

## CI Checks (Must Pass Before Merging)

| Workflow | Trigger | Command |
|---|---|---|
| `test-unit.yml` | push to `master`/`main`, all PRs | `go test ./...` |
| `golangci-lint.yml` | push to `master`/`main`, version tags, all PRs | `golangci-lint run` |

Both workflows run on **Go 1.19**. Ensure local Go version matches to avoid surprises.

---

## Notes for Agents

- The main source file is `rpc.go`.
- `test_rpc_service.go` is intentionally nearly empty — it is a placeholder. Add shared test
  helpers there if needed rather than creating new files.
- There is no `Makefile`; do not create one unless explicitly requested.
- There are no Cursor rules (`.cursorrules` / `.cursor/rules/`) or Copilot instructions
  (`.github/copilot-instructions.md`) in this repository.
- The `example/` package must always compile: run `go build ./example/` after API changes.
