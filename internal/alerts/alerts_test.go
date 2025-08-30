package alerts

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/geekxflood/nereus/internal/storage"
)

func TestNewAlertConverter(t *testing.T) {
	converter := NewAlertConverter()
	if converter == nil {
		t.Fatal("AlertConverter is nil")
	}

	if converter.defaultLabels == nil {
		t.Error("Default labels not initialized")
	}

	if converter.annotations == nil {
		t.Error("Default annotations not initialized")
	}

	// Check default labels
	if string(converter.defaultLabels["alertname"]) != "SNMPTrap" {
		t.Error("Default alertname not set correctly")
	}

	if string(converter.defaultLabels["service"]) != "nereus-snmp-listener" {
		t.Error("Default service not set correctly")
	}
}

func TestConvertEvent(t *testing.T) {
	converter := NewAlertConverter()
	
	// Create test event
	now := time.Now()
	event := &storage.Event{
		ID:            123,
		Timestamp:     now,
		SourceIP:      "192.168.1.100",
		Community:     "public",
		Version:       2,
		PDUType:       4,
		RequestID:     456,
		TrapOID:       "1.3.6.1.6.3.1.1.5.3",
		TrapName:      "linkDown",
		Severity:      "major",
		Status:        "firing",
		Acknowledged:  false,
		Count:         1,
		FirstSeen:     now,
		LastSeen:      now,
		Varbinds:      `[{"oid":"1.3.6.1.2.1.2.2.1.1.1","value":"1"}]`,
		Metadata:      `{"interface":"eth0"}`,
		Hash:          "abc123",
		CorrelationID: "corr-456",
	}

	alert, err := converter.ConvertEvent(event)
	if err != nil {
		t.Fatalf("Failed to convert event: %v", err)
	}

	if alert == nil {
		t.Fatal("Alert is nil")
	}

	// Check labels
	if string(alert.Labels["alertname"]) != "SNMPTrap_linkDown" {
		t.Errorf("Expected alertname 'SNMPTrap_linkDown', got '%s'", alert.Labels["alertname"])
	}

	if string(alert.Labels["source_ip"]) != "192.168.1.100" {
		t.Errorf("Expected source_ip '192.168.1.100', got '%s'", alert.Labels["source_ip"])
	}

	if string(alert.Labels["severity"]) != "major" {
		t.Errorf("Expected severity 'major', got '%s'", alert.Labels["severity"])
	}

	if string(alert.Labels["trap_oid"]) != "1.3.6.1.6.3.1.1.5.3" {
		t.Errorf("Expected trap_oid '1.3.6.1.6.3.1.1.5.3', got '%s'", alert.Labels["trap_oid"])
	}

	if string(alert.Labels["correlation_id"]) != "corr-456" {
		t.Errorf("Expected correlation_id 'corr-456', got '%s'", alert.Labels["correlation_id"])
	}

	// Check annotations
	if string(alert.Annotations["summary"]) != "SNMP trap linkDown from 192.168.1.100" {
		t.Errorf("Unexpected summary: %s", alert.Annotations["summary"])
	}

	if string(alert.Annotations["varbinds"]) != `[{"oid":"1.3.6.1.2.1.2.2.1.1.1","value":"1"}]` {
		t.Errorf("Unexpected varbinds: %s", alert.Annotations["varbinds"])
	}

	// Check timestamps
	if !alert.StartsAt.Equal(event.FirstSeen) {
		t.Error("StartsAt timestamp mismatch")
	}

	if !alert.EndsAt.IsZero() {
		t.Error("EndsAt should be zero for firing alerts")
	}
}

func TestConvertEventWithAcknowledgment(t *testing.T) {
	converter := NewAlertConverter()
	
	now := time.Now()
	ackTime := now.Add(5 * time.Minute)
	
	event := &storage.Event{
		ID:           123,
		SourceIP:     "192.168.1.100",
		Community:    "public",
		TrapOID:      "1.3.6.1.6.3.1.1.5.3",
		TrapName:     "linkDown",
		Severity:     "major",
		Status:       "firing",
		Acknowledged: true,
		AckBy:        "admin",
		AckTime:      &ackTime,
		FirstSeen:    now,
		LastSeen:     now,
	}

	alert, err := converter.ConvertEvent(event)
	if err != nil {
		t.Fatalf("Failed to convert event: %v", err)
	}

	// Check acknowledgment annotations
	if string(alert.Annotations["acknowledged"]) != "true" {
		t.Error("Acknowledged annotation not set correctly")
	}

	if string(alert.Annotations["acknowledged_by"]) != "admin" {
		t.Error("Acknowledged by annotation not set correctly")
	}

	if string(alert.Annotations["acknowledged_at"]) != ackTime.Format(time.RFC3339) {
		t.Error("Acknowledged at annotation not set correctly")
	}
}

