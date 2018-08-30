package slice

import (
	"reflect"
	"testing"
)

var testCases = []struct {
	input1 []string
	input2 string
	output []string
}{
	{[]string{""}, "", nil},
	{[]string{"queues"}, "", []string{"queues"}},
	{[]string{"", "queues"}, "", []string{"queues"}},
	{[]string{"queues", ""}, "", []string{"queues"}},
	{[]string{"namespace", "queues"}, "", []string{"namespace", "queues"}},
	{[]string{"namespace", "queue", "default"}, "", []string{"namespace", "queue", "default"}},
	{[]string{"", "queue", ""}, "", []string{"queue"}},
}

func TestDelete(t *testing.T) {
	for _, testCase := range testCases {
		expect := testCase.output
		actual := Delete(testCase.input1, testCase.input2)

		if !reflect.DeepEqual(expect, actual) {
			t.Errorf("Expect %q, Actual %q (input1: %q, input2: %q)", expect, actual, testCase.input1, testCase.input2)
		}
	}
}
