// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains exporters for converting WFM data to PNG images and YAML dialogue files.
package pkg

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/hansbonini/tombatools/pkg/psx"
	"gopkg.in/yaml.v3"
)

// WFMFileExporter implements the WFMExporter interface and provides
// functionality to export WFM data to external formats (PNG, YAML).
type WFMFileExporter struct{}

// NewWFMExporter creates a new WFM exporter instance.
// Returns a pointer to a WFMFileExporter ready for use.
func NewWFMExporter() *WFMFileExporter {
	return &WFMFileExporter{}
}

// ExportGlyphs exports each glyph as an individual PNG file.
// This function processes all glyphs in the WFM file and creates separate PNG images
// for each glyph in a "glyphs" subdirectory within the output directory.
// Parameters:
//   - wfm: The WFM file containing glyph data to export
//   - outputDir: Base directory path where the "glyphs" subdirectory will be created
//
// Returns an error if the export operation fails (directory creation, file writing, etc.).
func (e *WFMFileExporter) ExportGlyphs(wfm *WFMFile, outputDir string) error {
	glyphsDir := filepath.Join(outputDir, "glyphs")
	if err := os.MkdirAll(glyphsDir, 0o750); err != nil {
		return fmt.Errorf("failed to create glyphs directory: %w", err)
	}

	if err := e.validateGlyphCount(wfm); err != nil {
		return err
	}

	exportedCount := e.exportAllGlyphs(wfm, glyphsDir)
	common.LogInfo(common.InfoGlyphsExported, exportedCount, glyphsDir)
	return nil
}

// validateGlyphCount validates that the expected number of glyphs matches actual count
func (e *WFMFileExporter) validateGlyphCount(wfm *WFMFile) error {
	expectedGlyphs := int(wfm.Header.TotalGlyphs)
	actualGlyphs := len(wfm.Glyphs)
	if actualGlyphs != expectedGlyphs {
		return fmt.Errorf("glyph count mismatch: expected %d, got %d", expectedGlyphs, actualGlyphs)
	}
	return nil
}

// exportAllGlyphs exports all valid glyphs and returns the count of exported glyphs
func (e *WFMFileExporter) exportAllGlyphs(wfm *WFMFile, glyphsDir string) int {
	exportedCount := 0

	for glyphIndex, glyph := range wfm.Glyphs {
		if e.exportSingleGlyph(glyphIndex, glyph, glyphsDir) {
			exportedCount++
		}
	}

	return exportedCount
}

// exportSingleGlyph exports a single glyph as PNG and returns true if successful
func (e *WFMFileExporter) exportSingleGlyph(glyphIndex int, glyph Glyph, glyphsDir string) bool {
	// Skip invalid glyphs
	if !e.isValidGlyph(glyph) {
		common.LogDebug(common.DebugGlyphSkipped, glyphIndex)
		return false
	}

	glyphImg, err := e.convertGlyphToImage(glyph)
	if err != nil {
		common.LogWarn("Failed to convert glyph %d to image: %v", glyphIndex, err)
		return false
	}

	filename := fmt.Sprintf("glyph_%04d.png", glyphIndex)
	if err := e.saveGlyphImage(glyphImg, glyphsDir, filename, glyphIndex); err != nil {
		return false
	}

	common.LogDebug(common.DebugGlyphExported,
		glyphIndex, glyph.GlyphWidth, glyph.GlyphHeight,
		glyph.GlyphClut, glyph.GlyphHandakuten, filename)
	return true
}

// isValidGlyph checks if a glyph has valid data for export
func (e *WFMFileExporter) isValidGlyph(glyph Glyph) bool {
	return len(glyph.GlyphImage) > 0 && glyph.GlyphWidth > 0 && glyph.GlyphHeight > 0
}

