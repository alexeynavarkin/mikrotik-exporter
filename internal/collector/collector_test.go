package collector

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-routeros/routeros/v3"
	"github.com/go-routeros/routeros/v3/proto"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
)

// fakeClient replies with canned sentences keyed by command path.
type fakeClient struct {
	replies map[string][]map[string]string
	errs    map[string]error
	calls   []string
}

func (f *fakeClient) RunContext(_ context.Context, sentences ...string) (*routeros.Reply, error) {
	cmd := sentences[0]
	f.calls = append(f.calls, cmd)

	if err, ok := f.errs[cmd]; ok {
		return nil, err
	}

	reply := &routeros.Reply{}
	for _, m := range f.replies[cmd] {
		reply.Re = append(reply.Re, &proto.Sentence{Word: "!re", Map: m})
	}
	return reply, nil
}

func newTestCollector(client *fakeClient, collectors ...string) *MikroTikCollector {
	return NewMikroTikCollector(
		[]Target{{Name: "r1", Client: client, Collectors: collectors}},
		time.Second,
		zap.NewNop(),
	)
}

func assertMetrics(t *testing.T, c prometheus.Collector, expected string, names ...string) {
	t.Helper()
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), names...); err != nil {
		t.Error(err)
	}
}

func TestSystemCollector(t *testing.T) {
	client := &fakeClient{replies: map[string][]map[string]string{
		"/system/resource/print": {{
			"uptime":            "1d2h3m4s",
			"version":           "7.16 (stable)",
			"board-name":        "RB5009UG+S+",
			"architecture-name": "arm64",
			"cpu":               "ARM64",
			"platform":          "MikroTik",
			"cpu-count":         "4",
			"cpu-frequency":     "1400",
			"cpu-load":          "7",
			"free-memory":       "805306368",
			"total-memory":      "1073741824",
			"free-hdd-space":    "900M",
			"total-hdd-space":   "1G",
			"write-sect-total":  "12345",
		}},
		"/system/resource/cpu/print": {
			{"cpu": "cpu0", "load": "5"},
			{"cpu": "cpu1", "load": "9%"},
		},
	}}

	assertMetrics(t, newTestCollector(client, "system"), `
# HELP mikrotik_system_cpu_core_load_percent Per core CPU load in percent
# TYPE mikrotik_system_cpu_core_load_percent gauge
mikrotik_system_cpu_core_load_percent{cpu="cpu0",name="r1"} 5
mikrotik_system_cpu_core_load_percent{cpu="cpu1",name="r1"} 9
# HELP mikrotik_system_cpu_frequency_hertz CPU frequency
# TYPE mikrotik_system_cpu_frequency_hertz gauge
mikrotik_system_cpu_frequency_hertz{name="r1"} 1.4e+09
# HELP mikrotik_system_cpu_load_percent Total CPU load in percent
# TYPE mikrotik_system_cpu_load_percent gauge
mikrotik_system_cpu_load_percent{name="r1"} 7
# HELP mikrotik_system_hdd_free_bytes Free space on the system storage
# TYPE mikrotik_system_hdd_free_bytes gauge
mikrotik_system_hdd_free_bytes{name="r1"} 9.437184e+08
# HELP mikrotik_system_info Device information, value is always 1
# TYPE mikrotik_system_info gauge
mikrotik_system_info{architecture="arm64",board_name="RB5009UG+S+",cpu="ARM64",name="r1",platform="MikroTik",version="7.16 (stable)"} 1
# HELP mikrotik_system_memory_free_bytes Free RAM
# TYPE mikrotik_system_memory_free_bytes gauge
mikrotik_system_memory_free_bytes{name="r1"} 8.05306368e+08
# HELP mikrotik_system_uptime_seconds Time since the device boot
# TYPE mikrotik_system_uptime_seconds gauge
mikrotik_system_uptime_seconds{name="r1"} 93784
# HELP mikrotik_up Whether the device answered to API requests during the last scrape
# TYPE mikrotik_up gauge
mikrotik_up{name="r1"} 1
# HELP mikrotik_scrape_collector_success Whether the collector succeeded during the last scrape
# TYPE mikrotik_scrape_collector_success gauge
mikrotik_scrape_collector_success{collector="system",name="r1"} 1
`,
		"mikrotik_system_cpu_core_load_percent",
		"mikrotik_system_cpu_frequency_hertz",
		"mikrotik_system_cpu_load_percent",
		"mikrotik_system_hdd_free_bytes",
		"mikrotik_system_info",
		"mikrotik_system_memory_free_bytes",
		"mikrotik_system_uptime_seconds",
		"mikrotik_up",
		"mikrotik_scrape_collector_success",
	)
}

