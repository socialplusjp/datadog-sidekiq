FROM golang:1.9-alpine as builder

ENV PROJECT_PATH="/go/src/github.com/feedforce/datadog-sidekiq"

RUN apk add --update \
        ca-certificates \
        git \
        curl && \
    curl -L https://raw.githubusercontent.com/golang/dep/master/install.sh | sh

WORKDIR $PROJECT_PATH
COPY . $PROJECT_PATH/
RUN dep ensure && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags="-s -w" -o build/datadog-sidekiq

FROM scratch

ENV PROJECT_PATH="/go/src/github.com/feedforce/datadog-sidekiq"

COPY --from=builder $PROJECT_PATH/build/datadog-sidekiq /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["./datadog-sidekiq"]
