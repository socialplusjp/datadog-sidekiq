package main

import (
	"context"
	"errors"
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

func TestRunLoop(t *testing.T) {
	interval := 10 * time.Millisecond
	errBoom := errors.New("boom")

	t.Run("When the context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := runLoop(ctx, interval, 3, func(context.Context) error {
			calls++
			if calls == 3 {
				cancel()
			}
			return nil
		})
		if err != nil {
			t.Errorf("Expect nil, Actual %v", err)
		}
		if calls != 3 {
			t.Errorf("Expect 3 calls, Actual %d", calls)
		}
	})

	t.Run("When it fails consecutively", func(t *testing.T) {
		calls := 0
		err := runLoop(context.Background(), interval, 3, func(context.Context) error {
			calls++
			return errBoom
		})
		if !errors.Is(err, errBoom) {
			t.Errorf("Expect %v, Actual %v", errBoom, err)
		}
		if calls != 3 {
			t.Errorf("Expect 3 calls, Actual %d", calls)
		}
	})

	t.Run("When a success resets the count of failures", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		// fail, fail, success, fail, fail, success, then stop
		results := []error{errBoom, errBoom, nil, errBoom, errBoom, nil}
		calls := 0
		err := runLoop(ctx, interval, 3, func(context.Context) error {
			result := results[calls]
			calls++
			if calls == len(results) {
				cancel()
			}
			return result
		})
		if err != nil {
			t.Errorf("Expect nil, Actual %v", err)
		}
		if calls != len(results) {
			t.Errorf("Expect %d calls, Actual %d", len(results), calls)
		}
	})

	t.Run("When max consecutive errors is 0", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		err := runLoop(ctx, interval, 0, func(context.Context) error {
			calls++
			if calls == 10 {
				cancel()
			}
			return errBoom
		})
		if err != nil {
			t.Errorf("Expect nil, Actual %v", err)
		}
		if calls != 10 {
			t.Errorf("Expect 10 calls, Actual %d", calls)
		}
	})

	t.Run("When a call takes longer than the interval", func(t *testing.T) {
		calls := 0
		err := runLoop(context.Background(), interval, 2, func(ctx context.Context) error {
			calls++
			<-ctx.Done()
			return ctx.Err()
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Expect %v, Actual %v", context.DeadlineExceeded, err)
		}
		if calls != 2 {
			t.Errorf("Expect 2 calls, Actual %d", calls)
		}
	})

	t.Run("When the context is canceled during a failing call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		err := runLoop(ctx, interval, 1, func(context.Context) error {
			cancel()
			return errBoom
		})
		if err != nil {
			t.Errorf("Expect nil, Actual %v", err)
		}
	})
}
