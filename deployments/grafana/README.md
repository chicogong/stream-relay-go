# Grafana Monitoring Dashboard

Complete monitoring setup for Stream Relay Go using Prometheus + Grafana.

## Quick Start

### 1. Configure Prometheus Target

Edit `prometheus.yml` and replace `host.docker.internal` with your actual host IP:

```yaml
scrape_configs:
  - job_name: 'relay'
    static_configs:
      - targets: ['YOUR_HOST_IP:8080']  # e.g., 192.168.1.100:8080
```

### 2. Start Monitoring Stack

```bash
cd deployments/grafana
docker-compose up -d
```

### 3. Access Grafana

Open http://localhost:3000

**Default Credentials:**
- Username: `admin`
- Password: `admin`

### 4. Import Dashboard

The beautiful dashboard JSON is available in `beautiful-dashboard.json`.

**Option A: Auto-import (if provisioning works)**
Dashboard should appear automatically after startup.

**Option B: Manual import**
1. Go to Dashboards → Import
2. Upload `beautiful-dashboard.json`
3. Select "Prometheus" datasource
4. Click Import

## Dashboard Panels

### Key Metrics

1. **Request Rate** - Requests per second by route and status
2. **Error Rate** - Failed requests per second with alerting
3. **Request Duration** - p50, p95, p99 latency percentiles
4. **Total Requests** - Cumulative request count by route
5. **Request Duration Heatmap** - Visual latency distribution
6. **Active Connections** - Current open connections
7. **Storage Write Latency** - Database write performance
8. **Success Rate** - Percentage of successful requests (gauge)
9. **Avg Response Time** - Mean response time
10. **Total Requests (5m)** - Recent request volume

### Alerts

- **High Error Rate**: Triggers when error rate > 0.1 req/s for 5 minutes

## Architecture

```
Relay (localhost:8080/metrics)
    ↓
Prometheus (scrape every 5s)
    ↓
Grafana (visualization)
```

## Prometheus

- **URL**: http://localhost:9090
- **Scrape interval**: 5s
- **Targets**: http://host.docker.internal:8080/metrics

### Sample Queries

```promql
# Request rate
rate(relay_requests_total[1m])

# P95 latency
histogram_quantile(0.95, rate(relay_duration_ms_bucket[1m]))

# Success rate
sum(rate(relay_requests_total{status="2xx"}[5m])) / sum(rate(relay_requests_total[5m])) * 100
```

## Customization

### Edit Dashboard

1. Open Grafana
2. Navigate to dashboard
3. Click "Dashboard settings" (gear icon)
4. Modify panels as needed
5. Save

Changes are persisted in the `grafana_data` volume.

### Add Custom Panels

Click "Add panel" and use these metrics:

- `relay_requests_total` - Total requests counter
- `relay_duration_ms` - Request duration histogram
- `relay_errors_total` - Error counter
- `relay_active_connections` - Active connections gauge
- `relay_storage_write_ms` - Storage write latency histogram

## Troubleshooting

### Prometheus Can't Scrape Relay

Check that relay is running on localhost:8080:
```bash
curl http://localhost:8080/metrics
```

### Dashboard Not Showing

1. Verify Prometheus is scraping:
   ```bash
   curl http://localhost:9090/api/v1/targets
   ```

2. Check datasource connection in Grafana:
   - Settings → Data Sources → Prometheus → Test

### No Data in Grafana

1. Generate some traffic:
   ```bash
   curl -N http://localhost:8080/v1/chat/completions \
     -H 'Authorization: Bearer sk-relay-test-key-123' \
     -H 'Content-Type: application/json' \
     -d '{"model": "Qwen/Qwen2.5-7B-Instruct", "messages": [{"role": "user", "content": "Hi"}], "stream": true, "max_tokens": 10}'
   ```

2. Wait 15-30 seconds for Prometheus to scrape

## Cleanup

```bash
docker-compose down
docker volume rm grafana_grafana_data grafana_prometheus_data
```

## Production Tips

1. **Secure Grafana**: Change admin password immediately
2. **Persistent Storage**: Volumes are already configured
3. **Retention**: Adjust Prometheus retention in docker-compose.yml
4. **Scaling**: For production, use dedicated Prometheus/Grafana instances
5. **Alerts**: Configure alert notifications (Slack, PagerDuty, etc.)

