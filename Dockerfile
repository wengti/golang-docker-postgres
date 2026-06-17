# ---- Build stage: compile the binary in a full Go toolchain image ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Download dependencies first, in their own layer, so they are cached and only
# re-fetched when go.mod/go.sum change (not on every source edit).
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and build a statically linked binary.
# CGO_ENABLED=0 produces a self-contained binary with no libc dependency, so it
# runs on a minimal base image. schema.sql is baked in via //go:embed.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/server .

# ---- Run stage: ship only the binary on a tiny base image ----
FROM alpine:3.20
WORKDIR /app

# CA certificates, so the app can make TLS connections later (e.g. to a managed
# database with sslmode=require/verify-full).
RUN apk add --no-cache ca-certificates

COPY --from=build /app/server .

EXPOSE 8080
ENTRYPOINT ["/app/server"]
