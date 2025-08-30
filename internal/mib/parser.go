// Package mib provides MIB parsing and OID tree construction functionality.
package mib

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/geekxflood/nereus/internal/loader"
)

// OIDNode represents a node in the OID tree
type OIDNode struct {
	OID         string            `json:"oid"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Syntax      string            `json:"syntax,omitempty"`
	Access      string            `json:"access,omitempty"`
	Status      string            `json:"status,omitempty"`
	Parent      *OIDNode          `json:"-"` // Don't serialize to avoid cycles
	Children    map[string]*OIDNode `json:"children,omitempty"`
	MIBName     string            `json:"mib_name"`
	IsTable     bool              `json:"is_table,omitempty"`
	IsEntry     bool              `json:"is_entry,omitempty"`
	Index       []string          `json:"index,omitempty"`
}

// MIBInfo represents parsed MIB information
type MIBInfo struct {
	Name         string            `json:"name"`
	Description  string            `json:"description,omitempty"`
	Organization string            `json:"organization,omitempty"`
	ContactInfo  string            `json:"contact_info,omitempty"`
	LastUpdated  string            `json:"last_updated,omitempty"`
	Revision     string            `json:"revision,omitempty"`
	Imports      map[string]string `json:"imports,omitempty"`
	Objects      map[string]*OIDNode `json:"objects,omitempty"`
	FilePath     string            `json:"file_path"`
}

// Parser handles MIB parsing and OID tree construction
type Parser struct {
	loader    *loader.Loader
	oidTree   *OIDNode
	mibs      map[string]*MIBInfo
	nameToOID map[string]string
	oidToName map[string]string
	mu        sync.RWMutex
	stats     *ParserStats
}

// ParserStats tracks parser statistics
type ParserStats struct {
	MIBsParsed     int `json:"mibs_parsed"`
	ObjectsParsed  int `json:"objects_parsed"`
	ParseErrors    int `json:"parse_errors"`
	TreeDepth      int `json:"tree_depth"`
	TotalNodes     int `json:"total_nodes"`
	TablesFound    int `json:"tables_found"`
	EntriesFound   int `json:"entries_found"`
}

// Regular expressions for MIB parsing
var (
	mibNameRegex     = regexp.MustCompile(`^([A-Z][A-Z0-9-]*)\s+DEFINITIONS\s*::=\s*BEGIN`)
	objectRegex      = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9-]*)\s+OBJECT-TYPE`)
	oidRegex         = regexp.MustCompile(`::=\s*\{\s*([^}]+)\s*\}`)
	syntaxRegex      = regexp.MustCompile(`SYNTAX\s+([^\n\r]+)`)
	accessRegex      = regexp.MustCompile(`ACCESS\s+([^\n\r]+)`)
	statusRegex      = regexp.MustCompile(`STATUS\s+([^\n\r]+)`)
	descriptionRegex = regexp.MustCompile(`DESCRIPTION\s+"([^"]*)"`)
	indexRegex       = regexp.MustCompile(`INDEX\s*\{\s*([^}]+)\s*\}`)
	importsRegex     = regexp.MustCompile(`IMPORTS\s+(.*?);`)
)

// NewParser creates a new MIB parser
func NewParser(l *loader.Loader) *Parser {
	return &Parser{
		loader:    l,
		oidTree:   createRootNode(),
		mibs:      make(map[string]*MIBInfo),
		nameToOID: make(map[string]string),
		oidToName: make(map[string]string),
		stats:     &ParserStats{},
	}
}

// createRootNode creates the root node of the OID tree
func createRootNode() *OIDNode {
	root := &OIDNode{
		OID:      "",
		Name:     "root",
		Children: make(map[string]*OIDNode),
	}

	// Add standard root nodes
	iso := &OIDNode{
		OID:      "1",
		Name:     "iso",
		Parent:   root,
		Children: make(map[string]*OIDNode),
	}
	root.Children["1"] = iso

	org := &OIDNode{
		OID:      "1.3",
		Name:     "org",
		Parent:   iso,
		Children: make(map[string]*OIDNode),
	}
	iso.Children["3"] = org

	dod := &OIDNode{
		OID:      "1.3.6",
		Name:     "dod",
		Parent:   org,
		Children: make(map[string]*OIDNode),
	}
	org.Children["6"] = dod

	internet := &OIDNode{
		OID:      "1.3.6.1",
		Name:     "internet",
		Parent:   dod,
		Children: make(map[string]*OIDNode),
	}
	dod.Children["1"] = internet

	// Add standard internet subtrees
	mgmt := &OIDNode{
		OID:      "1.3.6.1.2",
		Name:     "mgmt",
		Parent:   internet,
		Children: make(map[string]*OIDNode),
	}
	internet.Children["2"] = mgmt

	mib2 := &OIDNode{
		OID:      "1.3.6.1.2.1",
		Name:     "mib-2",
		Parent:   mgmt,
		Children: make(map[string]*OIDNode),
	}
	mgmt.Children["1"] = mib2

	private := &OIDNode{
		OID:      "1.3.6.1.4",
		Name:     "private",
		Parent:   internet,
		Children: make(map[string]*OIDNode),
	}
	internet.Children["4"] = private

	enterprises := &OIDNode{
		OID:      "1.3.6.1.4.1",
		Name:     "enterprises",
		Parent:   private,
		Children: make(map[string]*OIDNode),
	}
	private.Children["1"] = enterprises

	return root
}

