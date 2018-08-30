package main

import "testing"

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