// convertGlyphToImage converts glyph data to image
func (e *WFMFileExporter) convertGlyphToImage(glyph Glyph) (image.Image, error) {
	width := int(glyph.GlyphWidth)
	height := int(glyph.GlyphHeight)

	palette := e.selectPalette(glyph)

	tile := &psx.PSXTile{
		Width:   width,
		Height:  height,
		Data:    glyph.GlyphImage,
		Palette: palette,
	}

	processor := psx.NewPSXTileProcessor()
	return processor.ConvertFromTile(tile)
}

// selectPalette selects the appropriate palette based on glyph height
func (e *WFMFileExporter) selectPalette(glyph Glyph) psx.PSXPalette {
	if glyph.GlyphHeight == 24 {
		// Use EventClut for glyphs with height 24
		return psx.NewPSXPalette(EventClut)
	}
	// Use DialogueClut for all other heights
	return psx.NewPSXPalette(DialogueClut)
}

// saveGlyphImage saves the glyph image as PNG file
func (e *WFMFileExporter) saveGlyphImage(glyphImg image.Image, glyphsDir, filename string, glyphIndex int) error {
	pngFile := filepath.Join(glyphsDir, filename)
	file, err := os.Create(pngFile)
	if err != nil {
		return fmt.Errorf("failed to create PNG file for glyph %d: %w", glyphIndex, err)
	}
	defer file.Close()

	if err := png.Encode(file, glyphImg); err != nil {
		return fmt.Errorf("failed to encode PNG for glyph %d: %w", glyphIndex, err)
	}

	return nil
}

// DialoguesYAML represents the complete dialogues structure for YAML export
type DialoguesYAML struct {
	TotalDialogues int             `yaml:"total_dialogues"`
	OriginalSize   int64           `yaml:"original_size"`
	Dialogues      []DialogueEntry `yaml:"dialogues"`
}

// processDialogueText processes dialogue text using the new content-based structure
func processDialogueText(rawData []byte, glyphMapping map[uint16]string, glyphs []Glyph) ([]map[string]interface{}, string, int, uint16, uint16) {
	processor := &dialogueTextProcessor{
		content:            make([]map[string]interface{}, 0),
		currentText:        "",
		entryType:          "event",
		detectedFontHeight: 8,
		detectedFontClut:   0,
		terminator:         0xFFFF,
		glyphMapping:       glyphMapping,
		glyphs:             glyphs,
	}

	processor.processRawData(rawData)
	return processor.content, processor.entryType, processor.detectedFontHeight, processor.detectedFontClut, processor.terminator
}

// dialogueTextProcessor handles dialogue text processing
type dialogueTextProcessor struct {
	content            []map[string]interface{}
	currentText        string
	entryType          string
	detectedFontHeight int
	detectedFontClut   uint16
	terminator         uint16
	glyphMapping       map[uint16]string
	glyphs             []Glyph
}

// addTextContent adds current text to content if it exists
func (p *dialogueTextProcessor) addTextContent() {
	if p.currentText != "" {
		p.content = append(p.content, map[string]interface{}{
			"text": p.currentText,
		})
		p.currentText = ""
	}
}

// processRawData processes the raw dialogue data
func (p *dialogueTextProcessor) processRawData(rawData []byte) {
	// Process dialogue data in 2-byte chunks
	for i := 0; i+1 < len(rawData); i += 2 {
		// Read 2 bytes as little endian uint16
		glyphID := binary.LittleEndian.Uint16(rawData[i : i+2])

		// Check for termination
		if glyphID == 0xFFFF || glyphID == 0xFFFE {
			p.terminator = glyphID
			break
		}

		// Handle special commands
		advance, shouldBreak := p.handleSpecialCommands(glyphID, rawData, i)
		if shouldBreak {
			break
		}
		if advance > 0 {
			i += advance
			continue
		}

		// Handle regular glyphs and special characters
		p.handleGlyphOrSpecialChar(glyphID)
	}

	// Add any remaining text
	p.addTextContent()
}

