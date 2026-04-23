# ---------- build stage ----------
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./src


# ---------- runtime stage ----------
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/server /app/seed_profiles.json ./

CMD ["./server"]