// ParseAll parses all loaded MIB files
func (p *Parser) ParseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	files := p.loader.GetValidFiles()
	p.stats.MIBsParsed = 0
	p.stats.ObjectsParsed = 0
	p.stats.ParseErrors = 0

	for _, file := range files {
		if err := p.parseMIBFile(file); err != nil {
			p.stats.ParseErrors++
			// Log error but continue with other files
			continue
		}
		p.stats.MIBsParsed++
	}

	// Build cross-references and resolve dependencies
	p.buildCrossReferences()
	p.calculateStats()

	return nil
}

// parseMIBFile parses a single MIB file
func (p *Parser) parseMIBFile(file *loader.MIBFile) error {
	content := string(file.Content)

	// Extract MIB name
	nameMatch := mibNameRegex.FindStringSubmatch(content)
	if len(nameMatch) < 2 {
		return fmt.Errorf("could not extract MIB name from %s", file.Path)
	}

	mibName := nameMatch[1]
	mibInfo := &MIBInfo{
		Name:     mibName,
		FilePath: file.Path,
		Objects:  make(map[string]*OIDNode),
		Imports:  make(map[string]string),
	}

	// Extract imports
	p.parseImports(content, mibInfo)

	// Extract MIB metadata
	p.parseMIBMetadata(content, mibInfo)

	// Parse OBJECT-TYPE definitions
	if err := p.parseObjects(content, mibInfo); err != nil {
		return fmt.Errorf("failed to parse objects in %s: %w", file.Path, err)
	}

	p.mibs[mibName] = mibInfo
	return nil
}

// parseImports extracts import statements from MIB content
func (p *Parser) parseImports(content string, mibInfo *MIBInfo) {
	matches := importsRegex.FindStringSubmatch(content)
	if len(matches) < 2 {
		return
	}

	importSection := matches[1]
	lines := strings.Split(importSection, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		// Simple parsing - look for "FROM ModuleName"
		if strings.Contains(line, " FROM ") {
			parts := strings.Split(line, " FROM ")
			if len(parts) == 2 {
				symbols := strings.TrimSpace(parts[0])
				module := strings.TrimSpace(parts[1])
				module = strings.TrimSuffix(module, ",")
				mibInfo.Imports[symbols] = module
			}
		}
	}
}

// parseMIBMetadata extracts metadata from MIB content
func (p *Parser) parseMIBMetadata(content string, mibInfo *MIBInfo) {
	// Extract organization
	if match := regexp.MustCompile(`ORGANIZATION\s+"([^"]*)"`)FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Organization = match[1]
	}

	// Extract contact info
	if match := regexp.MustCompile(`CONTACT-INFO\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.ContactInfo = match[1]
	}

	// Extract description
	if match := regexp.MustCompile(`DESCRIPTION\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Description = match[1]
	}

	// Extract last updated
	if match := regexp.MustCompile(`LAST-UPDATED\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.LastUpdated = match[1]
	}

	// Extract revision
	if match := regexp.MustCompile(`REVISION\s+"([^"]*)"`).FindStringSubmatch(content); len(match) > 1 {
		mibInfo.Revision = match[1]
	}
}

// parseObjects extracts OBJECT-TYPE definitions from MIB content
func (p *Parser) parseObjects(content string, mibInfo *MIBInfo) error {
	// Find all OBJECT-TYPE definitions
	objectMatches := objectRegex.FindAllStringSubmatch(content, -1)

	for _, match := range objectMatches {
		if len(match) < 2 {
			continue
		}

		objectName := match[1]
		if err := p.parseObjectDefinition(content, objectName, mibInfo); err != nil {
			// Log error but continue with other objects
			continue
		}
		p.stats.ObjectsParsed++
	}

	return nil
}

