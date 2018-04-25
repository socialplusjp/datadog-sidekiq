package main

import (
	"log"

	"github.com/DataDog/datadog-go/statsd"
)

func main() {
	c, err := statsd.New("127.0.0.1:8125")
	if err != nil {
		log.Fatal(err)
	}

	c.Namespace = "flubber."
	c.Tags = append(c.Tags, "region:us-east-1a")

	if err = c.Gauge("request.duration", 1.2, nil, 1); err != nil {
		log.Fatal(err)
	}
}
