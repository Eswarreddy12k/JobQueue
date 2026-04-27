# Stage 1: Build both Go binaries
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy dependency files first — this layer is cached until go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build both binaries
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /autoscaler ./cmd/autoscaler

# Stage 2: Minimal runtime image (~20MB instead of ~800MB)
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /api /api
COPY --from=builder /worker /worker
COPY --from=builder /autoscaler /autoscaler