## Dashboard JSON

The dashboard configuration is in `dashboard.json` and can be:
- Imported manually via Grafana UI
- Auto-provisioned (current setup)
- Version controlled and shared

## Metrics Reference

See [../../docs/METRICS.md](../../docs/METRICS.md) for complete metrics documentation.

## Enhanced Dashboard with Logs

We now provide an **enhanced dashboard** (`enhanced-dashboard.json`) with additional features:

### New Panels (Total: 15 panels)

1. **📊 Total Requests** - Instant counter
2. **✅ Success Rate** - Color-coded gauge (0-100%)
3. **⏱️ Avg Response Time** - Mean latency with thresholds
4. **🔗 Active Connections** - Real-time connection count
5. **🚨 Error Count** - Total errors with color thresholds
6. **💾 Storage Write Latency** - Database performance
7. **📈 Request Rate Trend** - Time series graph
8. **⏱️ Response Time Percentiles** - p50/p95/p99 lines
9. **🎯 Requests by Route** - Donut chart distribution
10. **📊 Status Code Distribution** - Bar gauge (2xx/4xx/5xx)
11. **🚨 Error Types** - Table breakdown by route and type
12. **🔗 Active Connections Over Time** - Stacked area chart
13. **🔥 Request Heatmap** - Latency distribution visualization
14. **📋 Recent Activity Log** - Metrics summary table
15. **📝 Application Logs** - Live log streaming (Loki)

### Log Collection (Loki + Promtail)

The enhanced setup includes log aggregation:

#### Services

- **Loki** (`:3100`) - Log storage and query engine
- **Promtail** - Log collector and shipper
- **Grafana** - Unified metrics + logs view

#### Log Sources

Promtail collects logs from:
1. **Application logs**: `../../logs/*.log`
2. **System logs**: `/var/log/*.log` (optional)

#### Viewing Logs in Grafana

1. Navigate to **Explore** tab
2. Select **Loki** datasource
3. Use LogQL queries:
   ```logql
   {job="relay-logs"}
   {job="relay-logs"} |= "error"
   {job="relay-logs"} | json | level="ERROR"
   ```

#### Log Retention

- Default: 7 days (168 hours)
- Configure in `loki-config.yml`:
  ```yaml
  limits_config:
    retention_period: 168h  # Adjust as needed
  ```

### Setting Up Logs

1. **Create logs directory** (if not exists):
   ```bash
   mkdir -p ../../logs
   ```

2. **Configure relay to write logs**:
   Update your relay to write JSON-formatted logs:
   ```go
   log.SetOutput(logFile)
   log.SetFormatter(&log.JSONFormatter{})
   ```

3. **Restart monitoring stack**:
   ```bash
   docker-compose down
   docker-compose up -d
   ```

4. **Verify log collection**:
   ```bash
   # Check Promtail is scraping
   docker logs relay-promtail

   # Query Loki directly
   curl -G -s "http://localhost:3100/loki/api/v1/query" \
     --data-urlencode 'query={job="relay-logs"}' | jq
   ```

### Dashboard Features

#### Color Coding

- **Green**: Healthy (success rate >99%, latency <500ms)
- **Yellow**: Warning (success rate 95-99%, latency 500-1000ms)
- **Orange**: Degraded (success rate 90-95%, latency 1-3s)
- **Red**: Critical (success rate <90%, latency >3s)

#### Auto-Refresh

- Default: Every 5 seconds
- Configurable in dashboard settings

### Architecture with Logs

```
Relay App
  ├─ Metrics → Prometheus → Grafana
  └─ Logs → Promtail → Loki → Grafana
```

### Comparison: Basic vs Enhanced

| Feature | Basic Dashboard | Enhanced Dashboard |
|---------|----------------|-------------------|
| Panels | 8 | 15 |
| Logs | ❌ | ✅ (Loki) |
| Route breakdown | ❌ | ✅ |
| Error analysis | Basic | Detailed table |
| Heatmaps | Basic | Enhanced |
| Active connections | Counter | Time series |
| Storage metrics | ❌ | ✅ |

### Performance Impact

- **Loki**: Minimal CPU/memory footprint
- **Promtail**: <50MB RAM
- **Storage**: ~100MB/day for moderate traffic
- **Retention**: Auto-cleanup after 7 days