// handleSpecialCommands handles special command processing
func (p *dialogueTextProcessor) handleSpecialCommands(glyphID uint16, rawData []byte, i int) (int, bool) {
	switch glyphID {
	case INIT_TEXT_BOX:
		return p.handleInitTextBox(rawData, i), false
	case INIT_TAIL:
		return p.handleInitTail(rawData, i), false
	case F6:
		return p.handleF6(rawData, i), false
	case CHANGE_COLOR_TO:
		return p.handleChangeColorTo(rawData, i), false
	case PAUSE_FOR:
		return p.handlePauseFor(rawData, i), false
	case FFF2:
		return p.handleFFF2(rawData, i), false
	case TERMINATOR_1, TERMINATOR_2:
		return 0, true
	default:
		return 0, false
	}
}

// handleInitTextBox handles INIT_TEXT_BOX command
func (p *dialogueTextProcessor) handleInitTextBox(rawData []byte, i int) int {
	p.entryType = "dialogue" // Set type to dialogue when INIT TEXT BOX is found
	// Next 2 bytes are width
	if i+4 <= len(rawData) {
		width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		// Next 2 bytes are height
		if i+6 <= len(rawData) {
			height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
			p.content = append(p.content, map[string]interface{}{
				"box": map[string]interface{}{
					"width":  width,
					"height": height,
				},
			})
			return 4 // Skip both width and height bytes
		} else {
			return 2 // Skip only width bytes
		}
	}
	return 0
}

// handleInitTail handles INIT_TAIL command
func (p *dialogueTextProcessor) handleInitTail(rawData []byte, i int) int {
	// Add current text before adding tail
	p.addTextContent()
	// Next 2 bytes are width
	if i+4 <= len(rawData) {
		width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		// Next 2 bytes are height
		if i+6 <= len(rawData) {
			height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
			p.content = append(p.content, map[string]interface{}{
				"tail": map[string]interface{}{
					"width":  width,
					"height": height,
				},
			})
			return 4 // Skip both width and height bytes
		} else {
			return 2 // Skip only width bytes
		}
	}
	return 0
}

// handleF6 handles F6 command
func (p *dialogueTextProcessor) handleF6(rawData []byte, i int) int {
	// Add current text before adding f6
	p.addTextContent()
	// Next 2 bytes are width
	if i+4 <= len(rawData) {
		width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		// Next 2 bytes are height
		if i+6 <= len(rawData) {
			height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
			p.content = append(p.content, map[string]interface{}{
				"f6": map[string]interface{}{
					"width":  width,
					"height": height,
				},
			})
			return 4 // Skip both width and height bytes
		} else {
			return 2 // Skip only width bytes
		}
	}
	return 0
}

// handleChangeColorTo handles CHANGE_COLOR_TO command
func (p *dialogueTextProcessor) handleChangeColorTo(rawData []byte, i int) int {
	// Add current text before changing color
	p.addTextContent()
	// Next 2 bytes are color value
	if i+4 <= len(rawData) {
		colorValue := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		p.content = append(p.content, map[string]interface{}{
			"color": map[string]interface{}{
				"value": colorValue,
			},
		})
		return 2 // Skip color value bytes
	}
	return 0
}

// handlePauseFor handles PAUSE_FOR command
func (p *dialogueTextProcessor) handlePauseFor(rawData []byte, i int) int {
	// Add current text before adding pause
	p.addTextContent()
	// Next 2 bytes are duration
	if i+4 <= len(rawData) {
		duration := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		p.content = append(p.content, map[string]interface{}{
			"pause": map[string]interface{}{
				"duration": duration,
			},
		})
		return 2 // Skip duration bytes
	}
	return 0
}

