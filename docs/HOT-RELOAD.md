# Hot Reload Support

Nereus provides comprehensive hot reload functionality that allows you to update configuration and MIB files without restarting the application. This feature is essential for production environments where downtime must be minimized.

## Overview

The hot reload system monitors file changes and automatically applies updates to:

- Application configuration
- MIB files and directories
- Webhook configurations
- Component-specific settings

All reloads are performed gracefully with proper validation, error handling, and state preservation.

## Configuration

Enable and configure hot reload in your configuration file:

```yaml
reload:
  # Enable hot reload functionality
  enabled: true

  # Watch the configuration file for changes
  watch_config_file: true

  # Watch MIB directories for changes
  watch_mib_directories: true

  # Delay before processing reload events (debounce)
  reload_delay: "2s"

  # Maximum number of reload attempts on failure
  max_reload_attempts: 3

  # Timeout for reload operations
  reload_timeout: "30s"

  # Preserve application state during reload
  preserve_state: true

  # Validate configuration before applying reload
  validate_before_reload: true
```

## Supported Reload Types

### Configuration Reload (`config`)

- Monitors the main configuration file for changes
- Validates new configuration before applying
- Updates all registered components with new settings
- Preserves application state and connections

### MIB File Reload (`mib`)

- Monitors MIB directories for file changes
- Supports adding, removing, and modifying MIB files
- Updates OID resolution cache automatically
- Maintains existing trap processing capabilities

### Webhook Reload (`webhook`)

- Updates webhook configurations dynamically
- Modifies notification templates and endpoints
- Adjusts retry policies and timeouts
- Preserves queued notifications

### Full Reload (`all`)

- Reloads all components and configurations
- Performs comprehensive system refresh
- Maintains service availability throughout process

## Usage

### Automatic Reload

Hot reload is triggered automatically when monitored files change:

1. **Configuration Changes**: Edit your configuration file and save it
2. **MIB Updates**: Add, remove, or modify MIB files in watched directories
3. **Debouncing**: Multiple rapid changes are batched together
4. **Validation**: New configuration is validated before application
5. **Application**: Changes are applied gracefully with minimal disruption

### Manual Reload

You can also trigger reloads manually:

```bash
# Send SIGHUP signal for full reload
kill -HUP <nereus-pid>

# Or use the application API (if implemented)
curl -X POST http://localhost:9090/reload
```

### Programmatic Reload

Components can trigger reloads programmatically:

```go
// Get reload manager from application
reloadManager := app.GetReloadManager()

// Trigger specific reload type
err := reloadManager.TriggerReload(reload.ReloadTypeConfig, "manual")
if err != nil {
    log.Printf("Reload failed: %v", err)
}
```

## Monitoring and Observability

### Reload Events

All reload events are logged with structured data:

```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "INFO",
  "msg": "Reload completed successfully",
  "component": "reload",
  "type": "config",
  "source": "/etc/nereus/config.yml",
  "duration": "150ms",
  "success": true
}
```

### Reload Statistics

Monitor reload performance and success rates:

```go
stats := reloadManager.GetStats()
fmt.Printf("Total reloads: %d\n", stats.TotalReloads)
fmt.Printf("Success rate: %.2f%%\n", 
    float64(stats.SuccessfulReloads)/float64(stats.TotalReloads)*100)
```

### Recent Events

View recent reload events for debugging:

```go
events := reloadManager.GetRecentEvents(10)
for _, event := range events {
    fmt.Printf("%s: %s (%v)\n", event.Type, event.Source, event.Success)
}
```

### Metrics Integration

Hot reload statistics are exposed via Prometheus metrics:

```promql
# Reload event counters
nereus_reload_events_total{type="config",result="success"} 15
nereus_reload_events_total{type="mib",result="success"} 8
nereus_reload_events_total{type="config",result="failure"} 1

# Reload timing
nereus_reload_duration_seconds{type="config"} 0.150
nereus_reload_duration_seconds{type="mib"} 0.320

# Component reload statistics
nereus_component_reloads_total{component="mib_loader"} 8
nereus_component_reloads_total{component="notifier"} 15
```

## Component Integration

### Implementing ComponentReloader

Components that support hot reload must implement the `ComponentReloader` interface:

```go
type ComponentReloader interface {
    Reload(configProvider config.Provider) error
    GetReloadStats() map[string]any
}
```

Example implementation:

