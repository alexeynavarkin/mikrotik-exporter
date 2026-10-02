package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	config "github.com/ThomasObenaus/go-conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/collector"
	"github.com/alexeynavarkin/mikrotik-exporter/internal/mikrotik"
)

type TargetConfig struct {
	Address    string   `cfg:"{'name':'address'}"`
	Username   string   `cfg:"{'name':'username'}"`
	Password   string   `cfg:"{'name':'password'}"`
	Name       string   `cfg:"{'name':'name'}"`
	PlainText  bool     `cfg:"{'name':'plaintext','desc':'Use plain API (8728) instead of API-SSL','default':false}"`
	Collectors []string `cfg:"{'name':'collectors','desc':'Enabled collectors, defaults are used when empty','default':[]}"`
}

type Config struct {
	ListenAddress string         `cfg:"{'name':'listen-address','desc':'Address to expose metrics on','default':':9100'}"`
	Timeout       time.Duration  `cfg:"{'name':'timeout','desc':'Timeout for collecting metrics from all targets','default':'10s'}"`
	Targets       []TargetConfig `cfg:"{'name':'targets'}"`
}

func main() {
	cfg := Config{}

	cfgProvider, err := config.NewConfigProvider(
		&cfg,
		"MIKROTIK_EXPORTER",
		"MIKROTIK_EXPORTER",
	)
	if err != nil {
		log.Fatalf("failed to build config provider: %v", err)
	}

	err = cfgProvider.ReadConfig(os.Args)
	if err != nil {
		log.Println("failed to load config", err)
		log.Println(cfgProvider.Usage())
		os.Exit(1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to build logger: %v", err)
	}
	defer func() { _ = lg.Sync() }()

	targets, err := buildTargets(cfg.Targets, lg)
	if err != nil {
		lg.Fatal("invalid config", zap.Error(err))
	}

	prometheus.MustRegister(collector.NewMikroTikCollector(targets, cfg.Timeout, lg))

	http.Handle("/metrics", promhttp.Handler())
	lg.Info("starting server", zap.String("address", cfg.ListenAddress), zap.Int("targets", len(targets)))
	lg.Fatal("server stopped", zap.Error(http.ListenAndServe(cfg.ListenAddress, nil)))
}

func buildTargets(cfgs []TargetConfig, lg *zap.Logger) ([]collector.Target, error) {
	if len(cfgs) == 0 {
		return nil, fmt.Errorf("no targets configured")
	}

	seen := map[string]bool{}
	targets := make([]collector.Target, 0, len(cfgs))
	for _, target := range cfgs {
		if target.Name == "" || target.Address == "" {
			return nil, fmt.Errorf("target name and address are required")
		}
		if seen[target.Name] {
			return nil, fmt.Errorf("duplicate target name %q", target.Name)
		}
		seen[target.Name] = true

		if err := collector.ValidateCollectors(target.Collectors); err != nil {
			return nil, fmt.Errorf("target %q: %w", target.Name, err)
		}

		client := mikrotik.NewRetryClient(
			mikrotik.RetryClientConfig{
				Username:   target.Username,
				Password:   target.Password,
				Address:    target.Address,
				RetryCount: 2,
				PlainText:  target.PlainText,
			},
			lg,
		)

		targets = append(
			targets,
			collector.Target{
				Name:       target.Name,
				Client:     client,
				Collectors: target.Collectors,
			},
		)
	}

	return targets, nil
}
