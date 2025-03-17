package main

import (
	"testing"
	"time"
)

var testCases = []struct {
	input  []string
	output string
}{
	{[]string{""}, ""},
	{[]string{"queues"}, "queues"},
	{[]string{"", "queues"}, "queues"},
	{[]string{"queues", ""}, "queues"},
	{[]string{"namespace", "queues"}, "namespace:queues"},
	{[]string{"namespace", "queue", "default"}, "namespace:queue:default"},
	{[]string{"", "queue", ""}, "queue"},
}

func TestMakeRedisKey(t *testing.T) {
	for _, testCase := range testCases {
		expect := testCase.output
		actual := makeRedisKey(testCase.input)

		if expect != actual {
			t.Errorf("Expect %q, Actual %q (input: %q)", expect, actual, testCase.input)
		}
	}
}

func TestCalculateQueueLatency(t *testing.T) {
	timeNow = func() time.Time { return time.Date(2025, 1, 1, 0, 1, 0, 123000000, time.UTC) }
	expect := 60.0

	t.Run("When passed Sidekiq 8.0 timestamp format", func(t *testing.T) {
		jobParams := `
        {
            "retry": false,
            "queue": "default",
            "created_at": 1735689600123,
            "enqueued_at": 1735689600123
        }` // enqueued_at 2025-01-01 00:00:00.123
		actual := calculateQueueLatency(jobParams)
		if expect != actual {
			t.Errorf("Expect %f, Actual %f (input: %q)", expect, actual, jobParams)
		}
	})

	t.Run("When passed timestamp format before Sidekiq 8.0", func(t *testing.T) {
		jobParams := `
        {
            "retry": false,
            "queue": "default",
            "created_at": 1735689600.123,
            "enqueued_at": 1735689600.123
        }` // enqueued_at 2025-01-01 00:00:00.123
		actual := calculateQueueLatency(jobParams)
		if expect != actual {
			t.Errorf("Expect %f, Actual %f (input :%q)", expect, actual, jobParams)
		}
	})
}
