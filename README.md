# datadog-sidekiq

Send Sidekiq metrics to Datadog via DogStatsD.

## Installation

Grab the latest binary from the GitHub [releases](https://github.com/feedforce/datadog-sidekiq/releases) page.

## Usage

```
$ datadog-sidekiq
```

In production, recommend using crontab etc. to run every minute.

```
$ crontab -l
* * * * * /usr/local/bin/datadog-sidekiq
```

### Options

| Option | Description | Default value |
| --- | --- | --- |
| `-redis-db` | Redis DB | 0 |
| `-redis-host` | Redis host | 127.0.0.1:6379 |
| `-redis-namespace` | Redis namespace for Sidekiq | |
| `-redis-password` | Redis password | |
| `-statsd-host` | DogStatsD host | 127.0.0.1:8125 |

## Development

### Requirements

* Docker
* Go `~> 1.10.0`

### Local development

Recommend using [dogstatsd-local](https://github.com/jonmorehouse/dogstatsd-local).

```
$ make deps
$ docker run --rm -d -p 8125:8125/udp --name dogstatsd-local jonmorehouse/dogstatsd-local
$ docker run --rm -d -p 6379:6379 redis:alpine
$ go run main.go
$ docker logs dogstatsd-local
2018/04/26 03:15:29 listening over UDP at  0.0.0.0:8125
sidekiq.retries:0.000000|g
sidekiq.dead:0.000000|g
sidekiq.schedule:0.000000|g
```

### Release

1. Create and export `$GITHUB_TOKEN` required from [ghr](https://github.com/tcnksm/ghr#github-api-token)
1. Run `$ git checkout master && git pull origin master`
1. Bump version in [Makefile](https://github.com/feedforce/datadog-sidekiq/blob/master/Makefile#L3)
1. Run `$ git commit -am "Bump version" && git push origin master`
1. Run `$ make release`
