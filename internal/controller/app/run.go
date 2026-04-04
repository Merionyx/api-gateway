package controller

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
		logger.Error(fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	logger.Info("Config loaded", "config", cfg)

	container, err := container.NewContainer(cfg)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to initialize container: %v", err))
		os.Exit(1)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, cancelRun := context.WithCancel(sigCtx)
	defer cancelRun()

	var failOnce sync.Once
	onFail := func() { failOnce.Do(cancelRun) }

	container.StartKubernetesDiscovery(runCtx)

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

	run("http_probe", func(c context.Context) error { return server.RunHTTPServer(c, container) })
	run("metrics_http", func(c context.Context) error { return metricshttp.ListenAndServeUntil(c, cfg.MetricsHTTP) })
	run("grpc_control_plane", func(c context.Context) error { return server.RunGRPCServer(c, container) })
	run("grpc_xds", func(c context.Context) error {
		xdsPort, err := strconv.Atoi(container.Config.Server.XDSPort)
		if err != nil {
			return fmt.Errorf("parse xDS port %q: %w", container.Config.Server.XDSPort, err)
		}
		return container.XDSServer.Run(c, xdsPort)
	})

	wg.Wait()
	logger.Info("Shutting down...")
	container.Close()
	return nil
}
