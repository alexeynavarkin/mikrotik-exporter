package collector

import (
	"context"
	"log"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

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
					c.interfaceTraffic,
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
					c.interfaceTraffic,
					prometheus.CounterValue,
					value,
					name, "tx",
				)
			}
		}
	}
}
