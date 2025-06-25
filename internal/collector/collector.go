package collector

import (
	"context"
	"sync"
	"time"

	"github.com/go-routeros/routeros/v3"
	"github.com/prometheus/client_golang/prometheus"
)

type Target struct {
	Name   string
	Client *routeros.Client
}

type MikroTikCollector struct {
	targets []Target

	interfaceTraffic     *prometheus.Desc
	wireguardPeerTraffic *prometheus.Desc
}

func NewMikroTikCollector(targets []Target) *MikroTikCollector {
	return &MikroTikCollector{
		targets: targets,
		interfaceTraffic: prometheus.NewDesc(
			"mikrotik_interface_traffic_bytes",
			"Interface received and transmitted bytes",
			[]string{"interface", "direction", "name"},
			nil,
		),
		wireguardPeerTraffic: prometheus.NewDesc(
			"mikrotik_wireguard_peer_traffic_bytes",
			"Wireguard peer recceived and trasmitted bytes",
			[]string{"interface", "peer", "direction", "name"},
			nil,
		),
	}
}

func (c *MikroTikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.interfaceTraffic

	ch <- c.wireguardPeerTraffic
}

func (c *MikroTikCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	for _, target := range c.targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.collectInterfaceMetrics(ctx, target, ch)
			c.collectWireguardMetrics(ctx, target, ch)
		}()
	}

	wg.Wait()
}
