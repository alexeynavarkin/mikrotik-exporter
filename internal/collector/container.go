package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type containerCollector struct {
	info    *prometheus.Desc
	running *prometheus.Desc
	status  *prometheus.Desc
}

func newContainerCollector() subCollector {
	return &containerCollector{
		info: newDesc(
			"container_info",
			"Container information, value is always 1",
			"container", "tag", "os", "arch", "interface",
		),
		running: newDesc(
			"container_running",
			"Whether the container is running (1) or not (0)",
			"container",
		),
		status: newDesc(
			"container_status",
			"Current container status, value is always 1",
			"container", "status",
		),
	}
}

func (c *containerCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.running
	ch <- c.status
}

func (c *containerCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(ctx, "/container/print")
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	for _, re := range res.Re {
		m := re.Map

		// "name" was added in later RouterOS 7 releases, older ones only have tag.
		container := firstNonEmpty(m["name"], m["tag"], m[".id"])
		if container == "" {
			continue
		}

		status := m["status"]
		if status == "" {
			switch {
			case parseBool(m["running"]):
				status = "running"
			case parseBool(m["stopped"]):
				status = "stopped"
			default:
				status = "unknown"
			}
		}

		ch <- prometheus.MustNewConstMetric(
			c.info,
			prometheus.GaugeValue,
			1,
			container, m["tag"], m["os"], m["arch"], m["interface"], target.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.running,
			prometheus.GaugeValue,
			boolToFloat(status == "running"),
			container, target.Name,
		)
		ch <- prometheus.MustNewConstMetric(
			c.status,
			prometheus.GaugeValue,
			1,
			container, status, target.Name,
		)
	}

	return nil
}
