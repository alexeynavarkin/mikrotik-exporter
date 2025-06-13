package collector

import (
	"context"
	"time"

	"github.com/go-routeros/routeros/v3"
	"github.com/prometheus/client_golang/prometheus"
)

// MikroTikCollector collects metrics from a MikroTik device
type MikroTikCollector struct {
	client *routeros.Client

	interfaceTraffic     *prometheus.Desc
	wireguardPeerTraffic *prometheus.Desc
}

// NewMikroTikCollector creates a new collector
func NewMikroTikCollector(client *routeros.Client) *MikroTikCollector {
	return &MikroTikCollector{
		client: client,
		interfaceTraffic: prometheus.NewDesc(
			"mikrotik_interface_traffic_bytes",
			"Interface received and transmitted bytes",
			[]string{"interface", "direction"},
			nil,
		),
		wireguardPeerTraffic: prometheus.NewDesc(
			"mikrotik_wireguard_peer_traffic_bytes",
			"Wireguard peer recceived and trasmitted bytes",
			[]string{"interface", "peer", "direction"},
			nil,
		),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
func (c *MikroTikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.interfaceTraffic

	ch <- c.wireguardPeerTraffic
}

// Collect fetches metrics from MikroTik and sends them to the provided channel
func (c *MikroTikCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c.collectInterfaceMetrics(ctx, ch)

	c.collectWireguardMetrics(ctx, ch)
}
