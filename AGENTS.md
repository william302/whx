# Repository Guidelines

## Project Structure & Module Organization
- `cmd/generate/` contains the Go entrypoint, tests, and embedded assets (`map.xlsx`). Treat this folder as the primary module.
- `examples/<case>/` holds fixture pairs (`input.xlsx`, `output.xlsx`, optional `result.xlsx`). Adding a new directory here automatically extends the regression suite.
- `.gocache/` is used to store the Go build cache locally; keep it out of version control.

## Build, Test, and Development Commands
- `GOCACHE=$(pwd)/.gocache go run ./cmd/generate <path/to/input.xlsx>` generates `result.xlsx` next to the provided workbook.
- `GOCACHE=$(pwd)/.gocache go test ./cmd/generate` or `./...` runs unit tests across all packages using the local cache (avoids permission issues).
- `gofmt -w cmd/generate/*.go` keeps Go sources formatted; run this before committing.

## Coding Style & Naming Conventions
- Go 1.24+; use standard Go formatting (`gofmt`), camelCase for locals, PascalCase for exported identifiers.
- Keep modules ASCII-only unless the file already contains Unicode for data (e.g., fixture directories).
- Place helper functions near their usage; prefer small, focused functions with explicit error handling.

## Testing Guidelines
- Tests live beside code (`cmd/generate/main_test.go`) and rely on fixtures under `examples/`.
- Add a new fixture by creating `examples/<name>/input.xlsx` and `output.xlsx`; the discovery helper will include it automatically.
- When debugging, you may keep the generated `result.xlsx` for inspection—tests no longer delete it.

## Commit & Pull Request Guidelines
- Write concise commit messages in the form `component: summary` (e.g., `generate: embed mapping file`).
- Include context in PR descriptions: what changed, why, manual test results (`go test`, `go run`), and any impacts on fixtures.
- Link issues or tickets where applicable and attach sample outputs or screenshots when behavior changes.

## Agent-Specific Notes
- Avoid editing fixture `.xlsx` files unless updating authoritative data; regenerate outputs via `go run` and inspect diffs carefully.
- Respect `.gitignore`: do not commit local caches or generated `result.xlsx` files. Use `git status -sb` before pushing to ensure a clean tree.