// handleFFF2 handles FFF2 command
func (p *dialogueTextProcessor) handleFFF2(rawData []byte, i int) int {
	// Add current text before adding fff2
	p.addTextContent()
	// Next 2 bytes are parameter value
	if i+4 <= len(rawData) {
		paramValue := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
		p.content = append(p.content, map[string]interface{}{
			"fff2": map[string]interface{}{
				"value": paramValue,
			},
		})
		return 2 // Skip parameter value bytes
	}
	return 0
}

// handleGlyphOrSpecialChar handles regular glyphs and special characters
func (p *dialogueTextProcessor) handleGlyphOrSpecialChar(glyphID uint16) {
	// Convert to glyph index (subtract GLYPH_ID_BASE)
	if glyphID >= GLYPH_ID_BASE && glyphID <= 0xFFF0 {
		p.handleRegularGlyph(glyphID)
	} else {
		p.handleSpecialCharacter(glyphID)
	}
}

// handleRegularGlyph handles regular glyph processing
func (p *dialogueTextProcessor) handleRegularGlyph(glyphID uint16) {
	actualGlyphID := glyphID - GLYPH_ID_BASE

	// Check glyph height and clut to determine font height and clut
	if p.glyphs != nil && int(actualGlyphID) < len(p.glyphs) {
		glyph := p.glyphs[actualGlyphID]
		if glyph.GlyphHeight == 16 {
			p.detectedFontHeight = 16
		} else if glyph.GlyphHeight == 24 {
			p.detectedFontHeight = 24
		}
		// Update font CLUT from the actual glyph data
		p.detectedFontClut = glyph.GlyphClut
	}

	// Try to decode character
	if p.glyphMapping != nil {
		if char, found := p.glyphMapping[actualGlyphID]; found {
			p.currentText += char
		} else {
			p.handleSpecialGlyphID(glyphID)
		}
	} else {
		p.handleSpecialGlyphID(glyphID)
	}
}

// handleSpecialGlyphID handles special glyph IDs
func (p *dialogueTextProcessor) handleSpecialGlyphID(glyphID uint16) {
	// Special handling for special commands
	if glyphID == C04D {
		p.currentText += TriangleDown
	} else if glyphID == C04E {
		p.currentText += TriangleRight
	} else {
		p.currentText += fmt.Sprintf("[%04X]", glyphID)
	}
}

// handleSpecialCharacter handles special control codes
func (p *dialogueTextProcessor) handleSpecialCharacter(glyphID uint16) {
	switch glyphID {
	case C04D:
		p.currentText += "▼" // Unicode down-pointing triangle for C04D
	case C04E:
		p.currentText += "⏷" // Unicode down-pointing triangle for C04E
	case WAIT_FOR_INPUT:
		p.currentText += "⧗" // Unicode hourglass for WAIT_FOR_INPUT
	case NEWLINE:
		p.currentText += "\n"
	case DOUBLE_NEWLINE:
		p.currentText += "\n\n"
	default:
		specialCode := getSpecialCharacterCode(glyphID)
		p.currentText += specialCode
	}
}

// getSpecialCharacterCode returns the formatted string for special control codes
func getSpecialCharacterCode(code uint16) string {
	// Handle control flow codes
	if controlCode := getControlFlowCode(code); controlCode != "" {
		return controlCode
	}

	// Handle command codes with arguments
	if commandCode := getCommandCode(code); commandCode != "" {
		return commandCode
	}

	// Handle formatting codes
	if formatCode := getFormattingCode(code); formatCode != "" {
		return formatCode
	}

	// Handle unknown codes
	return fmt.Sprintf("<%04X>", code)
}

// getControlFlowCode returns control flow codes like HALT, PROMPT
func getControlFlowCode(code uint16) string {
	switch code {
	case HALT:
		return "[HALT]"
	case PROMPT:
		return "[PROMPT]"
	case WAIT_FOR_INPUT:
		return "[WAIT FOR INPUT]"
	default:
		return ""
	}
}

