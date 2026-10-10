package collector

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/mikrotik"
)

const namespace = "mikrotik"

// targetLabel is appended as the last label to every metric and holds Target.Name.
const targetLabel = "name"

type Target struct {
	Name   string
	Client mikrotik.Client
	// Collectors lists enabled collector names, DefaultCollectors when empty.
	Collectors []string
}

// subCollector gathers one group of metrics from a single target.
type subCollector interface {
	describe(ch chan<- *prometheus.Desc)
	collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error
}

// collectorFactories holds every known collector in the order they are run.
var collectorFactories = []struct {
	name    string
	factory func() subCollector
}{
	{"system", newSystemCollector},
	{"health", newHealthCollector},
	{"interface", newInterfaceCollector},
	{"wireguard", newWireguardCollector},
	{"disk", newDiskCollector},
	{"container", newContainerCollector},
	{"dhcp", newDHCPCollector},
	{"wireless", newWirelessCollector},
}

// DefaultCollectors are used for targets with no explicit collectors list.
var DefaultCollectors = []string{"system", "health", "interface", "wireguard", "disk", "container", "wireless"}

// ValidateCollectors returns an error if any of the names is not a known collector.
func ValidateCollectors(names []string) error {
	for _, name := range names {
		found := false
		for _, f := range collectorFactories {
			if f.name == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown collector %q", name)
		}
	}
	return nil
}

type namedCollector struct {
	name string
	subCollector
}

type MikroTikCollector struct {
	targets    []Target
	collectors []namedCollector
	timeout    time.Duration
	lg         *zap.Logger

	up             *prometheus.Desc
	scrapeSuccess  *prometheus.Desc
	scrapeDuration *prometheus.Desc
}

func NewMikroTikCollector(targets []Target, timeout time.Duration, lg *zap.Logger) *MikroTikCollector {
	collectors := make([]namedCollector, 0, len(collectorFactories))
	for _, f := range collectorFactories {
		collectors = append(collectors, namedCollector{name: f.name, subCollector: f.factory()})
	}

	return &MikroTikCollector{
		targets:    targets,
		collectors: collectors,
		timeout:    timeout,
		lg:         lg,
		up: newDesc(
			"up",
			"Whether the device answered to API requests during the last scrape",
		),
		scrapeSuccess: newDesc(
			"scrape_collector_success",
			"Whether the collector succeeded during the last scrape",
			"collector",
		),
		scrapeDuration: newDesc(
			"scrape_collector_duration_seconds",
			"Duration of the collector during the last scrape",
			"collector",
		),
	}
}

func (c *MikroTikCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up
	ch <- c.scrapeSuccess
	ch <- c.scrapeDuration

	for _, collector := range c.collectors {
		collector.describe(ch)
	}
}

func (c *MikroTikCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	wg := sync.WaitGroup{}
	for _, target := range c.targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.collectTarget(ctx, target, ch)
		}()
	}

	wg.Wait()
}

// collectTarget runs enabled collectors one by one: the client serializes
// commands per connection anyway, so running them concurrently gains nothing.
func (c *MikroTikCollector) collectTarget(ctx context.Context, target Target, ch chan<- prometheus.Metric) {
	enabled := target.Collectors
	if len(enabled) == 0 {
		enabled = DefaultCollectors
	}

	up := false
	var connErr error
	for _, collector := range c.collectors {
		if !contains(enabled, collector.name) {
			continue
		}

		start := time.Now()
		err := connErr
		if err == nil {
			err = collector.collect(ctx, target, ch)
		}
		duration := time.Since(start).Seconds()

		success := 1.0
		switch {
		case err == nil:
			up = true
		case mikrotik.IsUnsupportedCommand(err):
			up = true
			success = 0
			c.lg.Debug(
				"collector is not supported by device",
				zap.String("target", target.Name),
				zap.String("collector", collector.name),
				zap.Error(err),
			)
		case mikrotik.IsDeviceError(err):
			up = true
			success = 0
			c.lg.Error(
				"collector failed",
				zap.String("target", target.Name),
				zap.String("collector", collector.name),
				zap.Error(err),
			)
		case connErr != nil:
			// Already logged, don't spam the same connection error per collector.
			success = 0
		default:
			// Connection level failure: the client already retried, so skip the
			// remaining collectors instead of waiting for each of them to time out.
			connErr = err
			success = 0
			c.lg.Error(
				"collector failed",
				zap.String("target", target.Name),
				zap.String("collector", collector.name),
				zap.Error(err),
			)
		}

		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, success, collector.name, target.Name)
		ch <- prometheus.MustNewConstMetric(c.scrapeDuration, prometheus.GaugeValue, duration, collector.name, target.Name)
	}

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, boolToFloat(up), target.Name)
}

// newDesc builds a metric description with the target label appended.
func newDesc(name, help string, labels ...string) *prometheus.Desc {
	return prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", name),
		help,
		append(labels, targetLabel),
		nil,
	)
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
