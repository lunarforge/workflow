# Development Preferences & Standards

**Last Updated:** 2026-02-10
**Purpose:** Canonical reference for all agent roles. Agents MUST follow these standards when working on projects.

---

## Tech Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Backend | Go (1.24+) | `internal/` package layout |
| API | gRPC + connect-go | gRPC for service-to-service, connect-go for REST/HTTP compatibility |
| Frontend | SvelteKit + TypeScript | Svelte 5 with runes, SvelteKit for routing/SSR |
| Styling | Tailwind CSS | Utility-first, extend via `tailwind.config.js` |
| State | Svelte stores | Writable/readable stores, context API for component trees |
| Database | PostgreSQL (primary) | SQLite for local/embedded use cases |
| Package manager | bun | Use `bun` instead of npm/yarn for all JS/TS projects |
| Testing | Go: stdlib + testify | JS: vitest. E2E: Playwright |
| Logging | Go: zerolog (rs/zerolog) | JS: console.log with prefixes |
| Git | Conventional commits | `feat:`, `fix:`, `test:`, `docs:`, `refactor:` |

---

## Core Principles

1. **Developer Experience First** — Prioritize tools and practices that make debugging faster
2. **Verbose in Development** — Log everything in dev mode, minimal in production
3. **Fail Fast & Loud** — Make errors obvious during development
4. **Reproducible Issues** — Every bug should be easy to reproduce with good logging
5. **Explore Before Acting** — Always read existing code before modifying. Understand the codebase before suggesting changes.

---

## Go Backend Standards

### Project Structure

```
project/
├── cmd/                    # CLI entry points
│   └── service-name/
│       └── main.go
├── internal/               # Private packages
│   ├── api/               # gRPC/connect handlers
│   ├── domain/            # Business logic, interfaces
│   ├── storage/           # Database implementations
│   └── config/            # Configuration
├── proto/                  # Protobuf definitions
├── migrations/             # SQL migrations
├── Makefile
└── go.mod
```

### Structured Logging (zerolog)

Every log entry MUST include contextual fields:

```go
log.Info().
    Str("request_id", requestID).
    Str("tenant_id", tenantID).
    Str("user_id", userID).
    Msg("operation description")
```

**Log Levels:**

| Level | When | Example |
|-------|------|---------|
| DEBUG | Detailed diagnostic info (dev only) | "evaluating rule", "parsing field" |
| INFO | Normal operations | "match created", "transaction ingested" |
| WARN | Potential issues | "slow query detected", "high memory usage" |
| ERROR | Operation failures | "failed to create match", "database connection lost" |

**Bad:**
```go
log.Error().Err(err).Msg("error")
```

**Good:**
```go
log.Error().
    Err(err).
    Str("tenant_id", tenantID).
    Str("transaction_id", txnID).
    Interface("candidates", candidates).
    Msg("failed to create match")
```

### Error Handling

Always wrap errors with context using `fmt.Errorf`:

```go
if err != nil {
    return fmt.Errorf("create user %s: %w", userID, err)
}
```

Rules:
- Wrap at every layer with operation context
- Use `%w` for wrappable errors
- Never discard errors silently
- Log at the boundary where you handle (not every layer)

### gRPC + connect-go

```go
// Interceptor logs every request
log.Info().
    Str("method", info.FullMethod).
    Str("request_id", requestID).
    Interface("payload", req).
    Msg("gRPC request")

log.Info().
    Str("method", info.FullMethod).
    Dur("duration", elapsed).
    Interface("result", resp).
    Msg("gRPC response")
```

### Database

- PostgreSQL for services, SQLite for embedded/local use
- Log all queries in dev mode with timing
- Detect slow queries (>100ms threshold)

```go
if duration > 100*time.Millisecond {
    log.Warn().
        Dur("duration", duration).
        Str("query", query).
        Msg("slow query")
}
```

### Testing (Go)

- **Framework:** `testing` + `testify/assert` + `testify/require`
- **Coverage:** 85% minimum for new code
- **Table-driven tests** for multiple input/output combinations
- **Test naming:** `TestFunctionName_Scenario_ExpectedBehavior`

