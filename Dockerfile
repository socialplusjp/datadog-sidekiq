FROM golang:1.17-alpine as builder

RUN apk add ca-certificates git

WORKDIR /app
COPY . /app/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o build/datadog-sidekiq

FROM scratch

COPY --from=builder /app/build/datadog-sidekiq /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["./datadog-sidekiq"]
