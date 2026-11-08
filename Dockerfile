# syntax=docker/dockerfile:1
#
# Single-image build used by the Render deployment (see render.yaml and docs/deploy-render.md):
# the Go API ALSO serves the built web UI, so the browser sees one origin. That is what lets the
# SameSite=Strict refresh cookie work with no CORS configuration.
#
# Local development does not use this file: `docker compose up` builds backend/ and frontend/
# separately and puts nginx in front.

# ---- 1. build the web UI ----
FROM node:24-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Shows the demo accounts on the login page. This image is for a public demo with seeded data;
# change to false if you turn seeding off.
ARG VITE_DEMO_HINT=true
ENV VITE_DEMO_HINT=$VITE_DEMO_HINT
RUN npm run build

# ---- 2. build the API (static binary) ----
FROM golang:1.27-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- 3. runtime: distroless (no shell, non-root, includes the CA certificates needed to reach Atlas over TLS) ----
FROM gcr.io/distroless/static:nonroot
COPY --from=api /out/server /server
COPY --from=web /web/dist /public
ENV STATIC_DIR=/public
# Render injects PORT (10000 by default); the server binds 0.0.0.0:$PORT.
ENV PORT=10000
EXPOSE 10000
HEALTHCHECK --interval=30s --timeout=4s --start-period=20s --retries=3 CMD ["/server", "healthcheck"]
ENTRYPOINT ["/server"]
