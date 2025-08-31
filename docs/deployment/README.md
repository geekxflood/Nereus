# Deployment Guide

This document provides comprehensive deployment instructions for Nereus in various environments.

## Deployment Options

- [Docker](#docker-deployment)
- [Kubernetes](#kubernetes-deployment)
- [Systemd Service](#systemd-service)
- [Binary Installation](#binary-installation)

## Prerequisites

### System Requirements

- **CPU**: 2+ cores recommended for production
- **Memory**: 512MB minimum, 2GB recommended
- **Storage**: 10GB for database and logs
- **Network**: UDP port 162 for SNMP traps, TCP port 9090 for metrics

### Dependencies

- **MIB Files**: Standard SNMP MIBs (SNMPv2-SMI, SNMPv2-TC, SNMPv2-MIB)
- **Database**: SQLite (embedded, no external dependency)
- **Monitoring**: Prometheus (optional, for metrics collection)

## Docker Deployment

### Quick Start

```bash
# Pull and run the latest image
docker run -d \
  --name nereus \
  -p 162:162/udp \
  -p 9090:9090 \
  -v $(pwd)/config.yaml:/etc/nereus/config.yaml \
  -v $(pwd)/mibs:/usr/share/snmp/mibs \
  -v $(pwd)/data:/var/lib/nereus \
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
      - "162:162/udp"    # SNMP trap port
      - "9090:9090/tcp"  # Metrics port
    volumes:
      - ./config.yaml:/etc/nereus/config.yaml:ro
      - ./mibs:/usr/share/snmp/mibs:ro
      - ./data:/var/lib/nereus
      - ./logs:/var/log/nereus
    environment:
      - NEREUS_LOG_LEVEL=info
      - NEREUS_LOG_FORMAT=json
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:9090/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  # Optional: Prometheus for metrics collection
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'

  # Optional: Alertmanager for alert routing
  alertmanager:
    image: prom/alertmanager:latest
    container_name: alertmanager
    ports:
      - "9093:9093"
    volumes:
      - ./alertmanager.yml:/etc/alertmanager/alertmanager.yml:ro
```

### Building Custom Image

```bash
# Clone repository
git clone https://github.com/geekxflood/nereus.git
cd nereus

# Build image
docker build -t nereus:custom .

# Run custom image
docker run -d --name nereus-custom nereus:custom
```

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: nereus-config
  namespace: monitoring
data:
  config.yaml: |
    app:
      name: "nereus-k8s"
      environment: "production"
    
    server:
      host: "0.0.0.0"
      port: 162
      buffer_size: 65536
    
    mib:
      directories: ["/usr/share/snmp/mibs"]
      enable_hot_reload: true
      required_mibs: ["SNMPv2-SMI", "SNMPv2-TC", "SNMPv2-MIB"]
    
    storage:
      connection_string: "/var/lib/nereus/events.db"
      retention_days: 30
    
    metrics:
      enabled: true
      listen_address: ":9090"
    
    logging:
      level: "info"
      format: "json"
```

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nereus
  namespace: monitoring
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
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: nereus
        image: geekxflood/nereus:latest
        imagePullPolicy: Always
        ports:
        - name: snmp
          containerPort: 162
          protocol: UDP
        - name: metrics
          containerPort: 9090
          protocol: TCP
        env:
        - name: NEREUS_LOG_LEVEL
          value: "info"
        - name: NEREUS_LOG_FORMAT
          value: "json"
        volumeMounts:
        - name: config
          mountPath: /etc/nereus
          readOnly: true
        - name: data
          mountPath: /var/lib/nereus
        - name: mibs
          mountPath: /usr/share/snmp/mibs
          readOnly: true
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
          periodSeconds: 30
          timeoutSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 9090
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 5
      volumes:
      - name: config
        configMap:
          name: nereus-config
      - name: data
        persistentVolumeClaim:
          claimName: nereus-data
      - name: mibs
        configMap:
          name: snmp-mibs
```

### Service and Ingress

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nereus-snmp
  namespace: monitoring
spec:
  type: LoadBalancer
  ports:
  - name: snmp
    port: 162
    targetPort: 162
    protocol: UDP
  selector:
    app: nereus
---
apiVersion: v1
kind: Service
metadata:
  name: nereus-metrics
  namespace: monitoring
  labels:
    app: nereus
spec:
  ports:
  - name: metrics
    port: 9090
    targetPort: 9090
    protocol: TCP
  selector:
    app: nereus
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: nereus-metrics
  namespace: monitoring
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: nereus-metrics.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: nereus-metrics
            port:
              number: 9090
```

### Persistent Volume

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nereus-data
  namespace: monitoring
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

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

### Installation Steps

```bash
# Create user and directories
sudo useradd -r -s /bin/false nereus
sudo mkdir -p /etc/nereus /var/lib/nereus /var/log/nereus
sudo chown nereus:nereus /var/lib/nereus /var/log/nereus

# Install binary
sudo cp nereus /usr/local/bin/
sudo chmod +x /usr/local/bin/nereus

# Install configuration
sudo cp config.yaml /etc/nereus/
sudo chown root:nereus /etc/nereus/config.yaml
sudo chmod 640 /etc/nereus/config.yaml

# Enable and start service
sudo systemctl daemon-reload
sudo systemctl enable nereus
sudo systemctl start nereus

# Check status
sudo systemctl status nereus
sudo journalctl -u nereus -f
```

## Binary Installation

### Download and Install

```bash
# Download latest release
wget https://github.com/geekxflood/nereus/releases/latest/download/nereus-linux-amd64.tar.gz

# Extract
tar -xzf nereus-linux-amd64.tar.gz

# Install
sudo cp nereus /usr/local/bin/
sudo chmod +x /usr/local/bin/nereus

# Create directories
sudo mkdir -p /etc/nereus /var/lib/nereus

# Generate configuration
nereus generate --output /etc/nereus/config.yaml

# Run
nereus --config /etc/nereus/config.yaml
```

### Build from Source

```bash
# Prerequisites
go version  # Requires Go 1.25+

# Clone and build
git clone https://github.com/geekxflood/nereus.git
cd nereus
go build -ldflags "-X main.version=$(git describe --tags)" -o nereus ./main.go

# Install
sudo cp nereus /usr/local/bin/
```

## Production Considerations

### Security

1. **Network Security**:
   - Use firewall rules to restrict SNMP trap sources
   - Enable TLS for webhook endpoints
   - Regular security updates

2. **Application Security**:
   - Run as non-root user
   - Use strong community strings
   - Validate all configuration inputs

3. **Container Security**:
   - Use official base images
   - Regular image updates
   - Security scanning

### High Availability

1. **Load Balancing**:
   - Multiple Nereus instances behind UDP load balancer
   - Shared database for event correlation
   - Health check integration

2. **Database**:
   - Regular database backups
   - Database replication for critical deployments
   - Monitoring database size and performance

3. **Monitoring**:
   - Prometheus metrics collection
   - Alerting on service health
   - Log aggregation and analysis

### Performance Tuning

1. **Resource Allocation**:
   - CPU: 2+ cores for high-throughput scenarios
   - Memory: 2GB+ for large MIB collections
   - Storage: SSD for database performance

2. **Configuration Tuning**:
   - Adjust worker pool sizes based on load
   - Optimize buffer sizes for network conditions
   - Configure appropriate timeouts

3. **Database Optimization**:
   - Enable WAL mode for better concurrency
   - Regular VACUUM operations
   - Monitor database size and performance

### Backup and Recovery

1. **Database Backup**:
   ```bash
   # Backup SQLite database
   sqlite3 /var/lib/nereus/events.db ".backup /backup/nereus-$(date +%Y%m%d).db"
   ```

2. **Configuration Backup**:
   ```bash
   # Backup configuration
   cp /etc/nereus/config.yaml /backup/config-$(date +%Y%m%d).yaml
   ```

3. **Recovery Procedures**:
   - Document recovery procedures
   - Test backup restoration regularly
   - Monitor backup integrity

## Troubleshooting

### Common Issues

1. **Port 162 Permission Denied**:
   ```bash
   # Solution: Use capabilities or run as root
   sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/nereus
   ```

2. **MIB Loading Failures**:
   ```bash
   # Check MIB file permissions and paths
   ls -la /usr/share/snmp/mibs/
   ```

3. **Database Lock Issues**:
   ```bash
   # Check for multiple instances or file permissions
   lsof /var/lib/nereus/events.db
   ```

### Monitoring and Alerting

Set up monitoring for:
- Service health and availability
- SNMP trap processing rates
- Database size and performance
- Webhook delivery success rates
- System resource usage

### Log Analysis

Key log patterns to monitor:
- SNMP trap reception and processing
- MIB loading and resolution errors
- Database operation failures
- Webhook delivery failures
- Configuration validation errors
