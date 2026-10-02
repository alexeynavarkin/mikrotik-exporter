package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type interfaceCollector struct {
	traffic   *prometheus.Desc
	packets   *prometheus.Desc
	errors    *prometheus.Desc
	drops     *prometheus.Desc
	linkDowns *prometheus.Desc
	running   *prometheus.Desc
}

func newInterfaceCollector() subCollector {
	return &interfaceCollector{
		traffic: newDesc(
			"interface_traffic_bytes",
			"Interface received and transmitted bytes",
			"interface", "direction",
		),
		packets: newDesc(
			"interface_traffic_packets",
			"Interface received and transmitted packets",
			"interface", "direction",
		),
		errors: newDesc(
			"interface_errors_total",
			"Interface receive and transmit errors",
			"interface", "direction",
		),
		drops: newDesc(
			"interface_drops_total",
			"Interface receive and transmit drops",
			"interface", "direction",
		),
		linkDowns: newDesc(
			"interface_link_downs_total",
			"Number of times the interface link went down",
			"interface",
		),
		running: newDesc(
			"interface_running",
			"Whether the interface is running (1) or not (0)",
			"interface", "type", "disabled",
		),
	}
}

func (c *interfaceCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.traffic
	ch <- c.packets
	ch <- c.errors
	ch <- c.drops
	ch <- c.linkDowns
	ch <- c.running
}

func (c *interfaceCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(
		ctx,
		"/interface/print",
		"=.proplist=name,type,running,disabled,link-downs,"+
			"rx-byte,tx-byte,rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop",
	)
	if err != nil {
		return fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, re := range res.Re {
		name := re.Map["name"]
		if name == "" {
			continue
		}

		for _, dir := range []string{"rx", "tx"} {
			emit(ch, c.traffic, prometheus.CounterValue, re.Map[dir+"-byte"], parseFloat, name, dir, target.Name)
			emit(ch, c.packets, prometheus.CounterValue, re.Map[dir+"-packet"], parseFloat, name, dir, target.Name)
			emit(ch, c.errors, prometheus.CounterValue, re.Map[dir+"-error"], parseFloat, name, dir, target.Name)
			emit(ch, c.drops, prometheus.CounterValue, re.Map[dir+"-drop"], parseFloat, name, dir, target.Name)
		}

		emit(ch, c.linkDowns, prometheus.CounterValue, re.Map["link-downs"], parseFloat, name, target.Name)

		ch <- prometheus.MustNewConstMetric(
			c.running,
			prometheus.GaugeValue,
			boolToFloat(parseBool(re.Map["running"])),
			name, re.Map["type"], fmt.Sprint(parseBool(re.Map["disabled"])), target.Name,
		)
	}

	return nil
}
