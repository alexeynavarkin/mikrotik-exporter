package main

import (
	"log"
	"net/http"
	"os"

	config "github.com/ThomasObenaus/go-conf"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/collector"
	"github.com/alexeynavarkin/mikrotik-exporter/internal/mikrotik"
)

type Config struct {
	Targets []struct {
		Address  string `cfg:"{'name':'address'}"`
		Username string `cfg:"{'name':'username'}"`
		Password string `cfg:"{'name':'password'}"`
		Name     string `cfg:"{'name':'name'}"`
	} `cfg:"{'name':'targets'}"`
}

func main() {
	cfg := Config{}

	cfgProvider, err := config.NewConfigProvider(
		&cfg,
		"MIKROTIK_EXPORTER",
		"MIKROTIK_EXPORTER",
	)
	if err != nil {
		log.Fatal("failed to build config provider %w", err)
	}

	err = cfgProvider.ReadConfig(os.Args)
	if err != nil {
		log.Println("failed to load config", err)
		log.Println(cfgProvider.Usage())
		os.Exit(-1)
	}

	lg, err := zap.NewProduction()
	if err != nil {
		log.Println("failed to build logger", err)
		os.Exit(-1)
	}

	targets := make([]collector.Target, 0)
	for _, target := range cfg.Targets {
		client := mikrotik.NewRetryClient(
			mikrotik.RetryClientConfig{
				Username:   target.Username,
				Password:   target.Password,
				Address:    target.Address,
				RetryCount: 2,
			},
			lg,
		)
		if err != nil {
			log.Fatalf("failed to connect to MikroTik: %v", err)
		}

		targets = append(
			targets,
			collector.Target{
				Name:   target.Name,
				Client: client,
			},
		)
	}

	collector := collector.NewMikroTikCollector(targets)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting server on :9100")
	log.Fatal(http.ListenAndServe(":9100", nil))
}
