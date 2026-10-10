package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type diskCollector struct {
	info *prometheus.Desc
	size *prometheus.Desc
	free *prometheus.Desc
}

func newDiskCollector() subCollector {
	return &diskCollector{
		info: newDesc("disk_info", "Disk information, value is always 1", "disk", "type", "fs", "model"),
		size: newDesc("disk_size_bytes", "Disk size", "disk"),
		free: newDesc("disk_free_bytes", "Free space on the disk", "disk"),
	}
}

func (c *diskCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.size
	ch <- c.free
}

func (c *diskCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(ctx, "/disk/print", "=.proplist=slot,name,type,fs,model,size,free")
	if err != nil {
		return fmt.Errorf("failed to list disks: %w", err)
	}

	for _, re := range res.Re {
		disk := firstNonEmpty(re.Map["slot"], re.Map["name"])
		if disk == "" {
			continue
		}

		ch <- prometheus.MustNewConstMetric(
			c.info,
			prometheus.GaugeValue,
			1,
			disk, re.Map["type"], re.Map["fs"], re.Map["model"], target.Name,
		)
		emit(ch, c.size, prometheus.GaugeValue, re.Map["size"], ParseBytes, disk, target.Name)
		emit(ch, c.free, prometheus.GaugeValue, re.Map["free"], ParseBytes, disk, target.Name)
	}

	return nil
}
