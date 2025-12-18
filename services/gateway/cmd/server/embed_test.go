package main

import (
	"net/http"
	"testing"
	"time"
)

func TestStartEmbeddedServices(t *testing.T) {
	go startEmbeddedServices()
	// wait for servers to start with a small retry loop
	endpoints := []string{"http://localhost:5100/auth/whoami", "http://localhost:5200/files/list", "http://localhost:5300/share/list"}
	deadline := time.Now().Add(2 * time.Second)
	for _, ep := range endpoints {
		var ok bool
		for time.Now().Before(deadline) {
			resp, err := http.Get(ep)
			if err == nil {
				resp.Body.Close()
				ok = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if !ok {
			t.Fatalf("endpoint %s did not start in time", ep)
		}
	}
}
