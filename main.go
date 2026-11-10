package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/go-redis/redis/v8"
	"github.com/socialplusjp/datadog-sidekiq/slice"
)

var version = "dev"
var timeNow = time.Now

// for Sidekiq 8.0.x's timestamp format
type JobParams struct {
	EnqueuedAt int64 `json:"enqueued_at"`
}

func makeRedisKey(keys []string) string {
	keys = slice.Delete(keys, "")
	return strings.Join(keys, ":")
}

func calculateQueueLatency(contents string) float64 {
	if contents == "" {
		return 0
	}

	var params JobParams
	if err := json.Unmarshal([]byte(contents), &params); err != nil {
		return calculateLegacyQueueLatency(contents)
	}

	if params.EnqueuedAt == 0 {
		return 0
	}

	return float64(timeNow().UnixMilli()-params.EnqueuedAt) / 1000.0
}

func calculateLegacyQueueLatency(contents string) float64 {
	var job map[string]interface{}
	if err := json.Unmarshal([]byte(contents), &job); err != nil {
		log.Println(err)
		return 0
	}

	if enqueuedAt, exists := job["enqueued_at"]; exists {
		latency := float64(timeNow().UnixMicro())/1000000.0 - enqueuedAt.(float64)
		return latency
	}

	return 0
}

