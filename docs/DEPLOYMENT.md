# Nereus Deployment Guide

This guide covers various deployment scenarios for the Nereus SNMP trap alerting system, from simple standalone installations to production-ready containerized deployments.

## Table of Contents

- [System Requirements](#system-requirements)
- [Installation Methods](#installation-methods)
- [Configuration](#configuration)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Systemd Service](#systemd-service)
- [Security Considerations](#security-considerations)
- [Monitoring and Observability](#monitoring-and-observability)
- [Troubleshooting](#troubleshooting)

## System Requirements

### Minimum Requirements
- **OS**: Linux (Ubuntu 20.04+, CentOS 8+, RHEL 8+), macOS 10.15+, Windows 10+
- **CPU**: 1 core, 2.0 GHz
- **Memory**: 512 MB RAM
- **Storage**: 1 GB available disk space
- **Network**: UDP port 162 (SNMP traps), HTTP port for webhooks

### Recommended Requirements
- **OS**: Linux (Ubuntu 22.04 LTS, Rocky Linux 9)
- **CPU**: 2+ cores, 2.4 GHz
- **Memory**: 2 GB RAM
- **Storage**: 10 GB available disk space (for logs and database)
- **Network**: Dedicated network interface for SNMP traffic

### Production Requirements
- **CPU**: 4+ cores, 3.0 GHz
- **Memory**: 8 GB RAM
- **Storage**: 50 GB SSD (with log rotation)
- **Network**: High-speed network interface, load balancer support

## Installation Methods

### Binary Installation

1. **Download the latest release:**
   ```bash
   wget https://github.com/geekxflood/nereus/releases/latest/download/nereus-linux-amd64.tar.gz
   tar -xzf nereus-linux-amd64.tar.gz
   sudo mv nereus /usr/local/bin/
   sudo chmod +x /usr/local/bin/nereus
   ```

2. **Verify installation:**
   ```bash
   nereus --version
   ```

### Build from Source

1. **Prerequisites:**
   ```bash
   # Install Go 1.21+
   wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin
   ```

2. **Clone and build:**
   ```bash
   git clone https://github.com/geekxflood/nereus.git
   cd nereus
   make build
   sudo cp build/nereus /usr/local/bin/
   ```

### Package Installation

#### Ubuntu/Debian
```bash
# Add repository
curl -fsSL https://packages.geekxflood.com/gpg | sudo apt-key add -
echo "deb https://packages.geekxflood.com/apt stable main" | sudo tee /etc/apt/sources.list.d/geekxflood.list

# Install
sudo apt update
sudo apt install nereus
```

#### RHEL/CentOS/Rocky Linux
```bash
# Add repository
sudo tee /etc/yum.repos.d/geekxflood.repo << EOF
[geekxflood]
name=GeekxFlood Repository
baseurl=https://packages.geekxflood.com/rpm
enabled=1
gpgcheck=1
gpgkey=https://packages.geekxflood.com/gpg
EOF

# Install
sudo dnf install nereus
```

## Configuration

### Basic Configuration

1. **Generate sample configuration:**
   ```bash
   nereus generate --output /etc/nereus/config.yaml
   ```

2. **Edit configuration:**
   ```bash
   sudo nano /etc/nereus/config.yaml
   ```

3. **Validate configuration:**
   ```bash
   nereus validate --config /etc/nereus/config.yaml
   ```

### Configuration Locations

The application searches for configuration files in the following order:
1. `--config` flag value
2. `NEREUS_CONFIG` environment variable
3. `./config.yaml` (current directory)
4. `$HOME/.nereus/config.yaml`
5. `/etc/nereus/config.yaml`

### Environment Variables

Key environment variables for deployment:

```bash
# Configuration
export NEREUS_CONFIG="/etc/nereus/config.yaml"
export NEREUS_LOG_LEVEL="info"

# Database
export NEREUS_DB_PATH="/var/lib/nereus/nereus.db"

# Network
export NEREUS_BIND_ADDRESS="0.0.0.0:162"
export NEREUS_METRICS_PORT="9090"

# Security
export NEREUS_COMMUNITY="your-snmp-community"
```

## Docker Deployment

### Basic Docker Run

```bash
# Create data directory
sudo mkdir -p /var/lib/nereus

# Run container
docker run -d \
  --name nereus \
  --restart unless-stopped \
  -p 162:162/udp \
  -p 9090:9090 \
  -v /etc/nereus:/etc/nereus:ro \
  -v /var/lib/nereus:/var/lib/nereus \
  geekxflood/nereus:latest
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  nereus:
    image: geekxflood/nereus:latest
    container_name: nereus
    restart: unless-stopped
    ports:
      - "162:162/udp"
      - "9090:9090"
    volumes:
      - ./config:/etc/nereus:ro
      - nereus_data:/var/lib/nereus
    environment:
      - NEREUS_LOG_LEVEL=info
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:9090/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  nereus_data:
```

Deploy:
```bash
docker-compose up -d
```

### Multi-Stage Dockerfile

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.version=${VERSION} -X main.buildTime=$(date +%Y%m%d%H%M%S)" \
    -o nereus ./main.go

# Runtime stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates tzdata
RUN adduser -D -s /bin/sh nereus

WORKDIR /app

COPY --from=builder /app/nereus .
COPY --chown=nereus:nereus examples/config.yaml /etc/nereus/config.yaml

USER nereus

EXPOSE 162/udp 9090

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:9090/health || exit 1

CMD ["./nereus", "--config", "/etc/nereus/config.yaml"]
```

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: nereus
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nereus-config
  namespace: nereus
data:
  config.yaml: |
    app:
      log_level: info
    
    listener:
      bind_address: "0.0.0.0:162"
      community: "public"
      workers: 4
      buffer_size: 1000
    
    storage:
      type: "sqlite"
      connection_string: "/var/lib/nereus/nereus.db"
      retention_days: 30
    
    notifier:
      enabled: true
      workers: 2
      endpoints:
        - name: "alertmanager"
          url: "http://alertmanager:9093/api/v1/alerts"
          method: "POST"
    
    metrics:
      enabled: true
      bind_address: "0.0.0.0:9090"
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nereus
  namespace: nereus
  labels:
    app: nereus
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nereus
  template:
    metadata:
      labels:
        app: nereus
    spec:
      containers:
      - name: nereus
        image: geekxflood/nereus:latest
        ports:
        - containerPort: 162
          protocol: UDP
          name: snmp
        - containerPort: 9090
          protocol: TCP
          name: metrics
        volumeMounts:
        - name: config
          mountPath: /etc/nereus
          readOnly: true
        - name: data
          mountPath: /var/lib/nereus
        env:
        - name: NEREUS_CONFIG
          value: "/etc/nereus/config.yaml"
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 9090
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: nereus-config
      - name: data
        persistentVolumeClaim:
          claimName: nereus-data
```

### Services

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nereus-snmp
  namespace: nereus
spec:
  type: LoadBalancer
  ports:
  - port: 162
    targetPort: 162
    protocol: UDP
    name: snmp
  selector:
    app: nereus
---
apiVersion: v1
kind: Service
metadata:
  name: nereus-metrics
  namespace: nereus
  labels:
    app: nereus
spec:
  ports:
  - port: 9090
    targetPort: 9090
    protocol: TCP
    name: metrics
  selector:
    app: nereus
```

### Persistent Volume Claim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nereus-data
  namespace: nereus
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: fast-ssd
```

## Systemd Service

### Service File

Create `/etc/systemd/system/nereus.service`:

```ini
[Unit]
Description=Nereus SNMP Trap Alerting System
Documentation=https://github.com/geekxflood/nereus
After=network.target
Wants=network.target

[Service]
Type=simple
User=nereus
Group=nereus
ExecStart=/usr/local/bin/nereus --config /etc/nereus/config.yaml
ExecReload=/bin/kill -HUP $MAINPID
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=nereus

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nereus /var/log/nereus
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

### User and Directory Setup

```bash
# Create user
sudo useradd --system --shell /bin/false --home-dir /var/lib/nereus nereus

# Create directories
sudo mkdir -p /etc/nereus /var/lib/nereus /var/log/nereus
sudo chown nereus:nereus /var/lib/nereus /var/log/nereus
sudo chmod 755 /etc/nereus
sudo chmod 750 /var/lib/nereus /var/log/nereus

# Set up configuration
sudo nereus generate --output /etc/nereus/config.yaml
sudo chown root:nereus /etc/nereus/config.yaml
sudo chmod 640 /etc/nereus/config.yaml
```

### Service Management

```bash
# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable nereus
sudo systemctl start nereus

# Check status
sudo systemctl status nereus

# View logs
sudo journalctl -u nereus -f

# Reload configuration
sudo systemctl reload nereus
```

## Security Considerations

### Network Security

1. **Firewall Configuration:**
   ```bash
   # UFW (Ubuntu)
   sudo ufw allow 162/udp comment 'SNMP Traps'
   sudo ufw allow from 10.0.0.0/8 to any port 9090 comment 'Metrics (internal)'

   # iptables
   sudo iptables -A INPUT -p udp --dport 162 -j ACCEPT
   sudo iptables -A INPUT -p tcp --dport 9090 -s 10.0.0.0/8 -j ACCEPT
   ```

2. **SNMP Community Strings:**
   - Use strong, unique community strings
   - Rotate community strings regularly
   - Consider IP-based access control

3. **TLS Configuration:**
   ```yaml
   notifier:
     endpoints:
       - name: "secure-webhook"
         url: "https://alerts.example.com/webhook"
         tls:
           cert_file: "/etc/nereus/certs/client.crt"
           key_file: "/etc/nereus/certs/client.key"
           ca_file: "/etc/nereus/certs/ca.crt"
           insecure_skip_verify: false
   ```

### File Permissions

```bash
# Configuration files
sudo chmod 640 /etc/nereus/config.yaml
sudo chown root:nereus /etc/nereus/config.yaml

# Database and logs
sudo chmod 750 /var/lib/nereus
sudo chmod 750 /var/log/nereus
sudo chown nereus:nereus /var/lib/nereus /var/log/nereus

# Binary
sudo chmod 755 /usr/local/bin/nereus
sudo chown root:root /usr/local/bin/nereus
```

### SELinux Configuration

For RHEL/CentOS systems with SELinux:

```bash
# Create SELinux policy
sudo setsebool -P nis_enabled 1
sudo semanage port -a -t snmp_port_t -p udp 162

# Custom policy for Nereus
sudo semanage fcontext -a -t bin_t "/usr/local/bin/nereus"
sudo restorecon -v /usr/local/bin/nereus
```

## Monitoring and Observability

### Prometheus Integration

Add to Prometheus configuration:

```yaml
scrape_configs:
  - job_name: 'nereus'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
    metrics_path: /metrics
```

### Grafana Dashboard

Key metrics to monitor:
- `nereus_traps_received_total` - Total traps received
- `nereus_traps_processed_total` - Total traps processed
- `nereus_webhooks_sent_total` - Total webhooks sent
- `nereus_webhook_errors_total` - Webhook errors
- `nereus_correlation_groups_active` - Active correlation groups
- `nereus_storage_events_total` - Events in storage

### Health Checks

```bash
# Application health
curl http://localhost:9090/health

# Readiness check
curl http://localhost:9090/ready

# Metrics endpoint
curl http://localhost:9090/metrics
```

### Log Management

#### Logrotate Configuration

Create `/etc/logrotate.d/nereus`:

```
/var/log/nereus/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 nereus nereus
    postrotate
        systemctl reload nereus
    endscript
}
```

#### Centralized Logging

For ELK Stack integration:

```yaml
logging:
  format: "json"
  file: "/var/log/nereus/nereus.log"
  max_size: "100MB"
  max_backups: 10
  max_age: "30d"
```

## Troubleshooting

### Common Issues

1. **Permission Denied on Port 162:**
   ```bash
   # Grant capability to bind to privileged ports
   sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/nereus
   ```

2. **Database Lock Errors:**
   ```bash
   # Check file permissions
   ls -la /var/lib/nereus/

   # Ensure proper ownership
   sudo chown nereus:nereus /var/lib/nereus/nereus.db
   ```

3. **High Memory Usage:**
   - Reduce `correlator.max_groups`
   - Decrease `storage.retention_days`
   - Increase `storage.batch_size` and decrease `storage.flush_interval`

4. **Webhook Timeouts:**
   - Increase `notifier.endpoints[].timeout`
   - Reduce `notifier.endpoints[].retry.max_attempts`
   - Check network connectivity to webhook endpoints

### Debug Mode

Enable debug logging:

```yaml
app:
  log_level: debug

logging:
  level: debug
```

### Performance Tuning

For high-volume environments:

```yaml
listener:
  workers: 8  # Scale with CPU cores
  buffer_size: 10000

storage:
  batch_size: 1000
  flush_interval: "5s"
  max_connections: 20

correlator:
  window_duration: "1m"  # Reduce for faster processing
  max_groups: 50000

notifier:
  workers: 4
  queue_size: 50000
```

### Backup and Recovery

```bash
# Backup database
sudo -u nereus sqlite3 /var/lib/nereus/nereus.db ".backup /backup/nereus-$(date +%Y%m%d).db"

# Backup configuration
sudo cp -r /etc/nereus /backup/nereus-config-$(date +%Y%m%d)

# Restore database
sudo systemctl stop nereus
sudo -u nereus cp /backup/nereus-20231201.db /var/lib/nereus/nereus.db
sudo systemctl start nereus
```
```
