package testutil

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDefaultTransportRefusesNonLoopback(t *testing.T) {
	start := time.Now()
	_, err := http.Get("https://example.com/")
	if err == nil || !strings.Contains(err.Error(), "netguard") {
		t.Fatalf("want netguard error, got %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("refusal was not immediate")
	}
}
