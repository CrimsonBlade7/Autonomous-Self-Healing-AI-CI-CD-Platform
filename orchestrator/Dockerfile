FROM golang:1.27 AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/orchestrator-bin ./cmd

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /out/orchestrator-bin ./orchestrator-bin
COPY config ./config
RUN mkdir -p ./workspaces
EXPOSE 8080
ENTRYPOINT [ "./orchestrator-bin" ]