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
| system | `mikrotik_system_info`, `mikrotik_system_uptime_seconds`, `mikrotik_system_cpu_load_percent`, `mikrotik_system_cpu_core_load_percent`, `mikrotik_system_cpu_cores`, `mikrotik_system_cpu_frequency_hertz`, `mikrotik_system_memory_free_bytes`, `mikrotik_system_memory_total_bytes`, `mikrotik_system_hdd_free_bytes`, `mikrotik_system_hdd_total_bytes`, `mikrotik_system_write_sectors_total` |
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

## Development

```sh
go test ./...
```

### Testing against a real RouterOS (CHR)

`dev/chr/run-chr.sh` boots MikroTik Cloud Hosted Router (x86_64 RouterOS 7) in
QEMU in the background, waits for the API and creates demo objects: two WireGuard
interfaces peered with each other, a DHCP server with static leases and a
formatted data disk. It uses KVM when `/dev/kvm` is writable and software
emulation otherwise (2-5 minutes to boot).

```sh
sudo apt-get install -y qemu-system-x86 qemu-utils   # needs Go and curl too
dev/chr/run-chr.sh                                   # API on 127.0.0.1:8728, admin / empty password

MIKROTIK_TEST_ADDRESS=127.0.0.1:8728 go test ./internal/collector -run Integration -v
go run ./cmd --config-file=dev/chr/exporter.yml --listen-address=127.0.0.1:9199

dev/chr/stop-chr.sh
```

`TestIntegration` scrapes the device with every collector through a pedantic
registry and fails on collector errors, reconnects or duplicate series.
`go run ./dev/rosq /system/resource/print` runs ad-hoc API commands.

Options are environment variables documented at the top of `run-chr.sh`:
`CHR_VERSION`, `CHR_FRESH=1` for a factory-fresh router, `CHR_DEMO=0`, ports,
`CHR_HOME` (images and state, `~/.cache/mikrotik-exporter-chr` by default).
The image comes from download.mikrotik.com, or from MikroTik's official
`mikrotik/chr` Docker Hub image when that site is not reachable.

CHR limitations: no radios (`/interface/wifi` exists but has no clients), no
sensors in `/system/health`, no extra packages such as `container` or
`wireless`, so those collectors are only covered by unit tests.
