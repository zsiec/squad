# Recipe: scrape squad's /metrics from local Prometheus

`squad serve` exposes a Prometheus text exposition at `/metrics`. The route is
gated by the same auth token as `/api/*`; deployments needing unauthenticated
scrapes either bind loopback or stage a reverse-proxy exception.

## 1. Start the dashboard

```bash
squad serve --addr 127.0.0.1:7777 --token "$(cat ~/.squad/dashtoken)"
```

`squad serve` writes a fresh `~/.squad/dashtoken` on each start unless `--token`
is supplied.

## 2. Add a Prometheus scrape job

In `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: squad
    metrics_path: /metrics
    scheme: http
    static_configs:
      - targets: ["127.0.0.1:7777"]
    bearer_token_file: /home/you/.squad/dashtoken
    scrape_interval: 30s
```

`bearer_token_file` keeps the token off disk-scanned config; squad rotates the
file on every `serve` run.

## 3. Confirm the scrape works

```bash
curl -s -H "Authorization: Bearer $(cat ~/.squad/dashtoken)" \
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
