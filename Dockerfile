# --- Stage 1: Build the Go application ---
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /app

COPY Backend/go.mod Backend/go.sum ./
RUN go mod download

COPY Backend/ .
RUN go build


# --- Stage 2: Create a minimal production image ---
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/piscord-backend .

ENV PORT=$PORT
EXPOSE 8000

ENTRYPOINT ["./piscord-backend"]
