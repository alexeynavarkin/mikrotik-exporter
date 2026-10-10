package collector

import (
	"testing"

	"github.com/go-routeros/routeros/v3"
	"github.com/go-routeros/routeros/v3/proto"
)

func unsupported() error {
	return &routeros.DeviceError{Sentence: &proto.Sentence{
		Word: "!trap",
		Map:  map[string]string{"message": "no such command prefix"},
	}}
}

func TestWirelessCollectorBothDrivers(t *testing.T) {
	client := &fakeClient{
		replies: map[string][]map[string]string{
			"/interface/wifi/registration-table/print": {{
				"interface": "wifi1", "ssid": "home", "band": "5ghz-ax", "mac-address": "aa:bb:cc:00:00:01",
				"signal": "-55", "tx-rate": "1200.9Mbps-80MHz/2S/MCS11", "rx-rate": "864.8Mbps-80MHz/2S/MCS9",
				"bytes": "1000,2000", "packets": "10,20", "uptime": "1h2m", "last-activity": "30ms",
			}},
			"/interface/wireless/registration-table/print": {
				{
					"interface": "wlan1", "mac-address": "AA:BB:CC:00:00:02", "radio-name": "printer",
					"signal-strength": "-71dBm@6Mbps", "signal-to-noise": "35", "tx-ccq": "84",
					"tx-rate": "65Mbps-20MHz/1S", "rx-rate": "54Mbps", "bytes": "300,400", "packets": "3,4",
					"uptime": "2d", "last-ip": "10.0.0.9",
				},
				// Duplicate entry must not break the scrape.
				{"interface": "wlan1", "mac-address": "AA:BB:CC:00:00:02", "signal-strength": "-80"},
			},
			"/ip/dhcp-server/lease/print": {
				{"mac-address": "AA:BB:CC:00:00:01", "address": "10.0.0.5", "host-name": "phone", "comment": ""},
			},
		},
		errs: map[string]error{"/caps-man/registration-table/print": unsupported()},
	}

	assertMetrics(t, newTestCollector(client, "wireless"), `
# HELP mikrotik_wireless_client_ccq_percent Client transmit CCQ (legacy wireless driver only)
# TYPE mikrotik_wireless_client_ccq_percent gauge
mikrotik_wireless_client_ccq_percent{interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 84
# HELP mikrotik_wireless_client_info Connected wireless client, value is always 1. Hostname and ip come from DHCP leases.
# TYPE mikrotik_wireless_client_info gauge
mikrotik_wireless_client_info{band="5ghz-ax",driver="wifi",hostname="phone",interface="wifi1",ip="10.0.0.5",mac="AA:BB:CC:00:00:01",name="r1",ssid="home"} 1
mikrotik_wireless_client_info{band="",driver="wireless",hostname="printer",interface="wlan1",ip="10.0.0.9",mac="AA:BB:CC:00:00:02",name="r1",ssid=""} 1
# HELP mikrotik_wireless_client_last_activity_seconds Time since the last frame from the client
# TYPE mikrotik_wireless_client_last_activity_seconds gauge
mikrotik_wireless_client_last_activity_seconds{interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 0.03
# HELP mikrotik_wireless_client_rate_bps Current PHY rate, tx is from the access point to the client
# TYPE mikrotik_wireless_client_rate_bps gauge
mikrotik_wireless_client_rate_bps{direction="rx",interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 8.648e+08
mikrotik_wireless_client_rate_bps{direction="rx",interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 5.4e+07
mikrotik_wireless_client_rate_bps{direction="tx",interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 1.2009e+09
mikrotik_wireless_client_rate_bps{direction="tx",interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 6.5e+07
# HELP mikrotik_wireless_client_signal_dbm Client signal strength
# TYPE mikrotik_wireless_client_signal_dbm gauge
mikrotik_wireless_client_signal_dbm{interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} -55
mikrotik_wireless_client_signal_dbm{interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} -71
# HELP mikrotik_wireless_client_signal_to_noise_db Client signal to noise ratio (legacy wireless driver only)
# TYPE mikrotik_wireless_client_signal_to_noise_db gauge
mikrotik_wireless_client_signal_to_noise_db{interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 35
# HELP mikrotik_wireless_client_traffic_bytes Bytes since the client connected, tx is from the access point to the client
# TYPE mikrotik_wireless_client_traffic_bytes counter
mikrotik_wireless_client_traffic_bytes{direction="rx",interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 2000
mikrotik_wireless_client_traffic_bytes{direction="rx",interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 400
mikrotik_wireless_client_traffic_bytes{direction="tx",interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 1000
mikrotik_wireless_client_traffic_bytes{direction="tx",interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 300
# HELP mikrotik_wireless_client_uptime_seconds Time since the client connected
# TYPE mikrotik_wireless_client_uptime_seconds gauge
mikrotik_wireless_client_uptime_seconds{interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} 3720
mikrotik_wireless_client_uptime_seconds{interface="wlan1",mac="AA:BB:CC:00:00:02",name="r1"} 172800
# HELP mikrotik_scrape_collector_success Whether the collector succeeded during the last scrape
# TYPE mikrotik_scrape_collector_success gauge
mikrotik_scrape_collector_success{collector="wireless",name="r1"} 1
`,
		"mikrotik_wireless_client_ccq_percent",
		"mikrotik_wireless_client_info",
		"mikrotik_wireless_client_last_activity_seconds",
		"mikrotik_wireless_client_rate_bps",
		"mikrotik_wireless_client_signal_dbm",
		"mikrotik_wireless_client_signal_to_noise_db",
		"mikrotik_wireless_client_traffic_bytes",
		"mikrotik_wireless_client_uptime_seconds",
		"mikrotik_scrape_collector_success",
	)
}

