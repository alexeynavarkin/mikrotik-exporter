package collector

import (
	"os"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/alexeynavarkin/mikrotik-exporter/internal/mikrotik"
)

// TestIntegration scrapes a real RouterOS device, e.g. the CHR started by
// dev/chr/run-chr.sh:
//
//	MIKROTIK_TEST_ADDRESS=127.0.0.1:8728 go test ./internal/collector -run Integration -v
//
// Optional: MIKROTIK_TEST_USERNAME (default admin), MIKROTIK_TEST_PASSWORD,
// MIKROTIK_TEST_TLS=1 for api-ssl.
func TestIntegration(t *testing.T) {
	address := os.Getenv("MIKROTIK_TEST_ADDRESS")
	if address == "" {
		t.Skip("MIKROTIK_TEST_ADDRESS is not set")
	}

	username := os.Getenv("MIKROTIK_TEST_USERNAME")
	if username == "" {
		username = "admin"
	}

	core, logs := observer.New(zapcore.DebugLevel)
	lg := zap.New(core)

	client := mikrotik.NewRetryClient(mikrotik.RetryClientConfig{
		Address:    address,
		Username:   username,
		Password:   os.Getenv("MIKROTIK_TEST_PASSWORD"),
		RetryCount: 2,
		PlainText:  os.Getenv("MIKROTIK_TEST_TLS") != "1",
	}, lg)

	var all []string
	for _, f := range collectorFactories {
		all = append(all, f.name)
	}

	// The pedantic registry also checks descriptions against collected
	// metrics and rejects duplicate series.
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(NewMikroTikCollector(
		[]Target{{Name: "it", Client: client, Collectors: all}},
		30*time.Second,
		lg,
	))

	// Scrape twice to make sure the connection is reused correctly.
	for i := 0; i < 2; i++ {
		families, err := reg.Gather()
		if err != nil {
			t.Fatalf("scrape %d: %v", i, err)
		}

		values := map[string]float64{}
		for _, mf := range families {
			for _, m := range mf.GetMetric() {
				labels := ""
				for _, l := range m.GetLabel() {
					if l.GetName() == "collector" {
						labels = l.GetValue()
					}
				}
				values[mf.GetName()+"/"+labels] = m.GetGauge().GetValue()
			}
		}

		if values["mikrotik_up/"] != 1 {
			t.Fatalf("scrape %d: device is not up", i)
		}
		for _, name := range []string{"system", "interface"} {
			if values["mikrotik_scrape_collector_success/"+name] != 1 {
				t.Errorf("scrape %d: collector %s failed", i, name)
			}
		}
	}

	// Collectors missing on the device are logged at debug level and are fine,
	// anything louder (failures, reconnects) is not.
	for _, entry := range logs.All() {
		switch {
		case entry.Level >= zapcore.WarnLevel:
			t.Errorf("%s: %s %v", entry.Level, entry.Message, entry.ContextMap())
		case entry.Message == "collector is not supported by device":
			t.Logf("unsupported: %v", entry.ContextMap()["collector"])
		}
	}

	problems, err := testutil.GatherAndLint(reg)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Logf("lint: %s: %s", p.Metric, p.Text)
	}
}
