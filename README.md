# WHX – Warehouse Excel Mapper

WHX converts order spreadsheets into the warehouse-ready format used by this project. It reads an `input.xlsx`, maps SKUs via an embedded `map.xlsx`, and writes a new workbook named `Warehouse_<input>.xlsx` next to the source file. Styling is ignored; only data layout matters. Version: **0.3.0** (full web preview modal, clearer UI, Docker ready).

## Quick Start

```bash
# Build once (requires Go 1.24+)
GOCACHE=$(pwd)/.gocache go build -o whx ./cmd/generate

# Show version
./whx --version

# Start the web UI (serves on :8001)
./whx --serve --addr :8001

# Run against a file
./whx /path/to/input.xlsx
```

The output will appear in the same directory as the input, e.g. `examples/1103/Warehouse_input.xlsx`.

## Repository Layout
- `cmd/generate/` – Go entrypoint, tests, embedded mapping workbook.
- `examples/<case>/` – Fixture pairs (`input.xlsx`, `output.xlsx`) plus generated `Warehouse_*.xlsx` for inspection.
- `.gocache/` – Local Go build cache directory (ignored by git).

## Development Workflow

| Command | Description |
| --- | --- |
| `make run INPUT=examples/1103/input.xlsx` | Executes WHX using the embedded map and writes the warehouse file in place. |
| `make test` | Runs `go test ./...` with GOCACHE set to `.gocache`, validating all fixtures automatically. |
| `make fmt` | Applies `gofmt -w` to Go sources. |
| `make serve` | Runs the web UI locally on `:8001`. |

To add a new regression case, create `examples/<name>/input.xlsx` and `output.xlsx`. The test suite discovers these automatically.

## Docker

Build and run the web interface in a container:

```bash
docker build -t whx:0.3.0 .
docker run --rm -p 8001:8001 whx:0.3.0
```

Then open `http://localhost:8001` to upload and convert files.

## Notes
- The SKU mapping workbook is embedded, so distributing the compiled `whx` binary requires no extra files.
- Tests intentionally leave the generated `Warehouse_*.xlsx` in `examples/` to ease manual comparison.
- When sharing WHX with teammates, ensure they `chmod +x whx` and optionally move it into `~/bin` before running `whx /path/to/input.xlsx`.
