package collector

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type healthCollector struct {
	temperature *prometheus.Desc
	voltage     *prometheus.Desc
	current     *prometheus.Desc
	power       *prometheus.Desc
	fanSpeed    *prometheus.Desc
	other       *prometheus.Desc
}

func newHealthCollector() subCollector {
	return &healthCollector{
		temperature: newDesc("health_temperature_celsius", "Temperature sensor reading", "sensor"),
		voltage:     newDesc("health_voltage_volts", "Voltage sensor reading", "sensor"),
		current:     newDesc("health_current_amperes", "Current sensor reading", "sensor"),
		power:       newDesc("health_power_watts", "Power consumption sensor reading", "sensor"),
		fanSpeed:    newDesc("health_fan_speed_rpm", "Fan speed sensor reading", "sensor"),
		other:       newDesc("health_value", "Other numeric health sensor reading", "sensor", "unit"),
	}
}

func (c *healthCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.temperature
	ch <- c.voltage
	ch <- c.current
	ch <- c.power
	ch <- c.fanSpeed
	ch <- c.other
}

func (c *healthCollector) collect(ctx context.Context, target Target, ch chan<- prometheus.Metric) error {
	res, err := target.Client.RunContext(ctx, "/system/health/print")
	if err != nil {
		return fmt.Errorf("failed to get health: %w", err)
	}

	for _, re := range res.Re {
		// RouterOS 7: one sentence per sensor with name, value and type (unit).
		if name, ok := re.Map["name"]; ok {
			c.emit(ch, name, re.Map["type"], re.Map["value"], target.Name)
			continue
		}

		// RouterOS 6: single sentence with sensor names as keys. RouterOS 7
		// devices without sensors (e.g. CHR) reply with state flags only.
		for key, value := range re.Map {
			if strings.HasPrefix(key, ".") || strings.HasPrefix(key, "state") {
				continue
			}
			c.emit(ch, key, unitFromSensorName(key), value, target.Name)
		}
	}

	return nil
}

func (c *healthCollector) emit(ch chan<- prometheus.Metric, sensor, unit, raw, targetName string) {
	switch strings.ToUpper(unit) {
	case "C":
		emit(ch, c.temperature, prometheus.GaugeValue, raw, parseFloat, sensor, targetName)
	case "V":
		emit(ch, c.voltage, prometheus.GaugeValue, raw, parseFloat, sensor, targetName)
	case "A":
		emit(ch, c.current, prometheus.GaugeValue, raw, parseFloat, sensor, targetName)
	case "W":
		emit(ch, c.power, prometheus.GaugeValue, raw, parseFloat, sensor, targetName)
	case "RPM":
		emit(ch, c.fanSpeed, prometheus.GaugeValue, raw, parseFloat, sensor, targetName)
	default:
		emit(ch, c.other, prometheus.GaugeValue, raw, parseFloat, sensor, unit, targetName)
	}
}

func unitFromSensorName(name string) string {
	switch {
	case strings.Contains(name, "temperature"):
		return "C"
	case strings.Contains(name, "voltage"):
		return "V"
	case strings.Contains(name, "current"):
		return "A"
	case strings.Contains(name, "power"):
		return "W"
	case strings.Contains(name, "fan") && strings.Contains(name, "speed"):
		return "RPM"
	}
	return ""
}
