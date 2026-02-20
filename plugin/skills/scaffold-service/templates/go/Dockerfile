FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init __PROJECT_NAME__-backend && CGO_ENABLED=0 go build -o api .

FROM alpine:3.19
COPY --from=builder /app/api .
EXPOSE __BACKEND_PORT__
CMD ["./api"]
