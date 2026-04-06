package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"merionyx/api-gateway/internal/controller/config"
	"merionyx/api-gateway/internal/controller/container"
	"merionyx/api-gateway/internal/controller/server"
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

	c, err := container.NewContainer(cfg)
	if err != nil {
		return fmt.Errorf("init container: %w", err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, cancelRun := context.WithCancel(sigCtx)
	defer cancelRun()

	var failOnce sync.Once
	onFail := func() { failOnce.Do(cancelRun) }

	c.StartKubernetesDiscovery(runCtx)

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

	run("http_probe", func(ctx context.Context) error { return server.RunHTTPServer(ctx, c) })
	run("metrics_http", func(ctx context.Context) error { return metricshttp.ListenAndServeUntil(ctx, cfg.MetricsHTTP) })
	run("grpc_control_plane", func(ctx context.Context) error { return server.RunGRPCServer(ctx, c) })
	run("grpc_xds", func(ctx context.Context) error {
		xdsPort, err := strconv.Atoi(c.Config.Server.XDSPort)
		if err != nil {
			return fmt.Errorf("parse xDS port %q: %w", c.Config.Server.XDSPort, err)
		}
		return c.XDSServer.Run(ctx, xdsPort)
	})

	wg.Wait()
	logger.Info("shutting down")
	c.Close()
	return nil
}
