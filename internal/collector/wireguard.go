package collector

import (
	"context"
	"log"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

func (c *MikroTikCollector) collectWireguardMetrics(ctx context.Context, ch chan<- prometheus.Metric) {
	res, err := c.client.RunContext(
		ctx,
		"/interface/wireguard/peers/print",
		"proplist=interface,name,rx,tx",
	)
	if err != nil {
		log.Printf("Error listing wireguard peers: %v", err)
		return
	}

	for _, re := range res.Re {
		name, ok := re.Map["name"]
		if !ok {
			continue
		}

		iface, ok := re.Map["interface"]
		if !ok {
			continue
		}

		if rx, ok := re.Map["rx"]; ok {
			value, err := ParseBytes(rx)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(
					c.wireguardPeerTraffic,
					prometheus.CounterValue,
					value,
					iface, name, "rx",
				)
			}
		}

		if tx, ok := re.Map["tx"]; ok {
			value, err := strconv.ParseFloat(tx, 64)
			if err == nil {
				ch <- prometheus.MustNewConstMetric(
					c.wireguardPeerTraffic,
					prometheus.CounterValue,
					value,
					iface, name, "tx",
				)
			}
		}
	}
}
