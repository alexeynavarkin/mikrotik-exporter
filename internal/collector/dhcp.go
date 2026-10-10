package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type dhcpCollector struct {
	leases *prometheus.Desc
}

func newDHCPCollector() subCollector {
	return &dhcpCollector{
		leases: newDesc("dhcp_leases", "Number of DHCP server leases", "server", "status"),
	}
}

func (c *dhcpCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.leases
}

func (c *dhcpCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(ctx, "/ip/dhcp-server/lease/print", "=.proplist=server,status")
	if err != nil {
		return fmt.Errorf("failed to list dhcp leases: %w", err)
	}

	type key struct{ server, status string }
	counts := map[key]float64{}
	for _, re := range res.Re {
		counts[key{re.Map["server"], re.Map["status"]}]++
	}

	for k, count := range counts {
		ch <- prometheus.MustNewConstMetric(c.leases, prometheus.GaugeValue, count, k.server, k.status, target.Name)
	}

	return nil
}