```go
func TestCreateUser_DuplicateEmail_ReturnsConflict(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

### Correlation IDs

Every request gets:
- **Request ID** (generated or from header)
- **Tenant ID** (from auth context)
- **User ID** (from auth context)

Propagate through all service calls, database queries, event messages, and log entries.

### Performance Logging

```go
start := time.Now()
defer func() {
    elapsed := time.Since(start)
    log.Info().
        Str("operation", opName).
        Dur("duration", elapsed).
        Msg("operation completed")
    if elapsed > time.Second {
        log.Warn().
            Str("operation", opName).
            Dur("duration", elapsed).
            Msg("slow operation detected")
    }
}()
```

---

## Frontend Standards (SvelteKit + TypeScript)

### Console Logging (Always ON in Dev Mode)

```typescript
// API calls
console.log('[API]', method, endpoint, payload);
console.log('[API Response]', response);

// Errors
console.error('[ERROR]', component, error, {
    context: additionalInfo,
    timestamp: new Date().toISOString()
});

// Warnings
console.warn('[WARNING]', message, data);

// Debug info
console.debug('[DEBUG]', detail, state);

// Performance
console.time('operation-name');
// ... code
console.timeEnd('operation-name');
```

### Error Handling

```typescript
try {
    const result = await api.call();
    console.log('[API Success]', endpoint, result);
    return result;
} catch (error) {
    console.error('[API Error]', endpoint, error);
    throw error;
}
```

**Error Display:**
- User sees: Friendly error message
- Console shows: Full stack trace + context
- Dev mode: Show error details on screen
- Production: Send to monitoring service

### Error Context Type

```typescript
interface ErrorContext {
    message: string;           // User-friendly message
    code: string;              // Error code (e.g., "ERR_API_FAILED")
    timestamp: string;         // ISO timestamp
    requestId?: string;        // For correlation with backend
    component?: string;        // Where error occurred
    stackTrace?: string;       // Dev only
    additionalInfo?: unknown;  // Extra context
}
```

### Styling (Tailwind CSS)

- Use Tailwind utility classes; avoid inline `style` attributes
- Extract repeated patterns into component-level classes via `@apply` only when truly repetitive
- Use Tailwind config for project-wide theme tokens (colors, spacing, fonts)

### State Management (Svelte Stores)

- Use `writable`/`readable` stores for shared UI state
- Use Svelte context API (`setContext`/`getContext`) for component tree scoping
- Derive computed values with `derived` stores
- Keep stores in `src/lib/stores/`

### Testing (Frontend)

- **Unit/Integration:** Vitest with `@testing-library/svelte`
- **E2E:** Playwright
- **Coverage:** 85% minimum for new code

### Project Layout

```
src/
├── lib/
│   ├── components/        # Reusable components
│   ├── stores/            # Svelte stores
│   ├── api/              # API client functions
│   └── utils/            # Shared utilities
├── routes/                # SvelteKit pages
└── app.html
```

---

## Git Workflow

### Conventional Commits

```
feat: add user registration endpoint
fix: resolve race condition in transaction matching
test: add E2E tests for auth flow
docs: update API documentation
refactor: extract matching logic into service layer
```

Rules:
- Lowercase prefix, colon, space, then imperative description
- Body optional; use for "why" not "what"
- Reference issue IDs when applicable

### Branch Strategy

- Feature branches from `main`
- Push commits to remote before marking task complete
- Squash or rebase before merge (keep history clean)

---

## Quick Checklist for New Features

When implementing a new feature:

- [ ] Read existing code in the area before making changes
- [ ] Add structured logging with zerolog for backend operations
- [ ] Add console.log with prefixes for frontend API calls
- [ ] Include request/correlation IDs in all logs
- [ ] Log state transitions (before + after)
- [ ] Add performance timing for potentially slow operations
- [ ] Create error boundaries for frontend UI components
- [ ] Wrap errors with context at every layer (Go)
- [ ] Write tests (85% coverage target)
- [ ] Use conventional commit format
- [ ] Test error scenarios and verify log output