func TestHealthCollector(t *testing.T) {
	ros7 := &fakeClient{replies: map[string][]map[string]string{
		"/system/health/print": {
			{"name": "cpu-temperature", "value": "48", "type": "C"},
			{"name": "fan1-speed", "value": "3300", "type": "RPM"},
			{"name": "psu1-state", "value": "ok", "type": ""},
			{"name": "voltage", "value": "24.1", "type": "V"},
		},
	}}
	ros6 := &fakeClient{replies: map[string][]map[string]string{
		"/system/health/print": {{"voltage": "24.1", "temperature": "35", "fan-mode": "auto"}},
	}}

	assertMetrics(t, newTestCollector(ros7, "health"), `
# HELP mikrotik_health_fan_speed_rpm Fan speed sensor reading
# TYPE mikrotik_health_fan_speed_rpm gauge
mikrotik_health_fan_speed_rpm{name="r1",sensor="fan1-speed"} 3300
# HELP mikrotik_health_temperature_celsius Temperature sensor reading
# TYPE mikrotik_health_temperature_celsius gauge
mikrotik_health_temperature_celsius{name="r1",sensor="cpu-temperature"} 48
# HELP mikrotik_health_voltage_volts Voltage sensor reading
# TYPE mikrotik_health_voltage_volts gauge
mikrotik_health_voltage_volts{name="r1",sensor="voltage"} 24.1
`,
		"mikrotik_health_fan_speed_rpm",
		"mikrotik_health_temperature_celsius",
		"mikrotik_health_voltage_volts",
		"mikrotik_health_value",
	)

	assertMetrics(t, newTestCollector(ros6, "health"), `
# HELP mikrotik_health_temperature_celsius Temperature sensor reading
# TYPE mikrotik_health_temperature_celsius gauge
mikrotik_health_temperature_celsius{name="r1",sensor="temperature"} 35
# HELP mikrotik_health_voltage_volts Voltage sensor reading
# TYPE mikrotik_health_voltage_volts gauge
mikrotik_health_voltage_volts{name="r1",sensor="voltage"} 24.1
`,
		"mikrotik_health_temperature_celsius",
		"mikrotik_health_voltage_volts",
		"mikrotik_health_value",
	)
}

func TestInterfaceCollector(t *testing.T) {
	client := &fakeClient{replies: map[string][]map[string]string{
		"/interface/print": {{
			"name": "ether1", "type": "ether", "running": "true", "disabled": "false",
			"rx-byte": "100", "tx-byte": "200", "rx-error": "1", "tx-error": "0", "link-downs": "3",
		}},
	}}

	assertMetrics(t, newTestCollector(client, "interface"), `
# HELP mikrotik_interface_errors_total Interface receive and transmit errors
# TYPE mikrotik_interface_errors_total counter
mikrotik_interface_errors_total{direction="rx",interface="ether1",name="r1"} 1
mikrotik_interface_errors_total{direction="tx",interface="ether1",name="r1"} 0
# HELP mikrotik_interface_link_downs_total Number of times the interface link went down
# TYPE mikrotik_interface_link_downs_total counter
mikrotik_interface_link_downs_total{interface="ether1",name="r1"} 3
# HELP mikrotik_interface_running Whether the interface is running (1) or not (0)
# TYPE mikrotik_interface_running gauge
mikrotik_interface_running{disabled="false",interface="ether1",name="r1",type="ether"} 1
# HELP mikrotik_interface_traffic_bytes Interface received and transmitted bytes
# TYPE mikrotik_interface_traffic_bytes counter
mikrotik_interface_traffic_bytes{direction="rx",interface="ether1",name="r1"} 100
mikrotik_interface_traffic_bytes{direction="tx",interface="ether1",name="r1"} 200
`,
		"mikrotik_interface_errors_total",
		"mikrotik_interface_link_downs_total",
		"mikrotik_interface_running",
		"mikrotik_interface_traffic_bytes",
		"mikrotik_interface_traffic_packets",
	)
}

func TestWireguardCollector(t *testing.T) {
	client := &fakeClient{replies: map[string][]map[string]string{
		"/interface/wireguard/peers/print": {
			{"interface": "wg0", "name": "phone", "rx": "1.5KiB", "tx": "2048", "last-handshake": "1m5s"},
			{"interface": "wg0", "public-key": "abc=", "rx": "10", "tx": "20"},
			{"interface": "wg0"},
		},
	}}

	assertMetrics(t, newTestCollector(client, "wireguard"), `
# HELP mikrotik_wireguard_peer_last_handshake_age_seconds Seconds since the last handshake with the Wireguard peer
# TYPE mikrotik_wireguard_peer_last_handshake_age_seconds gauge
mikrotik_wireguard_peer_last_handshake_age_seconds{interface="wg0",name="r1",peer="phone"} 65
# HELP mikrotik_wireguard_peer_traffic_bytes Wireguard peer received and transmitted bytes
# TYPE mikrotik_wireguard_peer_traffic_bytes counter
mikrotik_wireguard_peer_traffic_bytes{direction="rx",interface="wg0",name="r1",peer="abc="} 10
mikrotik_wireguard_peer_traffic_bytes{direction="rx",interface="wg0",name="r1",peer="phone"} 1536
mikrotik_wireguard_peer_traffic_bytes{direction="tx",interface="wg0",name="r1",peer="abc="} 20
mikrotik_wireguard_peer_traffic_bytes{direction="tx",interface="wg0",name="r1",peer="phone"} 2048
`,
		"mikrotik_wireguard_peer_last_handshake_age_seconds",
		"mikrotik_wireguard_peer_traffic_bytes",
	)
}

