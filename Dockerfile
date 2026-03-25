FROM golang:1.26-bookworm AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /ollama-review-bot .

# Git provider shells out to git; HTTPS to Ollama/Gitea needs CA certs.
FROM debian:bookworm-slim

RUN apt-get update \
	&& apt-get install -y --no-install-recommends ca-certificates git \
	&& rm -rf /var/lib/apt/lists/*

COPY --from=builder /ollama-review-bot /usr/local/bin/ollama-review-bot

ENTRYPOINT ["/usr/local/bin/ollama-review-bot"]
