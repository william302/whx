# syntax=docker/dockerfile:1

FROM golang:1.24 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN GOCACHE=/go/.gocache go build -o /out/whx ./cmd/generate

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/whx /usr/local/bin/whx
EXPOSE 8001

ENTRYPOINT ["/usr/local/bin/whx"]
CMD ["--serve", "--addr", ":8001"]
