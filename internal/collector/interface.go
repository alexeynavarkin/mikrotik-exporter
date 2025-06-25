package collector

import (
	"context"
	"log"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

func (c *MikroTikCollector) collectInterfaceMetrics(ctx context.Context, target Target, ch chan<- prometheus.Metric) {
	res, err := target.Client.RunContext(ctx, "/interface/print")
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
					name, "rx", target.Name,
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
					name, "tx", target.Name,
				)
			}
		}
	}
}
