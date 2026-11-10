# datadog-sidekiq

Send Sidekiq metrics to Datadog via DogStatsD.

[![GitHub release](https://img.shields.io/github/release/socialplusjp/datadog-sidekiq.svg?style=flat-square)](https://github.com/socialplusjp/datadog-sidekiq/releases)
[![MIT license](https://img.shields.io/github/license/socialplusjp/datadog-sidekiq.svg?style=flat-square)](https://github.com/socialplusjp/datadog-sidekiq/blob/main/LICENSE)

## Installation

Grab the latest binary from the GitHub [releases](https://github.com/socialplusjp/datadog-sidekiq/releases) page.

or run with Docker.

```
$ docker run -it ghcr.io/socialplusjp/datadog-sidekiq
```

## Usage

```
$ datadog-sidekiq
```

In production, recommend using crontab etc. to run every minute.

```
$ crontab -l
* * * * * /usr/local/bin/datadog-sidekiq
```

### Running continuously

With `-interval`, it keeps running and sends metrics at that interval instead of exiting after one run.
This is useful when you want to send metrics more often than every minute, or run it as a long-lived process such as a Kubernetes Deployment.

```
$ datadog-sidekiq -interval 10s
```

- Each run is limited to `-interval`, so a stuck Redis does not block the loop.
- A failed run is logged and retried at the next interval. After `-max-consecutive-errors` failures in a row, it exits with an error so that the process manager can restart it.
- On SIGINT or SIGTERM, it stops after closing the connections and exits with 0.
- Errors from writing to DogStatsD are not detected, because the DogStatsD client sends asynchronously over UDP. Only failures to fetch from Redis are counted.

### Options

| Option | Description | Default value |
| --- | --- | --- |
| `-interval` | Send metrics repeatedly at this interval (e.g. `10s`). If `0`, send once and exit | 0 |
| `-max-consecutive-errors` | Exit with an error after this many consecutive failures when `-interval` is set. If `0`, never exit on errors | 6 |
| `-redis-db` | Redis DB | 0 |
| `-redis-host` | Redis host | 127.0.0.1:6379 |
| `-redis-namespace` | Redis namespace for Sidekiq | |
| `-redis-user` | Redis username | |
| `-redis-password` | Redis password | |
| `-redis-tls` | Use TLS for Redis connection | false |
| `-redis-tls-insecure` | Skip TLS verification for Redis connection (use with caution) | false |
| `-statsd-host` | DogStatsD host | 127.0.0.1:8125 |
| `-tags` | Add custom metric tags for Datadog. Specify in \"key:value\" format. Separate by comma to specify multiple tags | |
| `-version` | Show datadog-sidekiq version | false |

## Development

### Requirements

* Docker
* Go `>= 1.27.0`

### Local development

```
$ docker-compose up -d
$ docker-compose logs dogstatsd
Attaching to datadog-sidekiq_dogstatsd_1
dogstatsd_1        | 2019/04/12 02:55:17 listening over UDP at  0.0.0.0:8125
dogstatsd_1        | sidekiq.dead:0.000000|g|#tag1:value1,tag2:value2
dogstatsd_1        | sidekiq.retries:0.000000|g|#tag1:value1,tag2:value2
dogstatsd_1        | sidekiq.schedule:0.000000|g|#tag1:value1,tag2:value2
```

### Release

1. Create release for new version on GitHub
1. Release automatically from GitHub Actions
