FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

WORKDIR /app/cmd/mattermost-poll

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o mattermost-poll .

FROM alpine:latest
COPY --from=builder /app/cmd/mattermost-poll/mattermost-poll /mattermost-poll

ENTRYPOINT ["/mattermost-poll"]