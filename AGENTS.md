# TimeView repository guide

## Project shape

- `main.go` wires the embedded React application to the Go HTTP server.
- `internal/timer` owns timer state, validation, and configuration persistence.
- `internal/httpserver` owns HTTP routing, middleware, request handling, mutations, and SSE delivery.
- `web/src` contains the React/TypeScript control, display, and settings UI.
- `docs/SPECIFICATION.md` is the behavioral source of truth. Update it when user-visible behavior or API contracts change.

## Code organization

- Split files by responsibility so each file has one clear purpose; move unrelated concerns into focused files.
- Group related files into folders that make the repository easy to scan, especially as a feature grows.

## Backend rules

- Keep timer state in memory. Persist preferences only; never persist running state, remaining time, blackout release, or the current message.
- Preserve safe startup: idle timer with blackout enabled.
- Guard shared server state with `Server.mu`. Apply mutations to a model copy, validate and persist it, then commit it to the live model.
- Keep domain state and configuration in `internal/timer`. Keep HTTP responsibilities in `internal/httpserver`: routing/lifecycle in `server.go`, middleware and JSON helpers in `middleware.go`, shared request handling in `handlers.go`, endpoint mutations in `mutations.go`, and SSE in `events.go`.
- Preserve ETag checks, idempotency behavior, request-size limits, rate limits, browser-only mutation checks, and SSE connection limits when changing handlers.
- Keep API and UI error messages in Japanese.

## Frontend rules

- Keep timer calculations as pure functions in `web/src/timer/timer.ts` and cover boundary behavior in `web/src/timer/timer.test.ts`.
- Keep the stage display free of operational state labels; it should show the timer/message, warning colors, progress gauge, and connection warning only.
- Maintain accessible labels and progressbar values when changing display controls.

## Validation

Run the narrowest relevant checks while developing, then run the full checks before committing:

```sh
go test ./internal/...
npm --prefix web test
task lint
task test
```

Maintain at least 80% statement coverage for the backend packages:

```sh
go test -cover ./internal/...
```

Do not commit generated output from `dist/`, `web/dist/`, local configuration, logs, coverage profiles, or dependency directories.

## Commits

- Unless the user asks otherwise, create a commit after each logical change is complete and its relevant checks pass.
- Keep unrelated fixes in separate commits. Include directly related tests and documentation in the same commit as the implementation.
- Before committing, inspect the diff and stage only the files belonging to that change. Preserve unrelated user changes.
- Use concise Conventional Commit messages such as `feat: ...`, `fix: ...`, `refactor: ...`, `test: ...`, or `docs: ...`.
- Do not amend, squash, rewrite, or push commits unless the user explicitly requests it.
