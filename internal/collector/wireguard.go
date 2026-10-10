package collector

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

type wireguardCollector struct {
	traffic       *prometheus.Desc
	lastHandshake *prometheus.Desc
}

func newWireguardCollector() subCollector {
	return &wireguardCollector{
		traffic: newDesc(
			"wireguard_peer_traffic_bytes",
			"Wireguard peer received and transmitted bytes",
			"interface", "peer", "direction",
		),
		lastHandshake: newDesc(
			"wireguard_peer_last_handshake_age_seconds",
			"Seconds since the last handshake with the Wireguard peer",
			"interface", "peer",
		),
	}
}

func (c *wireguardCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.traffic
	ch <- c.lastHandshake
}

func (c *wireguardCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(
		ctx,
		"/interface/wireguard/peers/print",
		"=.proplist=interface,name,public-key,rx,tx,last-handshake",
	)
	if err != nil {
		return fmt.Errorf("failed to list wireguard peers: %w", err)
	}

	for _, re := range res.Re {
		iface := re.Map["interface"]
		if iface == "" {
			continue
		}

		// "name" exists only since RouterOS 7.15, fall back to the public key
		// (not comment: it is not unique and duplicate series break the scrape).
		peer := firstNonEmpty(re.Map["name"], re.Map["public-key"])
		if peer == "" {
			continue
		}

		emit(ch, c.traffic, prometheus.CounterValue, re.Map["rx"], ParseBytes, iface, peer, "rx", target.Name)
		emit(ch, c.traffic, prometheus.CounterValue, re.Map["tx"], ParseBytes, iface, peer, "tx", target.Name)
		emit(ch, c.lastHandshake, prometheus.GaugeValue, re.Map["last-handshake"], ParseDuration, iface, peer, target.Name)
	}

	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