func fetchMetrics(ctx context.Context, c *redis.Client, namespace string) (map[string]float64, error) {
	metrics := make(map[string]float64)

	queues, err := c.SMembers(ctx, makeRedisKey([]string{namespace, "queues"})).Result()
	if err != nil {
		return nil, err
	}

	var enqueuedSum float64
	for _, queue := range queues {
		contents, err := c.LIndex(ctx, makeRedisKey([]string{namespace, "queue", queue}), -1).Result()
		if err == nil {
			latency := calculateQueueLatency(contents)
			metrics["latency."+queue] = latency
		} else {
			metrics["latency."+queue] = 0.0
		}

		enqueued, err := c.LLen(ctx, makeRedisKey([]string{namespace, "queue", queue})).Result()
		if err != nil {
			return nil, err
		}
		metrics["queue."+queue] = float64(enqueued)
		enqueuedSum += float64(enqueued)
	}
	metrics["enqueued"] = float64(enqueuedSum)

	retries, err := c.ZCard(ctx, makeRedisKey([]string{namespace, "retries"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["retries"] = float64(retries)

	schedule, err := c.ZCard(ctx, makeRedisKey([]string{namespace, "schedule"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["schedule"] = float64(schedule)

	processes, err := c.SMembers(ctx, makeRedisKey([]string{namespace, "processes"})).Result()
	if err != nil {
		return nil, err
	}

	for _, process := range processes {
		busy, err := c.HGet(ctx, makeRedisKey([]string{namespace, process}), "busy").Float64()
		if err != nil {
			// a stopped process can remain in processes for a while, which is not worth logging on every interval
			if !errors.Is(err, redis.Nil) {
				log.Printf("failed to get busy of %s: %v", makeRedisKey([]string{namespace, process}), err)
			}
			continue
		}
		metrics["busy"] += busy
	}

	dead, err := c.ZCard(ctx, makeRedisKey([]string{namespace, "dead"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["dead"] = float64(dead)

	return metrics, nil
}

// gaugeFlusher is the part of the DogStatsD client that sendMetrics uses.
type gaugeFlusher interface {
	Gauge(name string, value float64, tags []string, rate float64) error
	Flush() error
}

// completeStatsdAddr fills in the address in the same way as statsd.New, which does not export it:
// an empty addr falls back to DD_AGENT_HOST (and DD_DOGSTATSD_PORT), and a host without a port gets 8125.
func completeStatsdAddr(addr string) string {
	envPort := ""
	if addr == "" {
		addr = os.Getenv("DD_AGENT_HOST")
		envPort = os.Getenv("DD_DOGSTATSD_PORT")
	}
	if addr == "" {
		return ""
	}

	if !strings.HasPrefix(addr, statsd.WindowsPipeAddressPrefix) && !strings.HasPrefix(addr, statsd.UnixAddressPrefix) && !strings.Contains(addr, ":") {
		if envPort == "" {
			envPort = "8125"
		}
		addr = addr + ":" + envPort
	}

	return addr
}

// resolveStatsdAddr resolves the host of addr with ctx. statsd.New resolves it without a context,
// so a DNS that does not respond blocks the call regardless of the interval or SIGTERM.
// An address that cannot be handled here is returned as it is, so that statsd.New reports it as before.
func resolveStatsdAddr(ctx context.Context, resolver *net.Resolver, addr string) (string, error) {
	addr = completeStatsdAddr(addr)
	if addr == "" || strings.HasPrefix(addr, statsd.UnixAddressPrefix) || strings.HasPrefix(addr, statsd.WindowsPipeAddressPrefix) {
		return addr, nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" || net.ParseIP(host) != nil {
		return addr, nil
	}

	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return "", err
	}

	// keep the whole IPAddr so that the zone of a scoped IPv6 address (fe80::1%eth0) is not lost
	selected := ips[0]
	for _, candidate := range ips {
		if candidate.IP.To4() != nil {
			selected = candidate
			break
		}
	}

	return net.JoinHostPort(selected.String(), port), nil
}

func newStatsdClient(ctx context.Context, addr string) (*statsd.Client, error) {
	resolved, err := resolveStatsdAddr(ctx, net.DefaultResolver, addr)
	if err != nil {
		return nil, err
	}

	client, err := statsd.New(resolved)
	if err != nil {
		return nil, err
	}

	client.Namespace = "sidekiq."

	return client, nil
}

func sendMetrics(ctx context.Context, c *redis.Client, g gaugeFlusher, namespace string, tags []string) error {
	metrics, err := fetchMetrics(ctx, c, namespace)
	if err != nil {
		return err
	}

	for k, v := range metrics {
		if err := g.Gauge(k, v, tags, 1); err != nil {
			return err
		}
	}

	return g.Flush()
}

// runLoop calls fn every interval until ctx is done, starting immediately.
// Each call is limited to interval so that a stuck Redis does not block the loop.
// A failed call is logged and retried on the next tick, but maxConsecutiveErrors
// failures in a row return an error so that the process can be restarted. If it is 0,
// runLoop never gives up.
func runLoop(ctx context.Context, interval time.Duration, maxConsecutiveErrors int, fn func(context.Context) error) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	consecutiveErrors := 0
	for {
		cycleCtx, cancel := context.WithTimeout(ctx, interval)
		err := fn(cycleCtx)
		cancel()

		switch {
		case ctx.Err() != nil:
			// a call interrupted by a stop request is not a failure
			return nil
		case err == nil:
			consecutiveErrors = 0
		default:
			consecutiveErrors++
			log.Printf("failed to send metrics (%d consecutive): %v", consecutiveErrors, err)
			if maxConsecutiveErrors > 0 && consecutiveErrors >= maxConsecutiveErrors {
				return fmt.Errorf("giving up after %d consecutive errors: %w", consecutiveErrors, err)
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// newTLSDialer returns a Dialer that applies ctx to the TLS handshake as well.
// go-redis's default one ignores ctx, so a Redis that does not respond to the handshake
// blocks the call until DialTimeout, regardless of the interval or SIGTERM.
func newTLSDialer(tlsConfig *tls.Config) func(ctx context.Context, network, addr string) (net.Conn, error) {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 5 * time.Minute},
		Config:    tlsConfig,
	}
	return dialer.DialContext
}

func main() {
	isShowVersion := flag.Bool("version", false, "Show datadog-sidekiq version")
	statsdHost := flag.String("statsd-host", "127.0.0.1:8125", "DogStatsD host")
	redisNamespace := flag.String("redis-namespace", "", "Redis namespace for Sidekiq")
	redisHost := flag.String("redis-host", "127.0.0.1:6379", "Redis host")
	redisUser := flag.String("redis-user", "", "Redis username")
	redisPassword := flag.String("redis-password", "", "Redis password")
	redisDB := flag.Int("redis-db", 0, "Redis DB")
	redisTLS := flag.Bool("redis-tls", false, "Use TLS for Redis connection")
	redisTLSInsecure := flag.Bool("redis-tls-insecure", false, "Skip TLS verification for Redis connection (use with caution)")
	tags := flag.String("tags", "", "Add custom metric tags for Datadog. Specify in \"key:value\" format. Separate by comma to specify multiple tags")
	interval := flag.Duration("interval", 0, "Send metrics repeatedly at this interval (e.g. 10s). If 0, send once and exit")
	maxConsecutiveErrors := flag.Int("max-consecutive-errors", 6, "Exit with an error after this many consecutive failures when -interval is set. If 0, never exit on errors")
	flag.Parse()

	if *isShowVersion {
		fmt.Printf("datadog-sidekiq version: %s\n", version)
		return
	}

	if *interval < 0 {
		log.Fatal("-interval must not be negative")
	}
	if *maxConsecutiveErrors < 0 {
		log.Fatal("-max-consecutive-errors must not be negative")
	}

	var tlsConfig *tls.Config
	if *redisTLS {
		tlsConfig = &tls.Config{}
	}
	if *redisTLSInsecure {
		if !*redisTLS {
			log.Fatal("-redis-tls-insecure can only be used with -redis-tls")
		}
		tlsConfig.InsecureSkipVerify = true
	}

	redisOptions := &redis.Options{
		Addr:      *redisHost,
		Username:  *redisUser,
		Password:  *redisPassword,
		DB:        *redisDB,
		TLSConfig: tlsConfig,
	}
	if tlsConfig != nil {
		redisOptions.Dialer = newTLSDialer(tlsConfig)
	}
	redisClient := redis.NewClient(redisOptions)

	separatedTags := strings.Split(*tags, ",")
	send := func(ctx context.Context) error {
		// create a client on every run so that the address of DogStatsD is resolved again
		statsdClient, err := newStatsdClient(ctx, *statsdHost)
		if err != nil {
			return err
		}
		defer func() { _ = statsdClient.Close() }()

		return sendMetrics(ctx, redisClient, statsdClient, *redisNamespace, separatedTags)
	}

	if *interval == 0 {
		if err := send(context.Background()); err != nil {
			log.Fatal(err)
		}
		return
	}

	// exit quietly on SIGTERM, e.g. when a Pod is stopped
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	log.Printf("sending metrics every %s", *interval)
	err := runLoop(ctx, *interval, *maxConsecutiveErrors, send)
	stop()
	_ = redisClient.Close()
	if err != nil {
		log.Fatal(err)
	}
}
