# syntax=docker/dockerfile:1

# --- Stage 1: generate the static SPA -----------------------------------------
FROM node:22-slim AS ui
WORKDIR /ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci
COPY ui/ ./
RUN npm run generate    # -> /ui/.output/public

# --- Stage 2: build the Go binary with the SPA embedded -----------------------
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Replace the committed placeholder with the real bundle, then build with embed.
RUN rm -rf internal/web/dist && mkdir -p internal/web/dist
COPY --from=ui /ui/.output/public/ internal/web/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /twinmodel ./cmd/twinmodel

# --- Stage 3: distroless runtime ----------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /twinmodel /twinmodel
EXPOSE 8080
ENTRYPOINT ["/twinmodel", "serve"]
