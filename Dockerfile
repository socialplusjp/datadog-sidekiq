FROM golang:1.18-alpine AS builder

RUN apk add ca-certificates git

ARG VERSION="dev"
WORKDIR /app
COPY . /app/
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w -X main.version=${VERSION}" -o build/datadog-sidekiq

FROM scratch

ARG VERSION="dev"
ARG REVISION="unknown"

LABEL org.opencontainers.image.source="https://github.com/socialplusjp/datadog-sidekiq"
LABEL org.opencontainers.image.revision="${REVISION}"

COPY --from=builder /app/build/datadog-sidekiq /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["./datadog-sidekiq"]
