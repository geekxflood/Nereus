package loader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockConfigProvider implements the config.Provider interface for testing.
type mockConfigProvider struct {
	values map[string]interface{}
}

func newMockConfigProvider() *mockConfigProvider {
	return &mockConfigProvider{
		values: map[string]interface{}{
			"mib.directories":      []string{"./testdata"},
			"mib.file_extensions":  []string{".mib", ".txt"},
			"mib.max_file_size":    1024 * 1024,
			"mib.enable_hot_reload": false,
			"mib.cache_enabled":    true,
			"mib.cache_expiry":     "1h",
		},
	}
}

func (m *mockConfigProvider) GetString(path string, defaultValue ...string) (string, error) {
	if val, exists := m.values[path]; exists {
		if str, ok := val.(string); ok {
			return str, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return "", nil
}

func (m *mockConfigProvider) GetInt(path string, defaultValue ...int) (int, error) {
	if val, exists := m.values[path]; exists {
		if i, ok := val.(int); ok {
			return i, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetFloat(path string, defaultValue ...float64) (float64, error) {
	if val, exists := m.values[path]; exists {
		if f, ok := val.(float64); ok {
			return f, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetBool(path string, defaultValue ...bool) (bool, error) {
	if val, exists := m.values[path]; exists {
		if b, ok := val.(bool); ok {
			return b, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return false, nil
}

func (m *mockConfigProvider) GetDuration(path string, defaultValue ...time.Duration) (time.Duration, error) {
	if val, exists := m.values[path]; exists {
		if str, ok := val.(string); ok {
			return time.ParseDuration(str)
		}
		if d, ok := val.(time.Duration); ok {
			return d, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
}

func (m *mockConfigProvider) GetStringSlice(path string, defaultValue ...[]string) ([]string, error) {
	if val, exists := m.values[path]; exists {
		if slice, ok := val.([]string); ok {
			return slice, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return nil, nil
}

func (m *mockConfigProvider) GetMap(path string) (map[string]any, error) {
	if val, exists := m.values[path]; exists {
		if m, ok := val.(map[string]any); ok {
			return m, nil
		}
	}
	return nil, nil
}

func (m *mockConfigProvider) Exists(path string) bool {
	_, exists := m.values[path]
	return exists
}

func (m *mockConfigProvider) Validate() error {
	return nil
}

func TestNewLoader(t *testing.T) {
	cfg := newMockConfigProvider()
	
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	if loader == nil {
		t.Fatal("Loader is nil")
	}
	
	if loader.config == nil {
		t.Error("Config not set")
	}
	
	if loader.files == nil {
		t.Error("Files map not initialized")
	}
	
	if loader.stats == nil {
		t.Error("Stats not initialized")
	}
}

func TestNewLoaderNilConfig(t *testing.T) {
	_, err := NewLoader(nil)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
	
	expectedMsg := "configuration provider cannot be nil"
	if err.Error() != expectedMsg {
		t.Errorf("Expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestDefaultLoaderConfig(t *testing.T) {
	config := DefaultLoaderConfig()
	
	if config == nil {
		t.Fatal("Config is nil")
	}
	
	if len(config.MIBDirectories) == 0 {
		t.Error("No default MIB directories")
	}
	
	if len(config.FileExtensions) == 0 {
		t.Error("No default file extensions")
	}
	
	if config.MaxFileSize <= 0 {
		t.Error("Invalid max file size")
	}
	
	if len(config.RequiredMIBs) == 0 {
		t.Error("No required MIBs specified")
	}
}

func TestValidateRequiredMIBs(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	// Test with no files loaded
	err = loader.validateRequiredMIBs()
	if err == nil {
		t.Error("Expected error for missing required MIBs")
	}
	
	// Add a required MIB
	loader.files["test.mib"] = &MIBFile{
		Name:     "SNMPv2-SMI",
		ParsedOK: true,
	}
	
	// Should still fail as not all required MIBs are present
	err = loader.validateRequiredMIBs()
	if err == nil {
		t.Error("Expected error for incomplete required MIBs")
	}
}

func TestHasValidExtension(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	testCases := []struct {
		path     string
		expected bool
	}{
		{"test.mib", true},
		{"test.txt", true},
		{"test.MIB", true},
		{"test.TXT", true},
		{"test.json", false},
		{"test", false},
		{"test.bak", false},
	}
	
	for _, tc := range testCases {
		result := loader.hasValidExtension(tc.path)
		if result != tc.expected {
			t.Errorf("hasValidExtension(%s) = %v, expected %v", tc.path, result, tc.expected)
		}
	}
}

func TestShouldIgnoreFile(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	testCases := []struct {
		path     string
		expected bool
	}{
		{".hidden", true},
		{"_temp", true},
		{"test.bak", true},
		{"test.tmp", true},
		{"normal.mib", false},
		{"test.txt", false},
	}
	
	for _, tc := range testCases {
		result := loader.shouldIgnoreFile(tc.path)
		if result != tc.expected {
			t.Errorf("shouldIgnoreFile(%s) = %v, expected %v", tc.path, result, tc.expected)
		}
	}
}

func TestExtractMIBName(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	testCases := []struct {
		path     string
		content  []byte
		expected string
	}{
		{"/path/to/SNMPv2-SMI.mib", []byte{}, "SNMPV2-SMI"},
		{"/path/to/rfc1213.mib", []byte{}, "1213"},
		{"/path/to/test-mib.txt", []byte{}, "TEST"},
		{"/path/to/enterprise_mib.my", []byte{}, "ENTERPRISE"},
	}
	
	for _, tc := range testCases {
		result := loader.extractMIBName(tc.path, tc.content)
		if result != tc.expected {
			t.Errorf("extractMIBName(%s) = %s, expected %s", tc.path, result, tc.expected)
		}
	}
}

func TestValidateMIBContent(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	validMIB := []byte(`
TEST-MIB DEFINITIONS ::= BEGIN
IMPORTS
    MODULE-IDENTITY FROM SNMPv2-SMI;

testMIB MODULE-IDENTITY
    LAST-UPDATED "202301010000Z"
    ORGANIZATION "Test Org"
    CONTACT-INFO "test@example.com"
    DESCRIPTION "Test MIB"
    ::= { enterprises 12345 }

END
`)
	
	invalidMIB := []byte(`
This is not a valid MIB file
`)
	
	// Test valid MIB
	err = loader.validateMIBContent(validMIB)
	if err != nil {
		t.Errorf("Valid MIB failed validation: %v", err)
	}
	
	// Test invalid MIB
	err = loader.validateMIBContent(invalidMIB)
	if err == nil {
		t.Error("Invalid MIB passed validation")
	}
}

func TestGetStats(t *testing.T) {
	cfg := newMockConfigProvider()
	loader, err := NewLoader(cfg)
	if err != nil {
		t.Fatalf("Failed to create loader: %v", err)
	}
	
	stats := loader.GetStats()
	if stats == nil {
		t.Fatal("Stats is nil")
	}
	
	// Check that stats structure is properly initialized
	if stats.FilesLoaded < 0 {
		t.Error("Invalid FilesLoaded count")
	}
	
	if stats.ParseErrors < 0 {
		t.Error("Invalid ParseErrors count")
	}
}
