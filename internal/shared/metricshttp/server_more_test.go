package metricshttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type failRegisterer struct{}

func (failRegisterer) Register(prometheus.Collector) error { return errors.New("register failed") }

func (failRegisterer) MustRegister(...prometheus.Collector) {}

func (failRegisterer) Unregister(prometheus.Collector) bool { return false }

func TestRegisterCollector_AlreadyRegistered(t *testing.T) {
	reg := prometheus.NewRegistry()
	c := collectors.NewGoCollector()
	if err := reg.Register(c); err != nil {
		t.Fatal(err)
	}
	registerCollector(reg, collectors.NewGoCollector())
}

func TestRegisterCollector_OtherErrorLogs(t *testing.T) {
	registerCollector(failRegisterer{}, collectors.NewGoCollector())
}

func pickFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestListenAndServeUntil_ServesMetricsAndStopsOnCancel(t *testing.T) {
	port := pickFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ListenAndServeUntil(ctx, Config{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    strconv.Itoa(port),
			Path:    "/custom-metrics",
		})
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/custom-metrics", port)
	var lastErr error
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
		if i == 49 {
			t.Fatalf("metrics not reachable: %v", lastErr)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop")
	}
}

func TestListenAndServeUntil_DefaultPathHostPort(t *testing.T) {
	port := pickFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = ListenAndServeUntil(ctx, Config{
			Enabled: true,
			Port:    strconv.Itoa(port),
		})
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/metrics", port)
	for i := 0; i < 50; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				cancel()
				<-done
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	t.Fatal("metrics not reachable on defaults")
}

func TestListenAndServe_Disabled(t *testing.T) {
	if err := ListenAndServe(Config{Enabled: false}); err != nil {
		t.Fatal(err)
	}
}

func TestListenAndServeUntil_ListenError(t *testing.T) {
	err := ListenAndServeUntil(context.Background(), Config{
		Enabled: true,
		Host:    "127.0.0.1",
		Port:    "100000",
		Path:    "/m",
	})
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}
