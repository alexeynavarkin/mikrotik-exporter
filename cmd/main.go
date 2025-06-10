package main

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/go-routeros/routeros/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/collector"
)

func main() {
	// Connect to MikroTik using v3 client
	client, err := routeros.DialTLS(
		"",
		"",
		"",
		&tls.Config{
			InsecureSkipVerify: true,
		},
	)
	if err != nil {
		log.Fatalf("Failed to connect to MikroTik: %v", err)
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
