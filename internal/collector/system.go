package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type systemCollector struct {
	info         *prometheus.Desc
	uptime       *prometheus.Desc
	cpuLoad      *prometheus.Desc
	cpuCount     *prometheus.Desc
	cpuFrequency *prometheus.Desc
	cpuCoreLoad  *prometheus.Desc
	memoryFree   *prometheus.Desc
	memoryTotal  *prometheus.Desc
	hddFree      *prometheus.Desc
	hddTotal     *prometheus.Desc
	writeSectors *prometheus.Desc
}

func newSystemCollector() subCollector {
	return &systemCollector{
		info: newDesc(
			"system_info",
			"Device information, value is always 1",
			"version", "board_name", "architecture", "cpu", "platform",
		),
		uptime: newDesc(
			"system_uptime_seconds",
			"Time since the device boot",
		),
		cpuLoad: newDesc(
			"system_cpu_load_percent",
			"Total CPU load in percent",
		),
		cpuCount: newDesc(
			"system_cpu_cores",
			"Number of CPU cores",
		),
		cpuFrequency: newDesc(
			"system_cpu_frequency_hertz",
			"CPU frequency",
		),
		cpuCoreLoad: newDesc(
			"system_cpu_core_load_percent",
			"Per core CPU load in percent",
			"cpu",
		),
		memoryFree: newDesc(
			"system_memory_free_bytes",
			"Free RAM",
		),
		memoryTotal: newDesc(
			"system_memory_total_bytes",
			"Total RAM",
		),
		hddFree: newDesc(
			"system_hdd_free_bytes",
			"Free space on the system storage",
		),
		hddTotal: newDesc(
			"system_hdd_total_bytes",
			"Total size of the system storage",
		),
		writeSectors: newDesc(
			"system_write_sectors_total",
			"Total sectors written to the system storage",
		),
	}
}

func (c *systemCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.uptime
	ch <- c.cpuLoad
	ch <- c.cpuCount
	ch <- c.cpuFrequency
	ch <- c.cpuCoreLoad
	ch <- c.memoryFree
	ch <- c.memoryTotal
	ch <- c.hddFree
	ch <- c.hddTotal
	ch <- c.writeSectors
}

func (c *systemCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(ctx, "/system/resource/print")
	if err != nil {
		return fmt.Errorf("failed to get system resources: %w", err)
	}

	for _, re := range res.Re {
		m := re.Map

		ch <- prometheus.MustNewConstMetric(
			c.info,
			prometheus.GaugeValue,
			1,
			m["version"], m["board-name"], m["architecture-name"], m["cpu"], m["platform"], target.Name,
		)

		emit(ch, c.uptime, prometheus.GaugeValue, m["uptime"], ParseDuration, target.Name)
		emit(ch, c.cpuLoad, prometheus.GaugeValue, m["cpu-load"], ParsePercent, target.Name)
		emit(ch, c.cpuCount, prometheus.GaugeValue, m["cpu-count"], parseFloat, target.Name)
		emit(ch, c.cpuFrequency, prometheus.GaugeValue, m["cpu-frequency"], parseMegahertz, target.Name)
		emit(ch, c.memoryFree, prometheus.GaugeValue, m["free-memory"], ParseBytes, target.Name)
		emit(ch, c.memoryTotal, prometheus.GaugeValue, m["total-memory"], ParseBytes, target.Name)
		emit(ch, c.hddFree, prometheus.GaugeValue, m["free-hdd-space"], ParseBytes, target.Name)
		emit(ch, c.hddTotal, prometheus.GaugeValue, m["total-hdd-space"], ParseBytes, target.Name)
		emit(ch, c.writeSectors, prometheus.CounterValue, m["write-sect-total"], parseFloat, target.Name)
	}

	res, err = target.Client.RunContext(ctx, "/system/resource/cpu/print", "=.proplist=cpu,load")
	if err != nil {
		return fmt.Errorf("failed to get cpu load: %w", err)
	}

	for _, re := range res.Re {
		cpu := re.Map["cpu"]
		if cpu == "" {
			continue
		}
		emit(ch, c.cpuCoreLoad, prometheus.GaugeValue, re.Map["load"], ParsePercent, cpu, target.Name)
	}

	return nil
}

func parseMegahertz(s string) (float64, error) {
	value, err := parseFloat(s)
	return value * 1e6, err
}