func TestDiskAndContainerCollectors(t *testing.T) {
	client := &fakeClient{replies: map[string][]map[string]string{
		"/disk/print": {
			{"slot": "usb1", "type": "usb", "fs": "ext4", "model": "Flash", "size": "16008609792", "free": "15000000000"},
		},
		"/container/print": {
			{".id": "*1", "name": "pihole", "tag": "pihole/pihole:latest", "os": "linux", "arch": "arm64", "interface": "veth1", "status": "running"},
			{".id": "*2", "tag": "alpine:latest", "stopped": "true"},
		},
	}}

	assertMetrics(t, newTestCollector(client, "disk", "container"), `
# HELP mikrotik_container_running Whether the container is running (1) or not (0)
# TYPE mikrotik_container_running gauge
mikrotik_container_running{container="alpine:latest",name="r1"} 0
mikrotik_container_running{container="pihole",name="r1"} 1
# HELP mikrotik_container_status Current container status, value is always 1
# TYPE mikrotik_container_status gauge
mikrotik_container_status{container="alpine:latest",name="r1",status="stopped"} 1
mikrotik_container_status{container="pihole",name="r1",status="running"} 1
# HELP mikrotik_disk_free_bytes Free space on the disk
# TYPE mikrotik_disk_free_bytes gauge
mikrotik_disk_free_bytes{disk="usb1",name="r1"} 1.5e+10
# HELP mikrotik_disk_info Disk information, value is always 1
# TYPE mikrotik_disk_info gauge
mikrotik_disk_info{disk="usb1",fs="ext4",model="Flash",name="r1",type="usb"} 1
# HELP mikrotik_disk_size_bytes Disk size
# TYPE mikrotik_disk_size_bytes gauge
mikrotik_disk_size_bytes{disk="usb1",name="r1"} 1.6008609792e+10
`,
		"mikrotik_container_running",
		"mikrotik_container_status",
		"mikrotik_disk_free_bytes",
		"mikrotik_disk_info",
		"mikrotik_disk_size_bytes",
	)
}

func TestDHCPCollector(t *testing.T) {
	client := &fakeClient{replies: map[string][]map[string]string{
		"/ip/dhcp-server/lease/print": {
			{"server": "lan", "status": "bound"},
			{"server": "lan", "status": "bound"},
			{"server": "lan", "status": "waiting"},
		},
	}}

	assertMetrics(t, newTestCollector(client, "dhcp"), `
# HELP mikrotik_dhcp_leases Number of DHCP server leases
# TYPE mikrotik_dhcp_leases gauge
mikrotik_dhcp_leases{name="r1",server="lan",status="bound"} 2
mikrotik_dhcp_leases{name="r1",server="lan",status="waiting"} 1
`,
		"mikrotik_dhcp_leases",
	)
}

func TestUnsupportedCommandKeepsTargetUp(t *testing.T) {
	client := &fakeClient{
		replies: map[string][]map[string]string{"/disk/print": {}},
		errs: map[string]error{"/container/print": &routeros.DeviceError{Sentence: &proto.Sentence{
			Word: "!trap",
			Map:  map[string]string{"message": "no such command prefix"},
		}}},
	}

	assertMetrics(t, newTestCollector(client, "disk", "container"), `
# HELP mikrotik_scrape_collector_success Whether the collector succeeded during the last scrape
# TYPE mikrotik_scrape_collector_success gauge
mikrotik_scrape_collector_success{collector="container",name="r1"} 0
mikrotik_scrape_collector_success{collector="disk",name="r1"} 1
# HELP mikrotik_up Whether the device answered to API requests during the last scrape
# TYPE mikrotik_up gauge
mikrotik_up{name="r1"} 1
`,
		"mikrotik_scrape_collector_success",
		"mikrotik_up",
	)
}

func TestConnectionErrorSkipsRemainingCollectors(t *testing.T) {
	connErr := errors.New("failed to connect: connection refused")
	client := &fakeClient{errs: map[string]error{
		"/system/resource/print": connErr,
		"/system/health/print":   connErr,
	}}

	assertMetrics(t, newTestCollector(client, "system", "health"), `
# HELP mikrotik_up Whether the device answered to API requests during the last scrape
# TYPE mikrotik_up gauge
mikrotik_up{name="r1"} 0
`,
		"mikrotik_up",
	)

	// One call per scrape: CollectAndCompare scrapes once.
	if len(client.calls) != 1 {
		t.Errorf("expected 1 call after connection error, got %v", client.calls)
	}
}

func TestValidateCollectors(t *testing.T) {
	if err := ValidateCollectors(DefaultCollectors); err != nil {
		t.Error(err)
	}
	if err := ValidateCollectors([]string{"system", "nope"}); err == nil {
		t.Error("expected error for unknown collector")
	}
}
