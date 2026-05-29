FROM golang:1.24-alpine AS builder

WORKDIR /workspace

# Install kustomize
RUN apk add --no-cache git && \
    go install sigs.k8s.io/kustomize/kustomize/v5@v5.6.0

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build binaries
RUN CGO_ENABLED=0 go build -o /usr/local/bin/env-detector ./cmd/env-detector && \
    CGO_ENABLED=0 go build -o /usr/local/bin/render-diff ./cmd/render-diff

# Final image
FROM alpine:3.21

RUN apk add --no-cache git bash

# Copy binaries from builder
COPY --from=builder /usr/local/bin/env-detector /usr/local/bin/env-detector
COPY --from=builder /usr/local/bin/render-diff /usr/local/bin/render-diff
COPY --from=builder /go/bin/kustomize /usr/local/bin/kustomize

# Copy entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
