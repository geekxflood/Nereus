package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geekxflood/nereus/internal/mib"
)

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
	return 0, nil
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

func (m *MockConfigProvider) GetDuration(key string, defaultValue ...time.Duration) (time.Duration, error) {
	if value, exists := m.data[key]; exists {
		if d, ok := value.(time.Duration); ok {
			return d, nil
		}
	}
	if len(defaultValue) > 0 {
		return defaultValue[0], nil
	}
	return 0, nil
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

func main() {
	fmt.Println("=== Nereus MIB Loading Test ===")

	// Check if mibs directory exists
	mibsDir := "./mibs"
	if _, err := os.Stat(mibsDir); os.IsNotExist(err) {
		log.Fatalf("MIBs directory does not exist: %s", mibsDir)
	}

	// List MIB files
	fmt.Printf("\n1. Scanning MIB directory: %s\n", mibsDir)
	err := filepath.Walk(mibsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".mib" || ext == ".txt" || ext == ".my" {
				fmt.Printf("   Found MIB file: %s (size: %d bytes)\n", path, info.Size())
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Error scanning MIB directory: %v", err)
	}

	// Create mock config with empty required MIBs
	mockConfig := &MockConfigProvider{
		data: map[string]interface{}{
			"mib.directories":       []string{"./mibs"},
			"mib.file_extensions":   []string{".mib", ".txt", ".my"},
			"mib.enable_hot_reload": false,
			"mib.cache_enabled":     true,
			"mib.required_mibs":     []string{}, // Empty list to bypass validation
		},
	}

	fmt.Printf("\n2. Creating MIB manager...\n")
	manager, err := mib.NewManager(mockConfig)
	if err != nil {
		log.Fatalf("Failed to create MIB manager: %v", err)
	}

	fmt.Printf("3. Loading MIB files...\n")
	err = manager.LoadAll()
	if err != nil {
		log.Fatalf("Failed to load MIB files: %v", err)
	}

	// Get statistics
	stats := manager.GetStats()
	fmt.Printf("\n=== MIB Loading Results ===\n")
	fmt.Printf("Directories scanned: %d\n", stats.DirectoriesScanned)
	fmt.Printf("Files loaded: %d\n", stats.FilesLoaded)
	fmt.Printf("MIBs parsed: %d\n", stats.MIBsParsed)
	fmt.Printf("Objects parsed: %d\n", stats.ObjectsParsed)
	fmt.Printf("Parse errors: %d\n", stats.ParseErrors)
	fmt.Printf("Total file size: %d bytes\n", stats.TotalFileSize)
	fmt.Printf("Scan duration: %v\n", stats.ScanDuration)

	// List loaded MIBs
	fmt.Printf("\n=== Loaded MIBs ===\n")
	allMibs := manager.GetAllMIBs()
	for name, mibInfo := range allMibs {
		fmt.Printf("MIB: %s (file: %s, objects: %d)\n", name, mibInfo.FilePath, len(mibInfo.Objects))
	}

	// Test OID resolution
	fmt.Printf("\n=== OID Resolution Test ===\n")
	testOIDs := []string{
		"1.3.6.1.6.3.1.1.5.1", // coldStart
		"1.3.6.1.6.3.1.1.5.2", // warmStart
		"1.3.6.1.6.3.1.1.5.3", // linkDown
		"1.3.6.1.6.3.1.1.5.4", // linkUp
	}

	for _, oid := range testOIDs {
		name, found := manager.ResolveName(oid)
		if !found {
			fmt.Printf("OID %s -> NOT FOUND\n", oid)
		} else {
			fmt.Printf("OID %s -> %s\n", oid, name)
		}
	}

	fmt.Printf("\n=== Test Completed Successfully ===\n")
}
