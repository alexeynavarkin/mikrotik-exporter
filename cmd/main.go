package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	config "github.com/ThomasObenaus/go-conf"
	"github.com/go-routeros/routeros/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/collector"
)

type Config struct {
	Targets []struct {
		Address  string `cfg:"{'name':'address'}"`
		Username string `cfg:"{'name':'username'}"`
		Password string `cfg:"{'name':'password'}"`
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

	// Connect to MikroTik using v3 client
	client, err := routeros.DialTLS(
		cfg.Targets[0].Address,
		cfg.Targets[0].Username,
		cfg.Targets[0].Password,
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)
	if err != nil {
		log.Fatalf("failed to connect to MikroTik: %v", err)
	}
	defer client.Close()

	// Create and register collector
	collector := collector.NewMikroTikCollector(client)
	prometheus.MustRegister(collector)

	// Set up HTTP server for metrics
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting server on :9100")
	log.Fatal(http.ListenAndServe(":9100", nil))
}