// resolveOID resolves an OID string to numeric form
func (p *Parser) resolveOID(oidStr string, mibInfo *MIBInfo) string {
	// Handle numeric OIDs
	if strings.Contains(oidStr, ".") {
		return oidStr
	}

	// Handle symbolic OIDs like "{ mib-2 1 }"
	oidStr = strings.Trim(oidStr, "{}")
	parts := strings.Fields(oidStr)

	var oidParts []string
	for _, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			oidParts = append(oidParts, strconv.Itoa(num))
		} else {
			// Try to resolve symbolic name
			if resolved, exists := p.nameToOID[part]; exists {
				oidParts = append(oidParts, resolved)
			} else {
				// Use well-known mappings
				switch part {
				case "iso":
					oidParts = append(oidParts, "1")
				case "org":
					oidParts = append(oidParts, "1.3")
				case "dod":
					oidParts = append(oidParts, "1.3.6")
				case "internet":
					oidParts = append(oidParts, "1.3.6.1")
				case "mgmt":
					oidParts = append(oidParts, "1.3.6.1.2")
				case "mib-2":
					oidParts = append(oidParts, "1.3.6.1.2.1")
				case "private":
					oidParts = append(oidParts, "1.3.6.1.4")
				case "enterprises":
					oidParts = append(oidParts, "1.3.6.1.4.1")
				default:
					oidParts = append(oidParts, part) // Keep as-is for later resolution
				}
			}
		}
	}

	return strings.Join(oidParts, ".")
}

// addToTree adds an OID node to the tree structure
func (p *Parser) addToTree(node *OIDNode) {
	if node.OID == "" {
		return
	}

	parts := strings.Split(node.OID, ".")
	current := p.oidTree

	// Navigate to the correct position in the tree
	for i, part := range parts {
		if part == "" {
			continue
		}

		if child, exists := current.Children[part]; exists {
			current = child
		} else {
			// Create intermediate nodes if they don't exist
			newNode := &OIDNode{
				OID:      strings.Join(parts[:i+1], "."),
				Name:     fmt.Sprintf("node_%s", part),
				Parent:   current,
				Children: make(map[string]*OIDNode),
			}
			current.Children[part] = newNode
			current = newNode
		}
	}

	// Update the final node with the parsed information
	if current != p.oidTree {
		current.Name = node.Name
		current.Description = node.Description
		current.Syntax = node.Syntax
		current.Access = node.Access
		current.Status = node.Status
		current.MIBName = node.MIBName
		current.IsTable = node.IsTable
		current.IsEntry = node.IsEntry
		current.Index = node.Index
	}
}

// buildCrossReferences builds cross-references between MIB objects
func (p *Parser) buildCrossReferences() {
	// This is a simplified implementation
	// In a full implementation, this would resolve all symbolic references
	for _, mib := range p.mibs {
		for _, obj := range mib.Objects {
			if obj.OID != "" {
				p.nameToOID[obj.Name] = obj.OID
				p.oidToName[obj.OID] = obj.Name
			}
		}
	}
}

// calculateStats calculates tree statistics
func (p *Parser) calculateStats() {
	p.stats.TotalNodes = p.countNodes(p.oidTree)
	p.stats.TreeDepth = p.calculateDepth(p.oidTree, 0)
}

// countNodes counts total nodes in the tree
func (p *Parser) countNodes(node *OIDNode) int {
	count := 1
	for _, child := range node.Children {
		count += p.countNodes(child)
	}
	return count
}

// calculateDepth calculates maximum tree depth
func (p *Parser) calculateDepth(node *OIDNode, currentDepth int) int {
	maxDepth := currentDepth
	for _, child := range node.Children {
		depth := p.calculateDepth(child, currentDepth+1)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// GetMIB returns MIB information by name
func (p *Parser) GetMIB(name string) (*MIBInfo, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	mib, exists := p.mibs[name]
	return mib, exists
}

// GetAllMIBs returns all parsed MIB information
func (p *Parser) GetAllMIBs() map[string]*MIBInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]*MIBInfo)
	for name, mib := range p.mibs {
		result[name] = mib
	}
	return result
}

// GetOIDTree returns the root of the OID tree
func (p *Parser) GetOIDTree() *OIDNode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.oidTree
}

// GetStats returns parser statistics
func (p *Parser) GetStats() *ParserStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := *p.stats
	return &stats
}

// ResolveOID resolves a numeric OID to its symbolic name
func (p *Parser) ResolveOID(oid string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	name, exists := p.oidToName[oid]
	return name, exists
}

// ResolveName resolves a symbolic name to its numeric OID
func (p *Parser) ResolveName(name string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	oid, exists := p.nameToOID[name]
	return oid, exists
}

// FindNode finds an OID node by numeric OID
func (p *Parser) FindNode(oid string) (*OIDNode, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	parts := strings.Split(oid, ".")
	current := p.oidTree

	for _, part := range parts {
		if part == "" {
			continue
		}

		if child, exists := current.Children[part]; exists {
			current = child
		} else {
			return nil, false
		}
	}

	return current, true
}

// GetNodeInfo returns detailed information about an OID node
func (p *Parser) GetNodeInfo(oid string) (*OIDNode, bool) {
	return p.FindNode(oid)
}


