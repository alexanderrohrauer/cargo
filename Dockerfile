# Stage 1: Build the Go binary
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go module files and download dependencies first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o cargo .

# Stage 2: Runtime image
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies:
# - git: for repository operations
# - openssh-client: for SSH-based git authentication
# - age: for age keypair generation (used by SOPS)
# - docker-cli + docker-compose-plugin: for running compose commands
RUN apk add --no-cache \
    git \
    openssh-client \
    age \
    docker-cli \
    docker-cli-compose \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Install sops from GitHub releases (supports amd64 and arm64)
ARG SOPS_VERSION=3.12.1
ARG TARGETARCH=amd64
RUN SOPS_ARCH="${TARGETARCH}" && \
    [ "${TARGETARCH}" = "arm64" ] && SOPS_ARCH="arm64" || true && \
    wget -qO /usr/local/bin/sops \
        "https://github.com/getsops/sops/releases/download/v${SOPS_VERSION}/sops-v${SOPS_VERSION}.linux.${SOPS_ARCH}" \
    && chmod +x /usr/local/bin/sops

# Copy the compiled binary from the builder stage
COPY --from=builder /build/cargo /usr/local/bin/cargo

# Expose the default HTTPS REST API port
EXPOSE 8443

# Default workdir inside the container
VOLUME ["/root/.cargo"]

ENTRYPOINT ["cargo"]
CMD ["server"]
