# OCR Checker Monitoring Guide

## Overview

The OCR Checker provides comprehensive monitoring capabilities for Chainlink OCR2 transmitters with Slack alerts and Prometheus metrics.

## Features

- **Real-time Monitoring**: Check transmitter activity across all associated OCR2 jobs
- **Slack Alerts**: Automated notifications when issues are detected
- **Prometheus Metrics**: Export metrics for integration with monitoring systems
- **Flexible Scheduling**: Run one-time checks or continuous monitoring with cron
- **Backward Compatible**: Works with existing shell scripts

## Commands

### Alert Command

The `alert` command provides one-time monitoring with optional Slack notifications:

```bash
# Basic usage (compatible with existing scripts)
ocr-checker alert <transmitter> <rounds_to_check> <blocks_to_check>

# With environment variable for Slack webhook
export SLACK_WEB_HOOK=https://hooks.slack.com/services/...
ocr-checker alert 0x2ddbfc05d324d4f424e451aa66069b995836333d 100 10000

# With command-line options
ocr-checker alert 0x2ddbfc05d324d4f424e451aa66069b995836333d 100 10000 \
  --webhook https://hooks.slack.com/services/... \
  --channel "#alerts" \
  --mention U05QL6E85QE \
  --stale-threshold 24h \
  --save result.json
```

#### Options

- `--webhook`: Slack webhook URL (overrides SLACK_WEB_HOOK env)
- `--channel`: Slack channel to post to
- `--mention`: Users to mention in alerts (can be repeated)
- `--stale-threshold`: Duration to consider job stale (default: 24h)
- `--save`: Save result to JSON file
- `--output/-o`: Output format (json, yaml, text)
- `--dry-run`: Check without sending alerts

### Monitor Command

The `monitor` command runs continuous monitoring with Prometheus metrics:

```bash
# Run monitoring server
ocr-checker monitor \
  --transmitters 0x2ddbfc05d324d4f424e451aa66069b995836333d \
  --transmitters 0x1234567890abcdef1234567890abcdef12345678 \
  --interval "@every 5m" \
  --port 8080 \
  --webhook $SLACK_WEB_HOOK
```

#### Options

- `--transmitters`: Transmitter addresses to monitor (can be repeated)
- `--interval`: Check interval in cron format (default: @every 5m)
- `--port/-p`: Metrics server port (default: 8080)
- `--webhook`: Slack webhook URL for alerts
- `--channel`: Slack channel
- `--mention`: Users to mention
- `--stale-threshold`: Duration to consider job stale
- `--metrics-path`: Path to expose metrics (default: /metrics)

#### Cron Format Examples

- `@every 5m`: Every 5 minutes
- `@every 1h`: Every hour
- `0 */6 * * *`: Every 6 hours
- `0 9 * * *`: Daily at 9 AM
- `0 0 * * MON`: Weekly on Monday

### Integration with Existing Scripts

The alert command is designed to be compatible with existing shell scripts:

```bash
#!/bin/bash
# Your existing script
export SLACK_WEB_HOOK=https://hooks.slack.com/services/...
export MSG="<@U05QL6E85QE> Matic"

# Old way (still works)
./ocr-checker watch $TRANSMITTER $LAST_ROUND $LAST_BLOCK -o json | jq > result.json
./check_report_to_slack.sh result.json "$MSG"

# New way (integrated)
./ocr-checker alert $TRANSMITTER $LAST_ROUND $LAST_BLOCK \
  --save result.json \
  --mention U05QL6E85QE
```

## Prometheus Metrics

The monitor command exposes the following metrics:

### Job Metrics
- `ocr_checker_jobs_total`: Total number of OCR jobs
- `ocr_checker_jobs_healthy`: Number of healthy jobs
- `ocr_checker_jobs_stale`: Number of stale jobs
- `ocr_checker_jobs_missing`: Number of missing jobs
- `ocr_checker_jobs_error`: Number of jobs with errors
- `ocr_checker_jobs_no_active`: Number of inactive jobs

### Health Metrics
- `ocr_checker_health_score`: Overall health score (0-1)
- `ocr_checker_last_check_timestamp`: Timestamp of last check
- `ocr_checker_check_duration_seconds`: Duration of monitoring checks
- `ocr_checker_check_errors_total`: Total number of check errors

### Transmission Metrics
- `ocr_checker_last_round_number`: Last round number for each job
- `ocr_checker_time_since_last_tx_seconds`: Time since last transmission

### Alert Metrics
- `ocr_checker_alerts_sent_total`: Total alerts sent
- `ocr_checker_alerts_failed_total`: Total failed alerts

All metrics include labels: `transmitter`, `chain`, `chain_id`, and job-specific metrics also include `job_id` and `contract`.

## Configuration

### Config File

Add alert configuration to your `config.toml`:

```toml
[alert]
enabled = true
webhook_url = "https://hooks.slack.com/services/..."
channel = "#ocr-alerts"
mention_users = ["U05QL6E85QE", "U12345678"]
stale_threshold = "24h"
alert_on_stale = true
alert_on_missing = true
alert_on_error = true
```

### Environment Variables

- `SLACK_WEB_HOOK`: Default Slack webhook URL
- Standard OCR checker environment variables

## Alert Format

Alerts are sent to Slack with the following information:

- **Status**: Overall health status (healthy/warning/critical)
- **Transmitter**: Address being monitored
- **Chain**: Network name and ID
- **Health Score**: Percentage of healthy jobs
- **Job Summary**: Count of jobs by status
- **Job Details**: Specific issues for each problematic job

Example alert:
```
🔴 Chainlink OCR Monitor Alert Polygon
<@U05QL6E85QE>

Status: critical
Transmitter: 0x2ddbfc05d324d4f424e451aa66069b995836333d
Total Jobs: 5
Health Score: 60.0%

🟢 Found: 3
🟡 Stale: 1
🔴 Missing: 1

Job Details:
🟡 job-123: stale - Last: 26h ago
🔴 job-456: missing
```

## Monitoring Best Practices

1. **Set Appropriate Thresholds**: Adjust stale threshold based on expected transmission frequency
2. **Monitor Multiple Transmitters**: Use the monitor command to track all your transmitters
3. **Export Metrics**: Integrate with Grafana for visualization
4. **Test Alerts**: Use --dry-run to verify alert configuration
5. **Save Results**: Use --save for audit trails and debugging

## Grafana Dashboard

Import the example dashboard from `docs/grafana-dashboard.json` for visualization.

## Troubleshooting

### No Alerts Received
- Check webhook URL is correct
- Verify SLACK_WEB_HOOK environment variable
- Use --dry-run to test configuration
- Check logs for error messages

### Metrics Not Updating
- Verify monitor is running (check /health endpoint)
- Check transmitter addresses are correct
- Ensure database connectivity
- Review logs for errors

### High False Positives
- Increase stale threshold
- Adjust alert conditions in config
- Consider chain-specific thresholds