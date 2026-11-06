package main

import (
	"context"
	"strings"
	"testing"
)

func TestInvalidArgumentsDoNotEchoValues(t *testing.T) {
	const marker = "synthetic-redaction-marker"
	for _, args := range [][]string{
		{"--password", marker},
		{"--login-id", "user", marker},
		{"--login-id", marker + " invalid"},
	} {
		err := run(context.Background(), args)
		if err == nil || strings.Contains(err.Error(), marker) {
			t.Fatal("invalid arguments must fail without echoing their contents")
		}
	}
}
