package mib

import (
	"fmt"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// Create a mock config provider
	mockConfig := &MockConfigProvider{
		data: make(map[string]interface{}),
	}

	manager, err := NewManager(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if manager == nil {
		t.Fatal("Manager is nil")
	}

	if manager.oidTree == nil {
		t.Error("OID tree not initialized")
	}

	if manager.mibs == nil {
		t.Error("MIBs map not initialized")
	}

	if manager.nameToOID == nil {
		t.Error("Name to OID map not initialized")
	}

	if manager.oidToName == nil {
		t.Error("OID to name map not initialized")
	}

	if manager.stats == nil {
		t.Error("Stats not initialized")
	}
}

// MockConfigProvider implements config.Provider for testing
type MockConfigProvider struct {
	data map[string]interface{}
}

func (m *MockConfigProvider) Get(key string) (interface{}, error) {
	if value, exists := m.data[key]; exists {
		return value, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

func (m *MockConfigProvider) GetString(key string, defaultValue ...string) (string, error) {
	if value, exists := m.data[key]; exists {
		if str, ok := value.(string); ok {
			return str, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return "", nil
}

func (m *MockConfigProvider) GetBool(key string, defaultValue ...bool) (bool, error) {
	if value, exists := m.data[key]; exists {
		if b, ok := value.(bool); ok {
			return b, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return false, nil
}

func (m *MockConfigProvider) GetInt(key string, defaultValue ...int) (int, error) {
	if value, exists := m.data[key]; exists {
		if i, ok := value.(int); ok {
			return i, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *MockConfigProvider) GetFloat(key string, defaultValue ...float64) (float64, error) {
	if value, exists := m.data[key]; exists {
		if f, ok := value.(float64); ok {
			return f, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0.0, nil
}

func (m *MockConfigProvider) GetDuration(key string, defaultValue ...time.Duration) (time.Duration, error) {
	if value, exists := m.data[key]; exists {
		if d, ok := value.(time.Duration); ok {
			return d, nil
		}
		if str, ok := value.(string); ok {
			return time.ParseDuration(str)
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *MockConfigProvider) GetStringSlice(key string, defaultValue ...[]string) ([]string, error) {
	if value, exists := m.data[key]; exists {
		if slice, ok := value.([]string); ok {
			return slice, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return nil, nil
}

func (m *MockConfigProvider) GetMap(key string) (map[string]any, error) {
	if value, exists := m.data[key]; exists {
		if mapVal, ok := value.(map[string]any); ok {
			return mapVal, nil
		}
	}
	return nil, nil
}

func (m *MockConfigProvider) Validate() error {
	return nil
}

func (m *MockConfigProvider) Exists(key string) bool {
	_, exists := m.data[key]
	return exists
}

func TestCreateRootNode(t *testing.T) {
	root := createRootNode()

	if root == nil {
		t.Fatal("Root node is nil")
	}

	if root.Name != "root" {
		t.Errorf("Expected root name 'root', got '%s'", root.Name)
	}

	if root.Children == nil {
		t.Fatal("Root children not initialized")
	}

	// Check standard nodes are created
	iso, exists := root.Children["1"]
	if !exists {
		t.Error("ISO node not found")
	} else if iso.Name != "iso" {
		t.Errorf("Expected ISO name 'iso', got '%s'", iso.Name)
	}

	// Check internet subtree
	if iso != nil {
		org, exists := iso.Children["3"]
		if !exists {
			t.Error("ORG node not found")
		} else if org.Name != "org" {
			t.Errorf("Expected ORG name 'org', got '%s'", org.Name)
		}

		if org != nil {
			dod, exists := org.Children["6"]
			if !exists {
				t.Error("DOD node not found")
			} else if dod.Name != "dod" {
				t.Errorf("Expected DOD name 'dod', got '%s'", dod.Name)
			}

			if dod != nil {
				internet, exists := dod.Children["1"]
				if !exists {
					t.Error("Internet node not found")
				} else if internet.Name != "internet" {
					t.Errorf("Expected Internet name 'internet', got '%s'", internet.Name)
				}

				// Check mgmt and private subtrees
				if internet != nil {
					mgmt, exists := internet.Children["2"]
					if !exists {
						t.Error("MGMT node not found")
					} else if mgmt.Name != "mgmt" {
						t.Errorf("Expected MGMT name 'mgmt', got '%s'", mgmt.Name)
					}

					private, exists := internet.Children["4"]
					if !exists {
						t.Error("Private node not found")
					} else if private.Name != "private" {
						t.Errorf("Expected Private name 'private', got '%s'", private.Name)
					}

					// Check enterprises
					if private != nil {
						enterprises, exists := private.Children["1"]
						if !exists {
							t.Error("Enterprises node not found")
						} else if enterprises.Name != "enterprises" {
							t.Errorf("Expected Enterprises name 'enterprises', got '%s'", enterprises.Name)
						}
					}
				}
			}
		}
	}
}

func TestManagerBasicFunctionality(t *testing.T) {
	mockConfig := &MockConfigProvider{
		data: make(map[string]interface{}),
	}

	manager, err := NewManager(mockConfig)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test basic functionality
	if manager.GetStats() == nil {
		t.Error("GetStats should not return nil")
	}

	// Test OID resolution with empty tree (should not panic)
	_, err = manager.ResolveOID("1.3.6.1.2.1.1.1.0")
	if err == nil {
		t.Log("ResolveOID returned no error for empty tree")
	}
}

func TestOIDNode(t *testing.T) {
	// Test OIDNode creation
	node := &OIDNode{
		OID:         "1.3.6.1.2.1.1.1.0",
		Name:        "sysDescr",
		Description: "System Description",
		Syntax:      "DisplayString",
		Access:      "read-only",
		Status:      "current",
		MIBName:     "SNMPv2-MIB",
		Children:    make(map[string]*OIDNode),
	}

	if node.OID != "1.3.6.1.2.1.1.1.0" {
		t.Errorf("Expected OID '1.3.6.1.2.1.1.1.0', got '%s'", node.OID)
	}

	if node.Name != "sysDescr" {
		t.Errorf("Expected name 'sysDescr', got '%s'", node.Name)
	}

	if node.Children == nil {
		t.Error("Children map should not be nil")
	}
}

// Additional tests for the consolidated MIB manager functionality
func TestMIBInfo(t *testing.T) {
	mib := &MIBInfo{
		Name:        "TEST-MIB",
		Description: "Test MIB for unit testing",
		Objects:     make(map[string]*OIDNode),
		Imports:     make(map[string]string),
	}

	if mib.Name != "TEST-MIB" {
		t.Errorf("Expected name 'TEST-MIB', got '%s'", mib.Name)
	}

	if mib.Objects == nil {
		t.Error("Objects map should not be nil")
	}

	if mib.Imports == nil {
		t.Error("Imports map should not be nil")
	}
}
