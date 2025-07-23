package src

import "testing"

func TestSomething(t *testing.T) {
	t.Log("This is a test for something.")
}

func TestSomethingFailure(t *testing.T) {
	t.Error("This test is expected to fail.")
}