func TestWirelessCollectorFallsBackToWifiwave2(t *testing.T) {
	client := &fakeClient{
		replies: map[string][]map[string]string{
			"/interface/wifiwave2/registration-table/print": {{"interface": "wifi1", "mac-address": "AA:BB:CC:00:00:01", "signal": "-60"}},
		},
		errs: map[string]error{
			"/interface/wifi/registration-table/print":     unsupported(),
			"/interface/wireless/registration-table/print": unsupported(),
			"/caps-man/registration-table/print":           unsupported(),
		},
	}

	assertMetrics(t, newTestCollector(client, "wireless"), `
# HELP mikrotik_wireless_client_signal_dbm Client signal strength
# TYPE mikrotik_wireless_client_signal_dbm gauge
mikrotik_wireless_client_signal_dbm{interface="wifi1",mac="AA:BB:CC:00:00:01",name="r1"} -60
`,
		"mikrotik_wireless_client_signal_dbm",
	)
}

func TestWirelessCollectorWithoutWireless(t *testing.T) {
	client := &fakeClient{errs: map[string]error{
		"/interface/wifi/registration-table/print":      unsupported(),
		"/interface/wifiwave2/registration-table/print": unsupported(),
		"/interface/wireless/registration-table/print":  unsupported(),
		"/caps-man/registration-table/print":            unsupported(),
	}}

	assertMetrics(t, newTestCollector(client, "wireless"), `
# HELP mikrotik_scrape_collector_success Whether the collector succeeded during the last scrape
# TYPE mikrotik_scrape_collector_success gauge
mikrotik_scrape_collector_success{collector="wireless",name="r1"} 0
# HELP mikrotik_up Whether the device answered to API requests during the last scrape
# TYPE mikrotik_up gauge
mikrotik_up{name="r1"} 1
`,
		"mikrotik_scrape_collector_success",
		"mikrotik_up",
	)
}

func TestParseRate(t *testing.T) {
	cases := map[string]float64{
		"866.7Mbps-80MHz/2S/SGI": 866.7e6,
		"54Mbps":                 54e6,
		"1.2Gbps":                1.2e9,
		"6Mbps-20MHz/1S":         6e6,
		"500kbps":                500e3,
	}
	for in, want := range cases {
		got, err := ParseRate(in)
		if err != nil || got < want-1 || got > want+1 {
			t.Errorf("ParseRate(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseRate("fast"); err == nil {
		t.Error("expected error")
	}
}

func TestParseLeadingNumber(t *testing.T) {
	for in, want := range map[string]float64{"-62": -62, "-62dBm@6Mbps": -62, "-62@HT20-7": -62, "35": 35} {
		got, err := parseLeadingNumber(in)
		if err != nil || got != want {
			t.Errorf("parseLeadingNumber(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}
