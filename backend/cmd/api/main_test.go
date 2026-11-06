package main

import "testing"

func TestListenAddressIsLoopbackOnly(t *testing.T) {
	for _, tc := range []struct {
		addr  string
		valid bool
	}{
		{"127.0.0.1:8080", true}, {"[::1]:8080", true}, {"127.0.0.1:18080", true},
		{"0.0.0.0:8080", false}, {":8080", false}, {"192.0.2.1:8080", false}, {"127.0.0.1:0", false}, {"127.0.0.1:65536", false}, {"invalid", false},
	} {
		if validLocalAddress(tc.addr) != tc.valid {
			t.Fatal("incorrect listen address validation")
		}
	}
}
