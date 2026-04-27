# Recipe: scrape squad's /metrics from local Prometheus

The dashboard daemon exposes a Prometheus text exposition at `/metrics`. The
route is gated by the same auth token as `/api/*`; deployments needing
unauthenticated scrapes either bind loopback or stage a reverse-proxy exception.

In the default flow, the squad MCP server installs and supervises the
dashboard automatically on first boot. This recipe is for power users who
want a non-default bind address, a custom token, or a separately managed
process — i.e., who run `squad serve` manually instead of relying on the
auto-installed system service.

## 1. Start the dashboard

```bash
squad serve --bind 127.0.0.1 --port 7777 --token "$(cat ~/.squad/token)"
```

`--bind` takes a host or IP only — pass the port via `--port`. The token can
also be supplied via the `SQUAD_DASHBOARD_TOKEN` env var. `~/.squad/token` is
the bearer token written by the daemon installer (`squad serve --install-service`
or the auto-install flow); if you're starting `squad serve` from scratch on a
host that never ran the installer, pass any opaque string via `--token` and
mirror it into your scrape config.

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
    bearer_token_file: /home/you/.squad/token
    scrape_interval: 30s
```

`bearer_token_file` keeps the token off disk-scanned config. The daemon
installer writes `~/.squad/token` once with 0600 perms and reuses it across
restarts; rotate manually with `rm ~/.squad/token && squad serve --install-service --reinstall-service`.

## 3. Confirm the scrape works

```bash
curl -s -H "Authorization: Bearer $(cat ~/.squad/token)" \
  http://127.0.0.1:7777/metrics | promtool check metrics
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
