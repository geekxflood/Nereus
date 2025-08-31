# Nereus Troubleshooting Guide

This guide helps diagnose and resolve common issues with the Nereus SNMP trap alerting system.

## Table of Contents

- [General Diagnostics](#general-diagnostics)
- [SNMP Listener Issues](#snmp-listener-issues)
- [Storage Problems](#storage-problems)
- [Webhook Delivery Issues](#webhook-delivery-issues)
- [Performance Problems](#performance-problems)
- [Configuration Issues](#configuration-issues)
- [Memory and Resource Issues](#memory-and-resource-issues)
- [Network Connectivity](#network-connectivity)
- [Logging and Debugging](#logging-and-debugging)

## General Diagnostics

### Check Application Status

```bash
# Check if Nereus is running
systemctl status nereus

# Check process
ps aux | grep nereus

# Check listening ports
netstat -tulpn | grep nereus
# or
ss -tulpn | grep nereus
```

### Health Check

```bash
# Application health
curl -f http://localhost:9090/health

# Readiness check
curl -f http://localhost:9090/ready

# Get detailed status
curl -s http://localhost:9090/health | jq '.'
```

### Log Analysis

```bash
# View recent logs
journalctl -u nereus -n 100

# Follow logs in real-time
journalctl -u nereus -f

# Filter by log level
journalctl -u nereus | grep ERROR

# Check for specific patterns
journalctl -u nereus | grep -E "(failed|error|timeout)"
```

## SNMP Listener Issues

### Problem: No SNMP Traps Received

**Symptoms:**
- Metrics show `nereus_traps_received_total` is not increasing
- No events in storage
- Source devices report successful trap sending

**Diagnosis:**
```bash
# Check if port 162 is bound
netstat -ulpn | grep :162

# Test UDP connectivity
nc -u -l 162  # Listen on port 162
# From another terminal or machine:
echo "test" | nc -u <nereus-ip> 162

# Check firewall rules
iptables -L -n | grep 162
ufw status | grep 162
```

**Solutions:**
1. **Port Permission Issues:**
   ```bash
   # Grant capability to bind privileged ports
   sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/nereus
   
   # Or run as root (not recommended)
   sudo systemctl edit nereus
   # Add: User=root
   ```

2. **Firewall Blocking:**
   ```bash
   # UFW
   sudo ufw allow 162/udp
   
   # iptables
   sudo iptables -A INPUT -p udp --dport 162 -j ACCEPT
   ```

3. **Binding Address Issues:**
   ```yaml
   # config.yaml
   listener:
     bind_address: "0.0.0.0:162"  # Listen on all interfaces
     # Not: "127.0.0.1:162"       # Only localhost
   ```

### Problem: Traps Received but Not Processed

**Symptoms:**
- `nereus_traps_received_total` increasing
- `nereus_traps_processed_total` not increasing or lower
- `nereus_traps_dropped_total` increasing

**Diagnosis:**
```bash
# Check processing errors in logs
journalctl -u nereus | grep -E "(parse|process|drop)"

# Check worker utilization
curl -s http://localhost:9090/api/v1/stats/listener | jq '.active_workers'
```

**Solutions:**
1. **Invalid SNMP Packets:**
   - Check source device SNMP configuration
   - Verify SNMPv2c format
   - Ensure correct community string

2. **Insufficient Workers:**
   ```yaml
   listener:
     workers: 8  # Increase based on CPU cores
     buffer_size: 5000  # Increase buffer
   ```

3. **Community String Mismatch:**
   ```yaml
   listener:
     community: "your-actual-community"
   ```

## Storage Problems

### Problem: Database Errors

**Symptoms:**
- "database is locked" errors
- Storage operations failing
- Events not persisting

**Diagnosis:**
```bash
# Check database file permissions
ls -la /var/lib/nereus/nereus.db

# Check disk space
df -h /var/lib/nereus

# Test database connectivity
sqlite3 /var/lib/nereus/nereus.db "SELECT COUNT(*) FROM events;"
```

**Solutions:**
1. **Permission Issues:**
   ```bash
   sudo chown nereus:nereus /var/lib/nereus/nereus.db
   sudo chmod 644 /var/lib/nereus/nereus.db
   ```

2. **Database Corruption:**
   ```bash
   # Stop Nereus
   sudo systemctl stop nereus
   
   # Check database integrity
   sqlite3 /var/lib/nereus/nereus.db "PRAGMA integrity_check;"
   
   # Repair if needed
   sqlite3 /var/lib/nereus/nereus.db ".recover" | sqlite3 /var/lib/nereus/nereus_recovered.db
   
   # Backup and replace
   sudo mv /var/lib/nereus/nereus.db /var/lib/nereus/nereus.db.backup
   sudo mv /var/lib/nereus/nereus_recovered.db /var/lib/nereus/nereus.db
   sudo chown nereus:nereus /var/lib/nereus/nereus.db
   
   # Start Nereus
   sudo systemctl start nereus
   ```

3. **Disk Space Issues:**
   ```bash
   # Clean old events
   sqlite3 /var/lib/nereus/nereus.db "DELETE FROM events WHERE created_at < datetime('now', '-30 days');"
   
   # Vacuum database
   sqlite3 /var/lib/nereus/nereus.db "VACUUM;"
   ```

### Problem: High Storage Usage

**Diagnosis:**
```bash
# Check database size
du -h /var/lib/nereus/nereus.db

# Count events
sqlite3 /var/lib/nereus/nereus.db "SELECT COUNT(*) FROM events;"

# Check oldest events
sqlite3 /var/lib/nereus/nereus.db "SELECT MIN(created_at) FROM events;"
```

**Solutions:**
```yaml
# Reduce retention period
storage:
  retention_days: 7  # Instead of 30

# Increase cleanup frequency
storage:
  cleanup_interval: "1h"  # Instead of "24h"
```

## Webhook Delivery Issues

### Problem: Webhooks Not Sent

**Symptoms:**
- `nereus_webhooks_sent_total` not increasing
- Events processed but no webhook notifications

**Diagnosis:**
```bash
# Check notifier status
curl -s http://localhost:9090/api/v1/stats/notifier

# Check webhook configuration
curl -s http://localhost:9090/api/v1/config | jq '.notifier'

# Test webhook endpoint manually
curl -X POST -H "Content-Type: application/json" \
  -d '{"test": "message"}' \
  https://your-webhook-endpoint.com/webhook
```

**Solutions:**
1. **Notifier Disabled:**
   ```yaml
   notifier:
     enabled: true  # Ensure enabled
   ```

2. **Invalid Endpoint Configuration:**
   ```yaml
   notifier:
     endpoints:
       - name: "webhook"
         url: "https://correct-endpoint.com/webhook"  # Check URL
         method: "POST"
         timeout: "30s"
   ```

### Problem: Webhook Timeouts

**Symptoms:**
- `nereus_webhook_errors_total` increasing
- "timeout" errors in logs

**Solutions:**
```yaml
notifier:
  endpoints:
    - name: "slow-endpoint"
      timeout: "60s"  # Increase timeout
      retry:
        max_attempts: 5
        delay: "2s"
        backoff_multiplier: 2.0
```

### Problem: Webhook Authentication Failures

**Solutions:**
```yaml
notifier:
  endpoints:
    - name: "authenticated-webhook"
      url: "https://api.example.com/webhook"
      headers:
        Authorization: "Bearer your-token"
        X-API-Key: "your-api-key"
      tls:
        cert_file: "/etc/nereus/certs/client.crt"
        key_file: "/etc/nereus/certs/client.key"
```

## Performance Problems

### Problem: High CPU Usage

**Diagnosis:**
```bash
# Check CPU usage
top -p $(pgrep nereus)

# Check goroutine count
curl -s http://localhost:9090/metrics | grep nereus_goroutines

# Profile CPU usage
go tool pprof http://localhost:9090/debug/pprof/profile
```

**Solutions:**
1. **Reduce Worker Count:**
   ```yaml
   listener:
     workers: 4  # Reduce if too high
   
   notifier:
     workers: 2  # Reduce webhook workers
   ```

2. **Optimize Correlation:**
   ```yaml
   correlator:
     window_duration: "30s"  # Reduce window
     max_groups: 1000       # Limit active groups
   ```

### Problem: High Memory Usage

**Diagnosis:**
```bash
# Check memory usage
ps aux | grep nereus

# Get memory profile
go tool pprof http://localhost:9090/debug/pprof/heap
```

**Solutions:**
```yaml
# Reduce buffer sizes
listener:
  buffer_size: 1000  # Reduce from higher values

notifier:
  queue_size: 1000   # Reduce queue size

correlator:
  max_groups: 5000   # Limit correlation groups

storage:
  batch_size: 100    # Smaller batches
  flush_interval: "1s"  # More frequent flushes
```

## Configuration Issues

### Problem: Configuration Not Loading

**Diagnosis:**
```bash
# Test configuration validation
nereus validate --config /etc/nereus/config.yaml

# Check file permissions
ls -la /etc/nereus/config.yaml

# Verify YAML syntax
yamllint /etc/nereus/config.yaml
```

**Solutions:**
1. **File Permissions:**
   ```bash
   sudo chmod 644 /etc/nereus/config.yaml
   sudo chown root:nereus /etc/nereus/config.yaml
   ```

2. **YAML Syntax Errors:**
   - Use proper indentation (spaces, not tabs)
   - Quote string values with special characters
   - Validate with online YAML validators

### Problem: Hot Reload Failures

**Diagnosis:**
```bash
# Check reload status
curl -X POST http://localhost:9090/api/v1/reload

# Check logs for reload errors
journalctl -u nereus | grep reload
```

**Solutions:**
- Validate configuration before reload
- Check component-specific reload support
- Restart service if hot reload fails

## Network Connectivity

### Problem: Cannot Reach Webhook Endpoints

**Diagnosis:**
```bash
# Test connectivity
curl -v https://your-webhook-endpoint.com/webhook

# Check DNS resolution
nslookup your-webhook-endpoint.com

# Test from Nereus server
sudo -u nereus curl -v https://your-webhook-endpoint.com/webhook
```

**Solutions:**
1. **Proxy Configuration:**
   ```bash
   # Set proxy environment variables
   export HTTP_PROXY=http://proxy.company.com:8080
   export HTTPS_PROXY=http://proxy.company.com:8080
   ```

2. **Certificate Issues:**
   ```yaml
   notifier:
     endpoints:
       - name: "webhook"
         tls:
           insecure_skip_verify: true  # Temporary for testing
   ```

## Logging and Debugging

### Enable Debug Logging

```yaml
app:
  log_level: debug

logging:
  level: debug
  format: json
```

### Useful Log Queries

```bash
# SNMP parsing errors
journalctl -u nereus | grep -i "parse.*error"

# Webhook failures
journalctl -u nereus | grep -i "webhook.*failed"

# Database errors
journalctl -u nereus | grep -i "database.*error"

# Memory issues
journalctl -u nereus | grep -i "memory\|oom"

# Performance issues
journalctl -u nereus | grep -i "slow\|timeout\|queue.*full"
```

### Performance Profiling

```bash
# CPU profile
go tool pprof http://localhost:9090/debug/pprof/profile?seconds=30

# Memory profile
go tool pprof http://localhost:9090/debug/pprof/heap

# Goroutine profile
go tool pprof http://localhost:9090/debug/pprof/goroutine
```

### Getting Help

When reporting issues, include:

1. **System Information:**
   ```bash
   nereus --version
   uname -a
   cat /etc/os-release
   ```

2. **Configuration:**
   ```bash
   # Sanitized configuration (remove sensitive data)
   nereus validate --config /etc/nereus/config.yaml
   ```

3. **Logs:**
   ```bash
   journalctl -u nereus --since "1 hour ago" > nereus.log
   ```

4. **Metrics:**
   ```bash
   curl -s http://localhost:9090/metrics > nereus-metrics.txt
   curl -s http://localhost:9090/health > nereus-health.json
   ```
