# Stage 1: build
FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o pawonwarga main.go

# Stage 2: minimal runtime image
FROM alpine:3.21

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/pawonwarga .

EXPOSE 8080
CMD ["./pawonwarga"]
