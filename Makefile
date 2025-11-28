GO          ?= go
GOCACHE_DIR ?= $(PWD)/.gocache

.PHONY: test run fmt serve push

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

push:
ifndef VERSION
	$(error Usage: make push VERSION=0.4.0)
endif
	docker build --platform linux/amd64 -t whx:$(VERSION) .
	docker tag whx:$(VERSION) swr.cn-north-4.myhuaweicloud.com/yogeeai/whx:$(VERSION)
	docker push swr.cn-north-4.myhuaweicloud.com/yogeeai/whx:$(VERSION)
