package collector

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/go-routeros/routeros/v3"
	"github.com/prometheus/client_golang/prometheus"
)

// MikroTikCollector collects metrics from a MikroTik device
type MikroTikCollector struct {
	client *routeros.Client

	interfaceRxTx *prometheus.Desc
}

// NewMikroTikCollector creates a new collector
func NewMikroTikCollector(client *routeros.Client) *MikroTikCollector {
	return &MikroTikCollector{
		client: client,
		interfaceRxTx: prometheus.NewDesc(
			"mikrotik_interface_traffic_bytes",
			"Interface received and transmitted bytes",
			[]string{"interface", "direction"}, nil,
		),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
func (c *MikroTikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.interfaceRxTx
}

// Collect fetches metrics from MikroTik and sends them to the provided channel
func (c *MikroTikCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Collect interface metrics
	c.collectInterfaceMetrics(ctx, ch)
}

func (c *MikroTikCollector) collectInterfaceMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	res, err := c.client.RunContext(ctx, "/interface/print")
	if err != nil {
		log.Printf("Error getting interface metrics: %v", err)
		return
	}

	for _, re := range res.Re {
		name, ok := re.Map["name"]
		if !ok {
			continue
		}

		if rx, ok := re.Map["rx-byte"]; ok {
			value, err := strconv.ParseFloat(rx, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(
					c.interfaceRxTx,
					prometheus.CounterValue,
					value,
					name, "rx",
				)
			}
		}

		if tx, ok := re.Map["tx-byte"]; ok {
			value, err := strconv.ParseFloat(tx, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(
					c.interfaceRxTx,
					prometheus.CounterValue,
					value,
					name, "tx",
				)
			}
		}
	}
}