// getCommandCode returns command codes with arguments
func getCommandCode(code uint16) string {
	switch code {
	case FFF2:
		return "[FFF2]" // args: 1
	case F4:
		return "[F4]"
	case F6:
		return "[F6]" // args: 2
	case CHANGE_COLOR_TO:
		return "[CHANGE COLOR TO]" // args: 1
	case INIT_TAIL:
		return "[INIT TAIL]" // args: 2
	case PAUSE_FOR:
		return "[PAUSE FOR]" // args: 1
	default:
		return ""
	}
}

// getFormattingCode returns formatting codes like newlines and special characters
func getFormattingCode(code uint16) string {
	switch code {
	case DOUBLE_NEWLINE:
		return "\n\n"
	case NEWLINE:
		return "\n" // Convert [NEWLINE] to actual newline
	case C04D:
		return "[C04D]"
	case C04E:
		return "[C04E]"
	default:
		return ""
	}
}

// ExportDialogues exports all dialogue entries from a WFM file to a YAML file.
// This function processes dialogue data, extracts text content with special control codes,
// and exports it as a structured YAML file with metadata.
// Parameters:
//   - wfm: The WFM file containing dialogue data to export
//   - outputDir: Directory path where the "dialogues.yaml" file will be created
//
// Returns an error if the export operation fails (file creation, encoding, etc.).
func (e *WFMFileExporter) ExportDialogues(wfm *WFMFile, outputDir string) error {
	// Validate that we have the expected number of dialogues
	expectedDialogues := int(wfm.Header.TotalDialogues)
	actualDialogues := len(wfm.Dialogues)
	if actualDialogues != expectedDialogues {
		return fmt.Errorf("dialogue count mismatch: expected %d, got %d", expectedDialogues, actualDialogues)
	}

	// Build glyph hash to character mapping from font files for text decoding
	glyphsDir := filepath.Join(outputDir, "glyphs")
	fontDir := "fonts" // User should have a 'fonts' directory with character-named PNG files
	glyphMapping, err := e.buildGlyphMapping(glyphsDir, fontDir)
	if err != nil {
		common.LogWarn(common.WarnCouldNotBuildGlyphMapping, err)
		common.LogWarn(common.WarnDialoguesWithoutDecoding)
	}

	// Process each dialogue using data already extracted in DecodeDialogues
	dialogueEntries := make([]DialogueEntry, 0, len(wfm.Dialogues))
	for i, dialogue := range wfm.Dialogues {
		// Process dialogue text using the new content-based structure
		content, dialogueType, fontHeight, fontClut, terminator := processDialogueText(dialogue.Data, glyphMapping, wfm.Glyphs)

		// Convert terminator from hex value to simple 1 or 2
		var terminatorValue uint16
		switch terminator {
		case 0xFFFE: // TERMINATOR_1
			terminatorValue = 1
		case 0xFFFF: // TERMINATOR_2
			terminatorValue = 2
		default:
			terminatorValue = 2 // Default to TERMINATOR_2
		}

		dialogueEntry := DialogueEntry{
			ID:         i,
			Type:       dialogueType,
			FontHeight: fontHeight,
			FontClut:   fontClut,
			Terminator: terminatorValue,
			Content:    content,
		}
		dialogueEntries = append(dialogueEntries, dialogueEntry)
	}

	// Detect special dialogues from Reserved section
	specialDialogueIDs := e.parseSpecialDialogues(wfm.Header.Reserved[:], expectedDialogues)

	// Mark special dialogues in the entries
	for i := range dialogueEntries {
		for _, specialID := range specialDialogueIDs {
			if dialogueEntries[i].ID == specialID {
				dialogueEntries[i].Special = true
				common.LogDebug(common.DebugDialogueMarkedSpecial, specialID)
				break
			}
		}
	}

	// Create YAML structure
	dialoguesYAML := DialoguesYAML{
		TotalDialogues: expectedDialogues,
		OriginalSize:   wfm.OriginalSize,
		Dialogues:      dialogueEntries,
	}

	// Export to YAML file in output root directory
	yamlFile := filepath.Join(outputDir, "dialogues.yaml")
	yamlWriter, err := os.Create(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to create YAML file: %w", err)
	}
	defer yamlWriter.Close()

	encoder := yaml.NewEncoder(yamlWriter)
	encoder.SetIndent(2)

	if err := encoder.Encode(dialoguesYAML); err != nil {
		return fmt.Errorf("failed to encode YAML: %w", err)
	}

	common.LogInfo(common.InfoDialoguesExported, len(dialogueEntries), yamlFile)
	return nil
}

