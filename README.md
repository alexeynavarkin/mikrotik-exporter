# mikrotik-exporter

Prometheus exporter for MikroTik RouterOS devices, uses the RouterOS API.

## Configuration

Config is read from a YAML file (`--config-file`), environment variables
(`MIKROTIK_EXPORTER_*`) and command line flags.

```yaml
listen-address: ":9100"   # default
timeout: 10s              # per scrape, keep it below Prometheus scrape_timeout
targets:
  - name: home            # value of the "name" label on every metric
    address: 192.168.88.1:8729
    username: prometheus
    password: secret
    # plaintext: true     # use the plain API service (port 8728) instead of api-ssl
    # collectors: [system, health, interface, wireguard, disk, container, wireless, dhcp]
```

A read-only user is enough:

```
/user group add name=prometheus policy=api,read,!write,!policy,!sensitive
/user add name=prometheus group=prometheus password=secret
```

## Collectors

Enabled by default: `system`, `health`, `interface`, `wireguard`, `disk`, `container`, `wireless`.
Optional: `dhcp`. Collectors for menus missing on a device (e.g. `/container`
without the container package) report `mikrotik_scrape_collector_success 0`
and are only logged at debug level; disable them via `collectors`.

| Collector | Metrics |
|-----------|---------|
| (always) | `mikrotik_up`, `mikrotik_scrape_collector_success`, `mikrotik_scrape_collector_duration_seconds` |
| system | `mikrotik_system_info`, `mikrotik_system_uptime_seconds`, `mikrotik_system_cpu_load_percent`, `mikrotik_system_cpu_core_load_percent`, `mikrotik_system_cpu_count`, `mikrotik_system_cpu_frequency_hertz`, `mikrotik_system_memory_free_bytes`, `mikrotik_system_memory_total_bytes`, `mikrotik_system_hdd_free_bytes`, `mikrotik_system_hdd_total_bytes`, `mikrotik_system_write_sectors_total` |
| health | `mikrotik_health_temperature_celsius`, `mikrotik_health_voltage_volts`, `mikrotik_health_current_amperes`, `mikrotik_health_power_watts`, `mikrotik_health_fan_speed_rpm`, `mikrotik_health_value` |
| interface | `mikrotik_interface_traffic_bytes`, `mikrotik_interface_traffic_packets`, `mikrotik_interface_errors_total`, `mikrotik_interface_drops_total`, `mikrotik_interface_link_downs_total`, `mikrotik_interface_running` |
| wireguard | `mikrotik_wireguard_peer_traffic_bytes`, `mikrotik_wireguard_peer_last_handshake_age_seconds` |
| disk | `mikrotik_disk_info`, `mikrotik_disk_size_bytes`, `mikrotik_disk_free_bytes` |
| container | `mikrotik_container_info`, `mikrotik_container_running`, `mikrotik_container_status` |
| wireless | `mikrotik_wireless_client_info`, `mikrotik_wireless_client_signal_dbm`, `mikrotik_wireless_client_signal_to_noise_db`, `mikrotik_wireless_client_ccq_percent`, `mikrotik_wireless_client_rate_bps`, `mikrotik_wireless_client_traffic_bytes`, `mikrotik_wireless_client_traffic_packets`, `mikrotik_wireless_client_uptime_seconds`, `mikrotik_wireless_client_last_activity_seconds` |
| dhcp | `mikrotik_dhcp_leases` |

## Grafana

Import [`grafana/mikrotik.json`](grafana/mikrotik.json) (Dashboards → New → Import,
Grafana 10+), then pick the Prometheus data source in the dashboard's
`Data source` variable. Rows: overview, system (CPU, RAM, storage, sensors,
disks), interfaces, WireGuard, containers, DHCP and exporter health.

### Wireless clients

The `wireless` collector exports one set of series per connected client, keyed by
`interface` and `mac`. It reads every registration table the device has:
`/interface/wifi` (new driver, `wifiwave2` before 7.13; on a CAPsMAN controller it
also lists clients of managed CAPs), `/interface/wireless` (legacy driver) and
`/caps-man` (legacy CAPsMAN). `tx`/`rx` are from the access point point of view.

`mikrotik_wireless_client_info` adds `hostname` and `ip` from DHCP leases (lease
comment wins over host-name), join it to other series when needed:

```promql
mikrotik_wireless_client_signal_dbm
  * on(name, interface, mac) group_left(hostname) mikrotik_wireless_client_info
```

Note that MAC addresses end up in Prometheus, and every new client creates new
series. Disable the collector via `collectors` if that is not acceptable.

Example queries:

```promql
# RAM usage, %
100 * (1 - mikrotik_system_memory_free_bytes / mikrotik_system_memory_total_bytes)
# Interface bandwidth, bits/s
rate(mikrotik_interface_traffic_bytes[5m]) * 8
# Stopped containers
mikrotik_container_running == 0
```
