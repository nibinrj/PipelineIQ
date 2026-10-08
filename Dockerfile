# syntax=docker/dockerfile:1
#
# Build tag: golang:1.27.1-bookworm
# Source: docker-library/official-images library/golang lists 1.27.1-bookworm
# and the shared tag 1.27.1. Matches the Go toolchain pinned in go.mod.
FROM golang:1.27.1-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags='-s -w' -o /out/pipelineiq-server ./cmd/pipelineiq-server

# Runtime tag: gcr.io/distroless/static-debian13:nonroot
# Source: GoogleContainerTools/distroless README, Debian 13 static image.
# A static Go binary does not need libc, so static (not base) is the right image.
# Distroless does not publish a semver tag. nonroot is the documented tag and
# already runs as uid 65532. A digest was not pinned: the registry was not
# reachable from the environment that wrote this file. See docs/study/P1-design.md.
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/pipelineiq-server /pipelineiq-server
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/pipelineiq-server"]
