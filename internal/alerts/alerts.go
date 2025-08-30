// Package alerts provides Prometheus alert model integration for SNMP trap notifications.
package alerts

import (
	"fmt"
	"strconv"
	"time"

	"github.com/geekxflood/nereus/internal/storage"
	"github.com/prometheus/common/model"
)

// AlertConverter converts SNMP trap events to Prometheus alert format
type AlertConverter struct {
	defaultLabels model.LabelSet
	annotations   model.LabelSet
}

// NewAlertConverter creates a new alert converter with default labels
func NewAlertConverter() *AlertConverter {
	return &AlertConverter{
		defaultLabels: model.LabelSet{
			"alertname": "SNMPTrap",
			"service":   "nereus-snmp-listener",
		},
		annotations: model.LabelSet{
			"summary":     "SNMP trap received",
			"description": "An SNMP trap was received and processed",
		},
	}
}

// ConvertEvent converts a storage event to a Prometheus alert
func (ac *AlertConverter) ConvertEvent(event *storage.Event) (*model.Alert, error) {
	if event == nil {
		return nil, fmt.Errorf("event cannot be nil")
	}

	// Create labels
	labels := model.LabelSet{
		"alertname": "SNMPTrap",
		"source_ip": model.LabelValue(event.SourceIP),
		"community": model.LabelValue(event.Community),
		"trap_oid":  model.LabelValue(event.TrapOID),
		"severity":  model.LabelValue(event.Severity),
		"status":    model.LabelValue(event.Status),
		"instance":  model.LabelValue(event.SourceIP),
		"job":       "snmp-trap",
	}

	// Add trap name if available
	if event.TrapName != "" {
		labels["trap_name"] = model.LabelValue(event.TrapName)
		labels["alertname"] = model.LabelValue(fmt.Sprintf("SNMPTrap_%s", event.TrapName))
	}

	// Add correlation ID if available
	if event.CorrelationID != "" {
		labels["correlation_id"] = model.LabelValue(event.CorrelationID)
	}

	// Add version and PDU type
	labels["snmp_version"] = model.LabelValue(strconv.Itoa(event.Version))
	labels["pdu_type"] = model.LabelValue(strconv.Itoa(event.PDUType))

	// Create annotations
	annotations := model.LabelSet{
		"summary": model.LabelValue(fmt.Sprintf("SNMP trap %s from %s", event.TrapName, event.SourceIP)),
		"description": model.LabelValue(fmt.Sprintf(
			"SNMP trap received from %s (community: %s, OID: %s, severity: %s)",
			event.SourceIP, event.Community, event.TrapOID, event.Severity,
		)),
		"trap_oid":    model.LabelValue(event.TrapOID),
		"source_ip":   model.LabelValue(event.SourceIP),
		"community":   model.LabelValue(event.Community),
		"first_seen":  model.LabelValue(event.FirstSeen.Format(time.RFC3339)),
		"last_seen":   model.LabelValue(event.LastSeen.Format(time.RFC3339)),
		"event_count": model.LabelValue(strconv.Itoa(event.Count)),
	}

	// Add request ID
	annotations["request_id"] = model.LabelValue(strconv.Itoa(int(event.RequestID)))

	// Add acknowledgment info if acknowledged
	if event.Acknowledged {
		annotations["acknowledged"] = "true"
		if event.AckBy != "" {
			annotations["acknowledged_by"] = model.LabelValue(event.AckBy)
		}
		if event.AckTime != nil {
			annotations["acknowledged_at"] = model.LabelValue(event.AckTime.Format(time.RFC3339))
		}
	}

	// Add varbinds if available
	if event.Varbinds != "" {
		annotations["varbinds"] = model.LabelValue(event.Varbinds)
	}

	// Add metadata if available
	if event.Metadata != "" {
		annotations["metadata"] = model.LabelValue(event.Metadata)
	}

	// Create the alert
	alert := &model.Alert{
		Labels:       labels,
		Annotations:  annotations,
		StartsAt:     event.FirstSeen,
		EndsAt:       time.Time{}, // Empty means ongoing
		GeneratorURL: "",
	}

	// Set EndsAt for resolved alerts
	if event.Status == "resolved" || event.Status == "closed" {
		alert.EndsAt = event.LastSeen
	}

	return alert, nil
}

