package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
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

func TestNewTLSDialer(t *testing.T) {
	// a server that accepts connections but never answers the TLS handshake
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			defer func() { _ = conn.Close() }()
		}
	}()

	dial := newTLSDialer(&tls.Config{})

	t.Run("When the handshake does not finish before the context is done", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		conn, err := dial(ctx, "tcp", listener.Addr().String())
		elapsed := time.Since(start)
		if err == nil {
			_ = conn.Close()
			t.Fatal("Expect an error, Actual nil")
		}
		if elapsed > time.Second {
			t.Errorf("Expect to give up by the context (100ms), Actual %s", elapsed)
		}
	})
}

func TestNewStatsdClient(t *testing.T) {
	t.Run("When passed a valid address", func(t *testing.T) {
		client, err := newStatsdClient(context.Background(), "127.0.0.1:8125")
		if err != nil {
			t.Fatalf("Expect nil, Actual %v", err)
		}
		defer func() { _ = client.Close() }()

		expect := "sidekiq."
		if client.Namespace != expect {
			t.Errorf("Expect %q, Actual %q", expect, client.Namespace)
		}
	})

	t.Run("When passed an invalid address", func(t *testing.T) {
		client, err := newStatsdClient(context.Background(), "127.0.0.1:invalid-port")
		if err == nil {
			_ = client.Close()
			t.Error("Expect an error, Actual nil")
		}
	})
}

func TestResolveStatsdAddr(t *testing.T) {
	t.Run("When the address does not need to be resolved", func(t *testing.T) {
		for _, addr := range []string{"127.0.0.1:8125", "[::1]:8125", ":8125", "unix:///var/run/datadog/dsd.socket", `\\.\pipe\dsd`} {
			actual, err := resolveStatsdAddr(context.Background(), net.DefaultResolver, addr)
			if err != nil {
				t.Errorf("Expect nil, Actual %v (input: %q)", err, addr)
			}
			if actual != addr {
				t.Errorf("Expect %q, Actual %q", addr, actual)
			}
		}
	})

	t.Run("When the address has no port", func(t *testing.T) {
		if _, err := resolveStatsdAddr(context.Background(), net.DefaultResolver, "datadog-agent"); err == nil {
			t.Error("Expect an error, Actual nil")
		}
	})

	t.Run("When the DNS does not respond before the context is done", func(t *testing.T) {
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := resolveStatsdAddr(ctx, resolver, "datadog-agent.invalid:8125")
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("Expect an error, Actual nil")
		}
		if elapsed > time.Second {
			t.Errorf("Expect to give up by the context (100ms), Actual %s", elapsed)
		}
	})
}