// parseSpecialDialogues extracts special dialogue IDs from the Reserved section.
// Special dialogues are marked differently in the WFM file structure and require
// special handling during export and import operations.
// Parameters:
//   - reservedData: Byte array from the WFM header's Reserved section
//   - totalDialogues: Total number of dialogues expected in the file
//
// Returns a slice of dialogue IDs that are marked as special.
func (e *WFMFileExporter) parseSpecialDialogues(reservedData []byte, totalDialogues int) []int {
	e.debugReservedSection(reservedData)

	// Check if all bytes are zero - if so, no special dialogues exist
	if e.isAllZero(reservedData) {
		common.LogInfo(common.InfoNoSpecialDialoguesInFile)
		return []int{}
	}

	specialIDs := e.extractSpecialDialogueIDs(reservedData, totalDialogues)
	e.logSpecialDialogueResults(specialIDs)

	return specialIDs
}

// debugReservedSection logs debug information about the reserved section
func (e *WFMFileExporter) debugReservedSection(reservedData []byte) {
	debugOutput := ""
	for i := 0; i < 32 && i < len(reservedData); i++ {
		debugOutput += fmt.Sprintf(common.DebugReservedSectionHex, reservedData[i])
	}
	common.LogDebug(common.DebugReservedSectionBytes + debugOutput)
}

