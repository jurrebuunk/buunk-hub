# buunk-hub: the frontend built with node, embedded in the Go binary, which
# runs alone in distroless as non-root. Build from the repo root:
#   docker build -t buunk-hub .
# CI (.github/workflows/image.yml) publishes it as
# ghcr.io/jurrebuunk/buunk-hub:$(cat VERSION).

FROM node:22.23.5-alpine@sha256:1314fb30ec37592d5845e5a160d522c0e680972aa534549940b8d73bde88481a AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run typecheck && npm run build

FROM golang:1.26.10-alpine@sha256:87c65a688975c0b1d4585d204874c45ca78354670e3a5f11368c5a492303d696 AS go
ENV CGO_ENABLED=0 GOTOOLCHAIN=local
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/dist web/dist
RUN go vet ./... && \
    go test ./... && \
    go build -trimpath -ldflags "-s -w -X main.version=$(cat VERSION)" -o /hub .

FROM gcr.io/distroless/static-debian13:nonroot@sha256:8ce6dd87e9823c5b48dcfb2c8604714ccb351ce59607327bcc4957448da279b5
LABEL org.opencontainers.image.source="https://github.com/jurrebuunk/buunk-hub" \
      org.opencontainers.image.description="buunk-hub, the people hub for buunk.org"
COPY --from=go /hub /hub
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/hub"]
