# Recipe: scrape squad's /metrics from local Prometheus

The dashboard daemon exposes a Prometheus text exposition at `/metrics`.
The daemon binds loopback by default and ships no auth — anything that can
reach the port can scrape it.

In the default flow, the squad MCP server installs and supervises the
dashboard automatically on first boot. This recipe is for power users who
want a non-default bind address or a separately managed process — i.e., who
run `squad serve` manually instead of relying on the auto-installed system
service.

## 1. Start the dashboard

```bash
squad serve --bind 127.0.0.1 --port 7777
```

`--bind` takes a host or IP only — pass the port via `--port`.

If you've left auto-install on (`SQUAD_NO_AUTO_DAEMON` unset), either uninstall
the service first (`squad install-plugin --uninstall` removes it symmetrically)
or set `SQUAD_NO_AUTO_DAEMON=1` so the next MCP boot doesn't re-install over
your manual process.

## 2. Add a Prometheus scrape job

In `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: squad
    metrics_path: /metrics
    scheme: http
    static_configs:
      - targets: ["127.0.0.1:7777"]
    scrape_interval: 30s
```

## 3. Confirm the scrape works

```bash
curl -s http://127.0.0.1:7777/metrics | promtool check metrics
```

`promtool` ships with the official Prometheus binary. Clean output means the
exposition parses; any errors print line numbers.

## Metric families squad exports

| Family | Type | Labels | Notes |
|---|---|---|---|
| `squad_items_total` | gauge | repo, status | snapshot of items |
| `squad_claim_duration_seconds` | summary | repo | p50/p90/p99 over completed claims in window |
| `squad_verification_rate` | gauge | repo | fraction of dones with full evidence |
| `squad_reviewer_disagreement_rate` | gauge | repo | fraction of reviews with disagreement |
| `squad_wip_violations_attempted_total` | counter | repo | claim attempts that hit cap |
| `squad_repeat_mistake_rate` | gauge | repo | bugs in window matching an approved learning |
| `squad_attestations_total` | counter | repo, kind, status | per-kind pass/fail counts |

Default scrape window for derived metrics is 24h. Override per-deployment by
adjusting the call site in `internal/server/prometheus.go`.