// ConvertEvents converts multiple storage events to Prometheus alerts
func (ac *AlertConverter) ConvertEvents(events []*storage.Event) ([]*model.Alert, error) {
	if len(events) == 0 {
		return nil, nil
	}

	alerts := make([]*model.Alert, 0, len(events))
	for _, event := range events {
		alert, err := ac.ConvertEvent(event)
		if err != nil {
			return nil, fmt.Errorf("failed to convert event %d: %w", event.ID, err)
		}
		alerts = append(alerts, alert)
	}

	return alerts, nil
}

// CreateAlertGroup creates a Prometheus alert group from events
func (ac *AlertConverter) CreateAlertGroup(groupName string, events []*storage.Event) (*AlertGroup, error) {
	alerts, err := ac.ConvertEvents(events)
	if err != nil {
		return nil, err
	}

	return &AlertGroup{
		Name:   groupName,
		Alerts: alerts,
	}, nil
}

// AlertGroup represents a group of related alerts
type AlertGroup struct {
	Name   string         `json:"name"`
	Alerts []*model.Alert `json:"alerts"`
}

// SeverityMapping maps SNMP trap severities to Prometheus alert severities
var SeverityMapping = map[string]string{
	"emergency":     "critical",
	"alert":         "critical",
	"critical":      "critical",
	"error":         "major",
	"warning":       "minor",
	"notice":        "info",
	"informational": "info",
	"info":          "info",
	"debug":         "info",
}

// MapSeverity maps SNMP severity to Prometheus severity
func MapSeverity(snmpSeverity string) string {
	if promSeverity, exists := SeverityMapping[snmpSeverity]; exists {
		return promSeverity
	}
	return "info" // default
}

// AlertManagerPayload represents the payload format expected by Alertmanager
type AlertManagerPayload struct {
	Alerts []AlertManagerAlert `json:"alerts"`
}

// AlertManagerAlert represents a single alert in Alertmanager format
type AlertManagerAlert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt,omitempty"`
	GeneratorURL string            `json:"generatorURL,omitempty"`
}

// ConvertToAlertManagerPayload converts Prometheus alerts to Alertmanager payload format
func (ac *AlertConverter) ConvertToAlertManagerPayload(alerts []*model.Alert) *AlertManagerPayload {
	payload := &AlertManagerPayload{
		Alerts: make([]AlertManagerAlert, len(alerts)),
	}

	for i, alert := range alerts {
		// Convert labels
		labels := make(map[string]string)
		for k, v := range alert.Labels {
			labels[string(k)] = string(v)
		}

		// Convert annotations
		annotations := make(map[string]string)
		for k, v := range alert.Annotations {
			annotations[string(k)] = string(v)
		}

		payload.Alerts[i] = AlertManagerAlert{
			Labels:       labels,
			Annotations:  annotations,
			StartsAt:     alert.StartsAt,
			EndsAt:       alert.EndsAt,
			GeneratorURL: alert.GeneratorURL,
		}
	}

	return payload
}

// ConvertEventToAlertManagerPayload converts a single event directly to Alertmanager payload
func (ac *AlertConverter) ConvertEventToAlertManagerPayload(event *storage.Event) (*AlertManagerPayload, error) {
	alert, err := ac.ConvertEvent(event)
	if err != nil {
		return nil, err
	}

	return ac.ConvertToAlertManagerPayload([]*model.Alert{alert}), nil
}

// SetDefaultLabels sets default labels for all alerts
func (ac *AlertConverter) SetDefaultLabels(labels model.LabelSet) {
	ac.defaultLabels = labels
}

// SetDefaultAnnotations sets default annotations for all alerts
func (ac *AlertConverter) SetDefaultAnnotations(annotations model.LabelSet) {
	ac.annotations = annotations
}

// AddDefaultLabel adds a default label
func (ac *AlertConverter) AddDefaultLabel(key, value string) {
	if ac.defaultLabels == nil {
		ac.defaultLabels = make(model.LabelSet)
	}
	ac.defaultLabels[model.LabelName(key)] = model.LabelValue(value)
}

// AddDefaultAnnotation adds a default annotation
func (ac *AlertConverter) AddDefaultAnnotation(key, value string) {
	if ac.annotations == nil {
		ac.annotations = make(model.LabelSet)
	}
	ac.annotations[model.LabelName(key)] = model.LabelValue(value)
}
