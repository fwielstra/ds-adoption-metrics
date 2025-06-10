package main_test

import "testing"

func TestMain(t *testing.T) {
	if 1 != 1 {
		t.Errorf("Reality is messed up")
	}
}
