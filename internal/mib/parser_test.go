package mib

import (
	"testing"

	"github.com/geekxflood/nereus/internal/loader"
)

func TestNewParser(t *testing.T) {
	// Create a mock loader
	mockLoader := &loader.Loader{}
	
	parser := NewParser(mockLoader)
	if parser == nil {
		t.Fatal("Parser is nil")
	}
	
	if parser.loader != mockLoader {
		t.Error("Loader not set correctly")
	}
	
	if parser.oidTree == nil {
		t.Error("OID tree not initialized")
	}
	
	if parser.mibs == nil {
		t.Error("MIBs map not initialized")
	}
	
	if parser.nameToOID == nil {
		t.Error("Name to OID map not initialized")
	}
	
	if parser.oidToName == nil {
		t.Error("OID to name map not initialized")
	}
	
	if parser.stats == nil {
		t.Error("Stats not initialized")
	}
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

func TestResolveOID(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	testCases := []struct {
		input    string
		expected string
	}{
		{"1.3.6.1.2.1", "1.3.6.1.2.1"},           // Numeric OID
		{"{ iso 3 6 1 2 1 }", "1.3.6.1.2.1"},     // Symbolic with iso
		{"{ mib-2 1 }", "1.3.6.1.2.1.1"},         // Symbolic with mib-2
		{"{ enterprises 12345 }", "1.3.6.1.4.1.12345"}, // Symbolic with enterprises
	}
	
	for _, tc := range testCases {
		result := parser.resolveOID(tc.input, &MIBInfo{})
		if result != tc.expected {
			t.Errorf("resolveOID(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestAddToTree(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	// Create a test node
	node := &OIDNode{
		OID:         "1.3.6.1.2.1.1.1.0",
		Name:        "sysDescr",
		Description: "System Description",
		Syntax:      "DisplayString",
		Access:      "read-only",
		Status:      "current",
		MIBName:     "SNMPv2-MIB",
	}
	
	// Add to tree
	parser.addToTree(node)
	
	// Verify the node was added correctly
	foundNode, exists := parser.FindNode("1.3.6.1.2.1.1.1.0")
	if !exists {
		t.Error("Node not found in tree")
	} else {
		if foundNode.Name != "sysDescr" {
			t.Errorf("Expected name 'sysDescr', got '%s'", foundNode.Name)
		}
		if foundNode.Description != "System Description" {
			t.Errorf("Expected description 'System Description', got '%s'", foundNode.Description)
		}
		if foundNode.Syntax != "DisplayString" {
			t.Errorf("Expected syntax 'DisplayString', got '%s'", foundNode.Syntax)
		}
	}
}

func TestFindNode(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	// Test finding existing nodes
	node, exists := parser.FindNode("1.3.6.1")
	if !exists {
		t.Error("Internet node not found")
	} else if node.Name != "internet" {
		t.Errorf("Expected name 'internet', got '%s'", node.Name)
	}
	
	// Test finding non-existing node
	_, exists = parser.FindNode("1.2.3.4.5.6.7.8.9")
	if exists {
		t.Error("Non-existing node found")
	}
}

func TestCountNodes(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	count := parser.countNodes(parser.oidTree)
	if count <= 0 {
		t.Error("Invalid node count")
	}
	
	// Should have at least the standard nodes
	if count < 7 { // root, iso, org, dod, internet, mgmt, private, enterprises
		t.Errorf("Expected at least 7 nodes, got %d", count)
	}
}

func TestCalculateDepth(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	depth := parser.calculateDepth(parser.oidTree, 0)
	if depth <= 0 {
		t.Error("Invalid tree depth")
	}
	
	// Should have at least depth 5 for the standard tree (root -> iso -> org -> dod -> internet -> mgmt/private)
	if depth < 5 {
		t.Errorf("Expected at least depth 5, got %d", depth)
	}
}

func TestGetStats(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	stats := parser.GetStats()
	if stats == nil {
		t.Fatal("Stats is nil")
	}
	
	// Check that stats structure is properly initialized
	if stats.MIBsParsed < 0 {
		t.Error("Invalid MIBsParsed count")
	}
	
	if stats.ObjectsParsed < 0 {
		t.Error("Invalid ObjectsParsed count")
	}
	
	if stats.ParseErrors < 0 {
		t.Error("Invalid ParseErrors count")
	}
}

func TestBuildCrossReferences(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	// Add a test MIB with objects
	testMIB := &MIBInfo{
		Name:    "TEST-MIB",
		Objects: make(map[string]*OIDNode),
	}
	
	testObj := &OIDNode{
		OID:  "1.3.6.1.4.1.12345.1",
		Name: "testObject",
	}
	
	testMIB.Objects["testObject"] = testObj
	parser.mibs["TEST-MIB"] = testMIB
	
	// Build cross-references
	parser.buildCrossReferences()
	
	// Check that mappings were created
	if oid, exists := parser.nameToOID["testObject"]; !exists || oid != "1.3.6.1.4.1.12345.1" {
		t.Error("Name to OID mapping not created correctly")
	}
	
	if name, exists := parser.oidToName["1.3.6.1.4.1.12345.1"]; !exists || name != "testObject" {
		t.Error("OID to name mapping not created correctly")
	}
}

func TestResolveOIDAndName(t *testing.T) {
	mockLoader := &loader.Loader{}
	parser := NewParser(mockLoader)
	
	// Add test mappings
	parser.nameToOID["testObject"] = "1.3.6.1.4.1.12345.1"
	parser.oidToName["1.3.6.1.4.1.12345.1"] = "testObject"
	
	// Test OID resolution
	name, exists := parser.ResolveOID("1.3.6.1.4.1.12345.1")
	if !exists || name != "testObject" {
		t.Error("OID resolution failed")
	}
	
	// Test name resolution
	oid, exists := parser.ResolveName("testObject")
	if !exists || oid != "1.3.6.1.4.1.12345.1" {
		t.Error("Name resolution failed")
	}
	
	// Test non-existing OID
	_, exists = parser.ResolveOID("1.2.3.4.5")
	if exists {
		t.Error("Non-existing OID resolved")
	}
	
	// Test non-existing name
	_, exists = parser.ResolveName("nonExistingObject")
	if exists {
		t.Error("Non-existing name resolved")
	}
}
