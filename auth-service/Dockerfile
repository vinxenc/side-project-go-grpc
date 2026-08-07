# syntax=docker/dockerfile:1

# ---- build stage -----------------------------------------------------------
# Pinned to the go.mod toolchain line (go 1.26.4). GOTOOLCHAIN=auto lets Go
# fetch the exact patch if the base image lags.
FROM golang:1.26-alpine AS build
ENV GOTOOLCHAIN=auto CGO_ENABLED=0 GOOS=linux
WORKDIR /app

# Cache module downloads separately from the source for faster rebuilds.
COPY go.mod go.sum ./
RUN go mod download

# Build the server (main package lives at ./src).
COPY core ./core
COPY setting ./setting
COPY src ./src
RUN go build -trimpath -ldflags="-s -w" -o /out/auth-service ./src

# ---- runtime stage ---------------------------------------------------------
# Distroless static + nonroot: no shell, minimal attack surface. The binary is
# statically linked (CGO disabled), so the static base is sufficient.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/auth-service /auth-service
EXPOSE 8080
ENTRYPOINT ["/auth-service"]
