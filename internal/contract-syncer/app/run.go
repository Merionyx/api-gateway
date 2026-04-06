package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"merionyx/api-gateway/internal/contract-syncer/config"
	"merionyx/api-gateway/internal/contract-syncer/container"
	"merionyx/api-gateway/internal/contract-syncer/server"
	"merionyx/api-gateway/internal/shared/metricshttp"
	"merionyx/api-gateway/internal/shared/serviceapp"
)

func Run() error {
	logger := serviceapp.NewJSONLogger()
	serviceapp.SetDefaultLogger(logger)

	cfg, err := config.LoadConfig(os.Getenv("CONFIG_PATH"))
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger.Info("config loaded", "config", cfg)

	cnt, err := container.NewContainer(cfg)
	if err != nil {
		return fmt.Errorf("init container: %w", err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, cancelRun := context.WithCancel(sigCtx)
	defer cancelRun()

	var failOnce sync.Once
	onFail := func() { failOnce.Do(cancelRun) }

	var wg sync.WaitGroup
	run := func(name string, fn func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(runCtx); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("server stopped", "name", name, "error", err)
				onFail()
			}
		}()
	}

	run("grpc", func(c context.Context) error { return server.RunGRPCServer(c, cnt) })
	run("metrics_http", func(c context.Context) error { return metricshttp.ListenAndServeUntil(c, cfg.MetricsHTTP) })

	wg.Wait()
	logger.Info("shutting down")
	cnt.Close()
	return nil
}
