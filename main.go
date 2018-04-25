package main

import (
	"log"

	"github.com/DataDog/datadog-go/statsd"
	"github.com/go-redis/redis"
)

func fetchMetrics(c *redis.Client) (map[string]float64, error) {
	namespace := "development"
	metrics := make(map[string]float64)

	queues, err := c.SMembers(namespace + ":queues").Result()
	if err != nil {
		return nil, err
	}

	for _, queue := range queues {
		enqueued, err := c.LLen(namespace + ":queue:" + queue).Result()
		if err != nil {
			return nil, err
		}
		metrics["queue."+queue] = float64(enqueued)
	}

	retries, err := c.ZCard(namespace + ":retries").Result()
	if err != nil {
		return nil, err
	}
	metrics["retries"] = float64(retries)

	schedule, err := c.ZCard(namespace + ":schedule").Result()
	if err != nil {
		return nil, err
	}
	metrics["schedule"] = float64(schedule)

	dead, err := c.ZCard(namespace + ":dead").Result()
	if err != nil {
		return nil, err
	}
	metrics["dead"] = float64(dead)

	return metrics, nil
}

func main() {
	statsdClient, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		log.Fatal(err)
	}

	statsdClient.Namespace = "sidekiq."

	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		Password: "",
		DB: 0,
	})

	metrics, err := fetchMetrics(redisClient)

	for k, v := range metrics {
		if err = statsdClient.Gauge(k, v, nil, 1); err != nil {
			log.Fatal(err)
		}
	}
}
