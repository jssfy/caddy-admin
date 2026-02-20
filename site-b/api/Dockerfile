FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init site-b-api && CGO_ENABLED=0 go build -o api .

FROM alpine:3.19
COPY --from=builder /app/api .
EXPOSE 8082
CMD ["./api"]
