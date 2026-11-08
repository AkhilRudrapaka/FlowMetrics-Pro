# syntax=docker/dockerfile:1

# ---- build: static binary, dependencies cached in their own layer ----
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- runtime: distroless = no shell, no package manager, runs as non-root ----
FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/server /server
EXPOSE 8080
# `server healthcheck` probes /healthz from inside the container (there is no curl to do it).
HEALTHCHECK --interval=10s --timeout=4s --start-period=10s --retries=5 CMD ["/server", "healthcheck"]
ENTRYPOINT ["/server"]
