package collector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/mikrotik"
)

// wirelessSource is one registration table, alternatives are tried in order
// and the first one the device supports is used.
type wirelessSource struct {
	driver       string
	alternatives [][]string
}

var wirelessSources = []wirelessSource{
	// New driver (wifi-qcom, wifi-qcom-ac). Also lists clients of CAPs managed
	// by the new CAPsMAN when run on the controller. Named wifiwave2 before 7.13.
	{driver: "wifi", alternatives: [][]string{
		{"/interface/wifi/registration-table/print"},
		{"/interface/wifiwave2/registration-table/print"},
	}},
	// Legacy wireless package. Without "stats" bytes, packets and CCQ are missing.
	{driver: "wireless", alternatives: [][]string{
		{"/interface/wireless/registration-table/print", "=stats="},
	}},
	// Legacy CAPsMAN controller.
	{driver: "capsman", alternatives: [][]string{
		{"/caps-man/registration-table/print"},
	}},
}

type wirelessCollector struct {
	info         *prometheus.Desc
	signal       *prometheus.Desc
	signalNoise  *prometheus.Desc
	ccq          *prometheus.Desc
	rate         *prometheus.Desc
	traffic      *prometheus.Desc
	packets      *prometheus.Desc
	uptime       *prometheus.Desc
	lastActivity *prometheus.Desc
}

func newWirelessCollector() subCollector {
	clientLabels := []string{"interface", "mac"}
	withDirection := []string{"interface", "mac", "direction"}

	return &wirelessCollector{
		info: newDesc(
			"wireless_client_info",
			"Connected wireless client, value is always 1. Hostname and ip come from DHCP leases.",
			"interface", "mac", "driver", "ssid", "band", "hostname", "ip",
		),
		signal: newDesc(
			"wireless_client_signal_dbm",
			"Client signal strength",
			clientLabels...,
		),
		signalNoise: newDesc(
			"wireless_client_signal_to_noise_db",
			"Client signal to noise ratio (legacy wireless driver only)",
			clientLabels...,
		),
		ccq: newDesc(
			"wireless_client_ccq_percent",
			"Client transmit CCQ (legacy wireless driver only)",
			clientLabels...,
		),
		rate: newDesc(
			"wireless_client_rate_bps",
			"Current PHY rate, tx is from the access point to the client",
			withDirection...,
		),
		traffic: newDesc(
			"wireless_client_traffic_bytes",
			"Bytes since the client connected, tx is from the access point to the client",
			withDirection...,
		),
		packets: newDesc(
			"wireless_client_traffic_packets",
			"Packets since the client connected, tx is from the access point to the client",
			withDirection...,
		),
		uptime: newDesc(
			"wireless_client_uptime_seconds",
			"Time since the client connected",
			clientLabels...,
		),
		lastActivity: newDesc(
			"wireless_client_last_activity_seconds",
			"Time since the last frame from the client",
			clientLabels...,
		),
	}
}

func (c *wirelessCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.info
	ch <- c.signal
	ch <- c.signalNoise
	ch <- c.ccq
	ch <- c.rate
	ch <- c.traffic
	ch <- c.packets
	ch <- c.uptime
	ch <- c.lastActivity
}

type wirelessClient struct {
	driver string
	fields map[string]string
}

func (c *wirelessCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	clients, err := c.fetchClients(ctx, target)
	if err != nil {
		return err
	}
	if len(clients) == 0 {
		return nil
	}

	leases, err := fetchLeases(ctx, target)
	if err != nil {
		return err
	}

	// The same client may show up twice (e.g. while roaming), duplicate series
	// would fail the whole scrape, so keep the first entry only.
	seen := map[[2]string]bool{}
	for _, client := range clients {
		m := client.fields
		iface, mac := m["interface"], strings.ToUpper(m["mac-address"])
		if iface == "" || mac == "" || seen[[2]string{iface, mac}] {
			continue
		}
		seen[[2]string{iface, mac}] = true

		lease := leases[mac]
		ch <- prometheus.MustNewConstMetric(
			c.info,
			prometheus.GaugeValue,
			1,
			iface, mac, client.driver, m["ssid"], m["band"],
			firstNonEmpty(lease.hostname, m["comment"], m["radio-name"]),
			firstNonEmpty(lease.address, m["last-ip"]),
			target.Name,
		)

		signal := firstNonEmpty(m["signal"], m["signal-strength"], m["rx-signal"])
		emit(ch, c.signal, prometheus.GaugeValue, signal, parseLeadingNumber, iface, mac, target.Name)
		emit(ch, c.signalNoise, prometheus.GaugeValue, m["signal-to-noise"], parseLeadingNumber, iface, mac, target.Name)
		emit(ch, c.ccq, prometheus.GaugeValue, m["tx-ccq"], ParsePercent, iface, mac, target.Name)
		emit(ch, c.uptime, prometheus.GaugeValue, m["uptime"], ParseDuration, iface, mac, target.Name)
		emit(ch, c.lastActivity, prometheus.GaugeValue, m["last-activity"], ParseDuration, iface, mac, target.Name)

		for _, dir := range []string{"tx", "rx"} {
			emit(ch, c.rate, prometheus.GaugeValue, m[dir+"-rate"], ParseRate, iface, mac, dir, target.Name)
		}

		// Pairs are "tx,rx" from the access point point of view.
		for i, dir := range []string{"tx", "rx"} {
			emit(ch, c.traffic, prometheus.CounterValue, pairElement(m["bytes"], i), parseFloat, iface, mac, dir, target.Name)
			emit(ch, c.packets, prometheus.CounterValue, pairElement(m["packets"], i), parseFloat, iface, mac, dir, target.Name)
		}
	}

	return nil
}