func TestConvertEventResolved(t *testing.T) {
	converter := NewAlertConverter()
	
	now := time.Now()
	endTime := now.Add(10 * time.Minute)
	
	event := &storage.Event{
		ID:        123,
		SourceIP:  "192.168.1.100",
		Community: "public",
		TrapOID:   "1.3.6.1.6.3.1.1.5.4",
		TrapName:  "linkUp",
		Severity:  "info",
		Status:    "resolved",
		FirstSeen: now,
		LastSeen:  endTime,
	}

	alert, err := converter.ConvertEvent(event)
	if err != nil {
		t.Fatalf("Failed to convert event: %v", err)
	}

	// Check that EndsAt is set for resolved alerts
	if alert.EndsAt.IsZero() {
		t.Error("EndsAt should be set for resolved alerts")
	}

	if !alert.EndsAt.Equal(event.LastSeen) {
		t.Error("EndsAt timestamp mismatch")
	}
}

func TestConvertEvents(t *testing.T) {
	converter := NewAlertConverter()
	
	now := time.Now()
	events := []*storage.Event{
		{
			ID:        1,
			SourceIP:  "192.168.1.100",
			TrapName:  "linkDown",
			FirstSeen: now,
		},
		{
			ID:        2,
			SourceIP:  "192.168.1.101",
			TrapName:  "linkUp",
			FirstSeen: now,
		},
	}

	alerts, err := converter.ConvertEvents(events)
	if err != nil {
		t.Fatalf("Failed to convert events: %v", err)
	}

	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(alerts))
	}

	// Check first alert
	if string(alerts[0].Labels["source_ip"]) != "192.168.1.100" {
		t.Error("First alert source_ip mismatch")
	}

	// Check second alert
	if string(alerts[1].Labels["source_ip"]) != "192.168.1.101" {
		t.Error("Second alert source_ip mismatch")
	}
}

func TestConvertToAlertManagerPayload(t *testing.T) {
	converter := NewAlertConverter()
	
	now := time.Now()
	alert := &model.Alert{
		Labels: model.LabelSet{
			"alertname": "TestAlert",
			"severity":  "critical",
		},
		Annotations: model.LabelSet{
			"summary": "Test alert summary",
		},
		StartsAt: now,
		EndsAt:   time.Time{},
	}

	payload := converter.ConvertToAlertManagerPayload([]*model.Alert{alert})
	
	if payload == nil {
		t.Fatal("Payload is nil")
	}

	if len(payload.Alerts) != 1 {
		t.Errorf("Expected 1 alert in payload, got %d", len(payload.Alerts))
	}

	alertManagerAlert := payload.Alerts[0]
	
	if alertManagerAlert.Labels["alertname"] != "TestAlert" {
		t.Error("Alertname not converted correctly")
	}

	if alertManagerAlert.Labels["severity"] != "critical" {
		t.Error("Severity not converted correctly")
	}

	if alertManagerAlert.Annotations["summary"] != "Test alert summary" {
		t.Error("Summary not converted correctly")
	}

	if !alertManagerAlert.StartsAt.Equal(now) {
		t.Error("StartsAt not converted correctly")
	}
}

func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"emergency", "critical"},
		{"alert", "critical"},
		{"critical", "critical"},
		{"error", "major"},
		{"warning", "minor"},
		{"notice", "info"},
		{"informational", "info"},
		{"info", "info"},
		{"debug", "info"},
		{"unknown", "info"}, // default case
	}

	for _, test := range tests {
		result := MapSeverity(test.input)
		if result != test.expected {
			t.Errorf("MapSeverity(%s) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestConvertEventNil(t *testing.T) {
	converter := NewAlertConverter()
	
	_, err := converter.ConvertEvent(nil)
	if err == nil {
		t.Error("Expected error for nil event")
	}
}

func TestAddDefaultLabelsAndAnnotations(t *testing.T) {
	converter := NewAlertConverter()
	
	// Add custom default label
	converter.AddDefaultLabel("environment", "production")
	converter.AddDefaultAnnotation("runbook", "https://example.com/runbook")
	
	if string(converter.defaultLabels["environment"]) != "production" {
		t.Error("Custom default label not set correctly")
	}
	
	if string(converter.annotations["runbook"]) != "https://example.com/runbook" {
		t.Error("Custom default annotation not set correctly")
	}
}
