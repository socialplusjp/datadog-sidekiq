package main

import (
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/feedforce/datadog-sidekiq/slice"
	"github.com/go-redis/redis"
)

const version = "v0.0.10"

func makeRedisKey(keys []string) string {
	keys = slice.Delete(keys, "")
	return strings.Join(keys, ":")
}

func fetchMetrics(c *redis.Client, namespace string) (map[string]float64, error) {
	metrics := make(map[string]float64)

	queues, err := c.SMembers(makeRedisKey([]string{namespace, "queues"})).Result()
	if err != nil {
		return nil, err
	}

	var enqueuedSum float64
	for _, queue := range queues {
		enqueued, err := c.LLen(makeRedisKey([]string{namespace, "queue", queue})).Result()
		if err != nil {
			return nil, err
		}
		metrics["queue."+queue] = float64(enqueued)
		enqueuedSum += float64(enqueued)
	}
	metrics["enqueued"] = float64(enqueuedSum)

	retries, err := c.ZCard(makeRedisKey([]string{namespace, "retries"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["retries"] = float64(retries)

	schedule, err := c.ZCard(makeRedisKey([]string{namespace, "schedule"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["schedule"] = float64(schedule)

	processes, err := c.SMembers(makeRedisKey([]string{namespace, "processes"})).Result()
	if err != nil {
		return nil, err
	}

	for _, process := range processes {
		busy, err := c.HGet(makeRedisKey([]string{namespace, process}), "busy").Float64()
		if err != nil {
			return nil, err
		}
		metrics["busy"] += busy
	}

	dead, err := c.ZCard(makeRedisKey([]string{namespace, "dead"})).Result()
	if err != nil {
		return nil, err
	}
	metrics["dead"] = float64(dead)

	return metrics, nil
}

func main() {
	isShowVersion := flag.Bool("version", false, "Show datadog-sidekiq version")
	statsdHost := flag.String("statsd-host", "127.0.0.1:8125", "DogStatsD host")
	redisNamespace := flag.String("redis-namespace", "", "Redis namespace for Sidekiq")
	redisHost := flag.String("redis-host", "127.0.0.1:6379", "Redis host")
	redisPassword := flag.String("redis-password", "", "Redis password")
	redisDB := flag.Int("redis-db", 0, "Redis DB")
	tags := flag.String("tags", "", "Add custom metric tags for Datadog. Specify in \"key:value\" format. Separate by comma to specify multiple tags")
	flag.Parse()

	if *isShowVersion {
		fmt.Printf("datadog-sidekiq version: %s\n", version)
		return
	}

	statsdClient, err := statsd.New(*statsdHost)
	if err != nil {
		log.Fatal(err)
	}

	statsdClient.Namespace = "sidekiq."

	redisClient := redis.NewClient(&redis.Options{
		Addr:     *redisHost,
		Password: *redisPassword,
		DB:       *redisDB,
	})

	metrics, err := fetchMetrics(redisClient, *redisNamespace)
	if err != nil {
		log.Fatal(err)
	}

	separatedTags := strings.Split(*tags, ",")

	for k, v := range metrics {
		if err = statsdClient.Gauge(k, v, separatedTags, 1); err != nil {
			log.Fatal(err)
		}
	}
}
