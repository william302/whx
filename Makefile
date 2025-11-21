GO          ?= go
GOCACHE_DIR ?= $(PWD)/.gocache

.PHONY: test run fmt serve

test:
	GOCACHE=$(GOCACHE_DIR) $(GO) test ./...

run:
ifndef INPUT
	$(error Usage: make run INPUT=examples/<case>/input.xlsx)
endif
	GOCACHE=$(GOCACHE_DIR) $(GO) run ./cmd/generate $(INPUT)

fmt:
	gofmt -w cmd/generate/*.go

serve:
	GOCACHE=$(GOCACHE_DIR) $(GO) run ./cmd/generate --serve --addr :8001
