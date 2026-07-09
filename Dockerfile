
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o billing-app ./cmd

FROM alpine:3.20

WORKDIR /app

# Copy the  binary from prev stage
COPY --from=builder /app/billing-app .

# Expose the port the application listens on
EXPOSE 8080

# Run the binary
CMD ["./billing-app"]