// isAllZero checks if all bytes in the data are zero
func (e *WFMFileExporter) isAllZero(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// extractSpecialDialogueIDs extracts special dialogue IDs from reserved data
func (e *WFMFileExporter) extractSpecialDialogueIDs(reservedData []byte, totalDialogues int) []int {
	var specialIDs []int

	// Handle special case where dialogue 0 should be included
	if e.shouldIncludeDialogueZero(reservedData) {
		specialIDs = append(specialIDs, 0)
		common.LogDebug(common.DebugDialogueZeroIncluded)
	}

	// Parse uint16 IDs stored in little endian format
	specialIDs = append(specialIDs, e.parseDialogueIDs(reservedData, totalDialogues)...)

	return specialIDs
}

// shouldIncludeDialogueZero determines if dialogue 0 should be included as special
func (e *WFMFileExporter) shouldIncludeDialogueZero(reservedData []byte) bool {
	if len(reservedData) < 2 {
		return false
	}

	firstID := uint16(reservedData[0]) | (uint16(reservedData[1]) << 8)
	if firstID != 0 {
		return false
	}

	// Check if there are non-zero values after the first ID
	for i := 2; i < len(reservedData); i++ {
		if reservedData[i] != 0 {
			return true
		}
	}

	return false
}

// parseDialogueIDs parses dialogue IDs from reserved data
func (e *WFMFileExporter) parseDialogueIDs(reservedData []byte, totalDialogues int) []int {
	var ids []int

	for i := 0; i < len(reservedData)-1; i += 2 {
		// Extract uint16 little endian
		id := uint16(reservedData[i]) | (uint16(reservedData[i+1]) << 8)

		// Skip zero values but don't stop processing
		if id == 0 {
			continue
		}

		// Only include IDs that are within the valid range
		if e.isValidDialogueID(id, totalDialogues) {
			ids = append(ids, int(id))
		} else {
			common.LogWarn(common.WarnInvalidDialogueID, id, totalDialogues-1)
		}
	}

	return ids
}

// isValidDialogueID checks if a dialogue ID is within valid range
func (e *WFMFileExporter) isValidDialogueID(id uint16, totalDialogues int) bool {
	return totalDialogues <= 65535 && id < uint16(totalDialogues)
}

// logSpecialDialogueResults logs the results of special dialogue parsing
func (e *WFMFileExporter) logSpecialDialogueResults(specialIDs []int) {
	if len(specialIDs) > 0 {
		common.LogInfo(common.InfoSpecialDialoguesDetected, specialIDs)
	} else {
		common.LogInfo(common.InfoNoValidSpecialDialogues)
	}
}

// buildGlyphMapping creates a mapping from glyph ID to character by comparing glyph images.
// This function analyzes exported glyph images and matches them against reference font files
// to establish character mappings for text decoding in dialogues.
// Parameters:
//   - glyphsDir: Directory containing exported glyph PNG files
//   - fontDir: Directory containing reference font PNG files organized by character
//
// Returns a map from glyph ID to character string, or an error if mapping fails.
func (e *WFMFileExporter) buildGlyphMapping(glyphsDir, fontDir string) (map[uint16]string, error) {
	// Check if font directory exists before proceeding
	if _, err := os.Stat(fontDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("font directory '%s' does not exist", fontDir)
	}

	fontFiles, err := e.collectFontFiles(fontDir)
	if err != nil {
		return nil, err
	}

	fontHashes, err := e.buildFontHashMap(fontFiles)
	if err != nil {
		return nil, err
	}

	mapping, err := e.matchGlyphsToFonts(glyphsDir, fontHashes)
	if err != nil {
		return nil, err
	}

	common.LogInfo(common.InfoGlyphMappingBuilt, len(mapping))
	return mapping, nil
}

// collectFontFiles recursively collects PNG files from the font directory
func (e *WFMFileExporter) collectFontFiles(fontDir string) ([]string, error) {
	fontFiles := make([]string, 0)
	err := filepath.Walk(fontDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.ToLower(filepath.Ext(path)) == ".png" {
			fontFiles = append(fontFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk font directory: %w", err)
	}
	return fontFiles, nil
}

// buildFontHashMap creates a hash map of font files to character names
func (e *WFMFileExporter) buildFontHashMap(fontFiles []string) (map[string]string, error) {
	fontHashes := make(map[string]string) // hash -> character name

	for _, fontFile := range fontFiles {
		hash, err := e.calculateImageHash(fontFile)
		if err != nil {
			continue // Skip files that can't be processed
		}

		charName := e.extractCharacterName(fontFile)
		fontHashes[hash] = charName
	}

	return fontHashes, nil
}

// extractCharacterName extracts character name from font file path
func (e *WFMFileExporter) extractCharacterName(fontFile string) string {
	baseName := filepath.Base(fontFile)
	fileName := strings.TrimSuffix(baseName, ".png")

	// Convert hexadecimal Unicode code to character
	if unicodeCode, err := strconv.ParseInt(fileName, 16, 32); err == nil {
		// Valid hexadecimal Unicode code point
		return string(rune(unicodeCode))
	}

	// Fallback to filename if not a valid hex code
	return fileName
}

// matchGlyphsToFonts matches glyph files to font characters using hash comparison
func (e *WFMFileExporter) matchGlyphsToFonts(glyphsDir string, fontHashes map[string]string) (map[uint16]string, error) {
	mapping := make(map[uint16]string)

	glyphFiles, err := filepath.Glob(filepath.Join(glyphsDir, "glyph_*.png"))
	if err != nil {
		return nil, fmt.Errorf("failed to list glyph files: %w", err)
	}

	for _, glyphFile := range glyphFiles {
		glyphID, charName, found := e.processGlyphFile(glyphFile, fontHashes)
		if found {
			mapping[glyphID] = charName
			common.LogDebug(common.DebugGlyphMapped, glyphID, charName)
		}
	}

	return mapping, nil
}

// processGlyphFile processes a single glyph file and returns mapping if found
func (e *WFMFileExporter) processGlyphFile(glyphFile string, fontHashes map[string]string) (uint16, string, bool) {
	hash, err := e.calculateImageHash(glyphFile)
	if err != nil {
		return 0, "", false
	}

	glyphID, err := e.extractGlyphID(glyphFile)
	if err != nil {
		return 0, "", false
	}

	if charName, found := fontHashes[hash]; found && glyphID <= 65535 {
		return uint16(glyphID), charName, true
	}

	return 0, "", false
}

// extractGlyphID extracts glyph ID from filename
func (e *WFMFileExporter) extractGlyphID(glyphFile string) (int, error) {
	baseName := filepath.Base(glyphFile)
	var glyphID int
	if _, err := fmt.Sscanf(baseName, "glyph_%04d.png", &glyphID); err != nil {
		return 0, err
	}
	return glyphID, nil
}

// calculateImageHash calculates a SHA256 hash of an image file for comparison.
// This function loads a PNG image and generates a hash based on its pixel content,
// which is used to match glyph images against reference font files.
// Parameters:
//   - imagePath: Path to the PNG image file to hash
//
// Returns the hexadecimal hash string, or an error if the operation fails.
func (e *WFMFileExporter) calculateImageHash(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return "", err
	}

	// Calculate hash based on image pixel content
	hasher := sha256.New()
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// Write pixel data to hasher for consistent hash generation
			// Color values from RGBA() are exactly in range 0-65535 (uint16 range), and &0xFFFF ensures safety
			if err := binary.Write(hasher, binary.LittleEndian, uint16(r&0xFFFF)); err != nil { // Safe: r is 0-65535, &0xFFFF is redundant but explicit
				return "", fmt.Errorf("failed to write red component to hasher: %w", err)
			}
			if err := binary.Write(hasher, binary.LittleEndian, uint16(g&0xFFFF)); err != nil { // Safe: g is 0-65535, &0xFFFF is redundant but explicit
				return "", fmt.Errorf("failed to write green component to hasher: %w", err)
			}
			if err := binary.Write(hasher, binary.LittleEndian, uint16(b&0xFFFF)); err != nil { // Safe: b is 0-65535, &0xFFFF is redundant but explicit
				return "", fmt.Errorf("failed to write blue component to hasher: %w", err)
			}
			if err := binary.Write(hasher, binary.LittleEndian, uint16(a&0xFFFF)); err != nil { // Safe: a is 0-65535, &0xFFFF is redundant but explicit
				return "", fmt.Errorf("failed to write alpha component to hasher: %w", err)
			}
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// WFMFileProcessor combines decoder and exporter functionality
type WFMFileProcessor struct {
	*WFMFileDecoder
	*WFMFileExporter
}

// NewWFMProcessor creates a new WFM processor with both decoder and exporter
func NewWFMProcessor() *WFMFileProcessor {
	return &WFMFileProcessor{
		WFMFileDecoder:  NewWFMDecoder(),
		WFMFileExporter: NewWFMExporter(),
	}
}

// Process handles the complete workflow of decoding and exporting a WFM file
func (p *WFMFileProcessor) Process(inputFile, outputDir string) error {
	// Open input file
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	originalSize := fileInfo.Size()

	// Decode WFM file
	wfm, err := p.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode WFM file: %w", err)
	}

	// Store original size in WFM structure
	wfm.OriginalSize = originalSize

	// Create output directory
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Export glyphs
	if err := p.ExportGlyphs(wfm, outputDir); err != nil {
		return fmt.Errorf("failed to export glyphs: %w", err)
	}

	// Export dialogues
	if err := p.ExportDialogues(wfm, outputDir); err != nil {
		return fmt.Errorf("failed to export dialogues: %w", err)
	}

	return nil
}