```go
func (c *MyComponent) Reload(configProvider config.Provider) error {
    // Update component configuration
    if newSetting, err := configProvider.GetString("my.setting"); err == nil {
        c.setting = newSetting
    }

    // Reinitialize resources if needed
    return c.reinitialize()
}

func (c *MyComponent) GetReloadStats() map[string]any {
    return map[string]any{
        "reload_count": c.reloadCount,
        "last_reload": c.lastReload,
        "config_version": c.configVersion,
    }
}
```

### Registering Components

Register components with the reload manager during initialization:

```go
// Register component for hot reload
app.GetReloadManager().RegisterComponent("my_component", myComponent)
```

## Best Practices

### Configuration Design

1. **Use hierarchical configuration** - Group related settings together
2. **Provide sensible defaults** - Ensure partial reloads work correctly
3. **Validate early** - Check configuration before applying changes
4. **Document reload behavior** - Specify which settings require restart

### Error Handling

1. **Graceful degradation** - Continue operation if reload fails
2. **Rollback capability** - Revert to previous configuration on failure
3. **Detailed logging** - Log reload attempts, successes, and failures
4. **Alert on failures** - Monitor reload success rates

### Performance Considerations

1. **Debounce file events** - Batch multiple rapid changes
2. **Minimize reload scope** - Only reload affected components
3. **Preserve connections** - Maintain existing client connections
4. **Background processing** - Perform reloads asynchronously

## Troubleshooting

### Common Issues

1. **File permission errors**

   ```log
   ERROR: Failed to watch config file: permission denied
   ```

   - Ensure Nereus has read access to configuration files
   - Check file and directory permissions

2. **Configuration validation failures**

   ```log
   ERROR: Configuration validation failed: invalid webhook URL
   ```

   - Validate configuration syntax before saving
   - Check CUE schema compliance

3. **Component reload failures**

   ```log
   ERROR: Failed to reload component mib_loader: file not found
   ```

   - Verify component dependencies are available
   - Check component-specific error logs

4. **File watcher limits**

   ```log
   ERROR: Failed to add file watcher: too many open files
   ```

   - Increase system file descriptor limits
   - Reduce number of watched directories

### Debug Commands

```bash
# Check reload manager status
curl http://localhost:9090/metrics | grep reload

# View recent reload events
curl http://localhost:9090/debug/reload/events

# Trigger manual reload
curl -X POST http://localhost:9090/debug/reload/trigger?type=config

# Check component reload statistics
curl http://localhost:9090/debug/reload/components
```

### Log Analysis

Monitor reload events in application logs:

```bash
# Filter reload events
tail -f /var/log/nereus.log | grep '"component":"reload"'

# Monitor reload success/failure
tail -f /var/log/nereus.log | grep -E '(Reload completed|Reload failed)'

# Track specific reload types
tail -f /var/log/nereus.log | grep '"type":"config"'
```

## Security Considerations

### File Access

- Ensure configuration files have appropriate permissions (600 or 640)
- Limit write access to authorized users only
- Monitor file changes for unauthorized modifications

### Validation

- Always validate configuration before applying changes
- Use CUE schemas for structural validation
- Implement business logic validation for complex settings

### Audit Trail

- Log all reload events with timestamps and sources
- Track configuration changes for compliance
- Monitor reload patterns for anomalies

## Performance Impact

Hot reload is designed to have minimal performance impact:

- **File watching**: Uses efficient OS-level file system notifications
- **Debouncing**: Batches multiple changes to reduce reload frequency
- **Selective reloading**: Only affected components are reloaded
- **Background processing**: Reloads don't block trap processing
- **State preservation**: Existing connections and queues are maintained

Typical reload times:

- Configuration reload: 50-200ms
- MIB file reload: 100-500ms (depending on file count)
- Webhook reload: 10-50ms
- Full reload: 200-1000ms

## Migration Guide

### Enabling Hot Reload

To enable hot reload on an existing Nereus installation:

1. **Update configuration** - Add reload section to config file
2. **Restart once** - Initial restart required to enable file watching
3. **Test reloads** - Verify hot reload works with non-critical changes
4. **Monitor metrics** - Watch reload success rates and performance
5. **Update procedures** - Modify operational procedures to use hot reload

### Component Updates

Existing components can be updated to support hot reload:

1. **Implement interface** - Add `Reload()` and `GetReloadStats()` methods
2. **Register component** - Add registration call during initialization
3. **Test reload behavior** - Verify component handles configuration changes
4. **Update documentation** - Document reload-specific behavior

This hot reload system provides a robust foundation for maintaining Nereus in production environments with minimal downtime and maximum operational flexibility.