// fetchClients reads every registration table the device supports. It fails
// only on real errors or when no table is supported at all.
func (c *wirelessCollector) fetchClients(ctx context.Context, target Target) ([]wirelessClient, error) {
	var (
		clients        []wirelessClient
		unsupportedErr error
		supported      bool
	)

	for _, source := range wirelessSources {
		for _, cmd := range source.alternatives {
			res, err := target.Client.RunContext(ctx, cmd...)
			if mikrotik.IsUnsupportedCommand(err) {
				unsupportedErr = err
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("failed to list %s clients: %w", source.driver, err)
			}

			supported = true
			for _, re := range res.Re {
				clients = append(clients, wirelessClient{driver: source.driver, fields: re.Map})
			}
			break
		}
	}

	if !supported {
		return nil, fmt.Errorf("no wireless registration table found: %w", unsupportedErr)
	}

	return clients, nil
}

type lease struct {
	hostname string
	address  string
}

// fetchLeases maps upper case MAC addresses to DHCP lease details.
// A device without DHCP server is not an error, the map is just empty.
func fetchLeases(ctx context.Context, target Target) (map[string]lease, error) {
	res, err := target.Client.RunContext(
		ctx,
		"/ip/dhcp-server/lease/print",
		"=.proplist=mac-address,address,host-name,comment",
	)
	if mikrotik.IsDeviceError(err) {
		return map[string]lease{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to list dhcp leases: %w", err)
	}

	leases := make(map[string]lease, len(res.Re))
	for _, re := range res.Re {
		mac := strings.ToUpper(re.Map["mac-address"])
		if mac == "" {
			continue
		}
		// Comment is set by the admin, so it is usually more meaningful.
		leases[mac] = lease{
			hostname: firstNonEmpty(re.Map["comment"], re.Map["host-name"]),
			address:  re.Map["address"],
		}
	}

	return leases, nil
}

// ParseRate converts RouterOS PHY rates like "866.7Mbps-80MHz/2S/SGI",
// "54Mbps" or "1.2Gbps" to bits per second.
func ParseRate(s string) (float64, error) {
	s = strings.TrimSpace(s)
	i := numericPrefixLen(s)
	if i == 0 {
		return 0, fmt.Errorf("no numeric value found in %q", s)
	}

	value, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, err
	}

	j := i
	for j < len(s) && isLetter(s[j]) {
		j++
	}

	switch unit := strings.ToLower(s[i:j]); unit {
	case "bps", "":
		return value, nil
	case "kbps":
		return value * 1e3, nil
	case "mbps":
		return value * 1e6, nil
	case "gbps":
		return value * 1e9, nil
	default:
		return 0, fmt.Errorf("invalid rate unit %q", unit)
	}
}

// parseLeadingNumber parses the signed number at the start of strings like
// "-62", "-62dBm@6Mbps" or "-62@HT20-7".
func parseLeadingNumber(s string) (float64, error) {
	s = strings.TrimSpace(s)
	sign := ""
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") {
		sign, s = s[:1], s[1:]
	}

	i := numericPrefixLen(s)
	if i == 0 {
		return 0, fmt.Errorf("no numeric value found in %q", s)
	}

	return strconv.ParseFloat(sign+s[:i], 64)
}

// pairElement returns the i-th element of a "tx,rx" pair, or "" if missing.
func pairElement(s string, i int) string {
	parts := strings.Split(s, ",")
	if i >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[i])
}
