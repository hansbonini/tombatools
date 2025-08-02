// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains exporters for converting WFM data to PNG images and YAML dialogue files.
package pkg

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hansbonini/tombatools/pkg/common"
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
	// Create glyphs subdirectory for organizing exported glyph images
	glyphsDir := filepath.Join(outputDir, "glyphs")
	if err := os.MkdirAll(glyphsDir, 0755); err != nil {
		return fmt.Errorf("failed to create glyphs directory: %w", err)
	}

	// Validate that we have the expected number of glyphs
	expectedGlyphs := int(wfm.Header.TotalGlyphs)
	actualGlyphs := len(wfm.Glyphs)
	if actualGlyphs != expectedGlyphs {
		return fmt.Errorf("glyph count mismatch: expected %d, got %d", expectedGlyphs, actualGlyphs)
	}

	// Function to convert PSX 16-bit color to RGBA
	psxToRGBA := func(psxColor uint16) color.RGBA {
		if psxColor == 0 {
			return color.RGBA{0, 0, 0, 0} // Transparent for color 0
		}
		r := uint8((psxColor & 0x1F) << 3)         // Red: bits 0-4
		g := uint8(((psxColor >> 5) & 0x1F) << 3)  // Green: bits 5-9
		b := uint8(((psxColor >> 10) & 0x1F) << 3) // Blue: bits 10-14
		return color.RGBA{r, g, b, 255}
	}

	// Process each glyph individually
	exportedCount := 0
	for glyphIndex, glyph := range wfm.Glyphs {
		// Skip invalid glyphs
		if len(glyph.GlyphImage) == 0 || glyph.GlyphWidth == 0 || glyph.GlyphHeight == 0 {
			common.LogDebug(common.DebugGlyphSkipped, glyphIndex)
			continue
		}

		width := int(glyph.GlyphWidth)
		height := int(glyph.GlyphHeight)

		// Select the correct palette based on GlyphHeight
		var currentPalette [16]uint16
		if glyph.GlyphHeight == 24 {
			// Use EventClut for glyphs with height 24
			currentPalette = EventClut
		} else {
			// Use DialogueClut for all other heights
			currentPalette = DialogueClut
		}

		// Create individual image for this glyph
		glyphImg := image.NewRGBA(image.Rect(0, 0, width, height))

		// Process 4bpp data
		pixelIndex := 0
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				byteIndex := pixelIndex / 2
				if byteIndex >= len(glyph.GlyphImage) {
					break
				}

				var pixelValue uint8
				if pixelIndex%2 == 0 {
					// Even pixel: lower 4 bits (little endian)
					pixelValue = glyph.GlyphImage[byteIndex] & 0x0F
				} else {
					// Odd pixel: upper 4 bits (little endian)
					pixelValue = (glyph.GlyphImage[byteIndex] & 0xF0) >> 4
				}

				// Convert PSX color to RGBA
				psxColor := currentPalette[pixelValue]
				finalColor := psxToRGBA(psxColor)
				glyphImg.Set(x, y, finalColor)
				pixelIndex++
			}
		}

		// Save individual PNG file
		filename := fmt.Sprintf("glyph_%04d.png", glyphIndex)
		pngFile := filepath.Join(glyphsDir, filename)
		file, err := os.Create(pngFile)
		if err != nil {
			return fmt.Errorf("failed to create PNG file for glyph %d: %w", glyphIndex, err)
		}

		if err := png.Encode(file, glyphImg); err != nil {
			file.Close()
			return fmt.Errorf("failed to encode PNG for glyph %d: %w", glyphIndex, err)
		}
		file.Close()

		common.LogDebug(common.DebugGlyphExported,
			glyphIndex, glyph.GlyphWidth, glyph.GlyphHeight,
			glyph.GlyphClut, glyph.GlyphHandakuten, filename)
		exportedCount++
	}

	common.LogInfo(common.InfoGlyphsExported, exportedCount, glyphsDir)
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
	var content []map[string]interface{}
	var currentText string
	entryType := "event"          // Default to event type
	detectedFontHeight := 8       // Default to 8, will be updated when we find actual glyphs
	detectedFontClut := uint16(0) // Default CLUT

	// Function to add current text to content if it exists
	addTextContent := func() {
		if currentText != "" {
			content = append(content, map[string]interface{}{
				"text": currentText,
			})
			currentText = ""
		}
	}

	var terminator uint16 = 0xFFFF // Default terminator

	// Process dialogue data in 2-byte chunks
	for i := 0; i+1 < len(rawData); i += 2 {
		// Read 2 bytes as little endian uint16
		glyphID := binary.LittleEndian.Uint16(rawData[i : i+2])

		// Check for termination
		if glyphID == 0xFFFF || glyphID == 0xFFFE {
			terminator = glyphID
			break
		}

		// Handle [INIT TEXT BOX] with width and height parameters
		if glyphID == INIT_TEXT_BOX { // [INIT TEXT BOX]
			entryType = "dialogue" // Set type to dialogue when INIT TEXT BOX is found
			// Next 2 bytes are width
			if i+4 <= len(rawData) {
				width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				// Next 2 bytes are height
				if i+6 <= len(rawData) {
					height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
					content = append(content, map[string]interface{}{
						"box": map[string]interface{}{
							"width":  width,
							"height": height,
						},
					})
					i += 4 // Skip both width and height bytes
				} else {
					i += 2 // Skip only width bytes
				}
			}
			continue
		}

		// Handle INIT_TAIL with width and height parameters
		if glyphID == INIT_TAIL {
			// Add current text before adding tail
			addTextContent()
			// Next 2 bytes are width
			if i+4 <= len(rawData) {
				width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				// Next 2 bytes are height
				if i+6 <= len(rawData) {
					height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
					content = append(content, map[string]interface{}{
						"tail": map[string]interface{}{
							"width":  width,
							"height": height,
						},
					})
					i += 4 // Skip both width and height bytes
				} else {
					i += 2 // Skip only width bytes
				}
			}
			continue
		}

		// Handle F6 command with width and height parameters
		if glyphID == F6 {
			// Add current text before adding f6
			addTextContent()
			// Next 2 bytes are width
			if i+4 <= len(rawData) {
				width := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				// Next 2 bytes are height
				if i+6 <= len(rawData) {
					height := int(binary.LittleEndian.Uint16(rawData[i+4 : i+6]))
					content = append(content, map[string]interface{}{
						"f6": map[string]interface{}{
							"width":  width,
							"height": height,
						},
					})
					i += 4 // Skip both width and height bytes
				} else {
					i += 2 // Skip only width bytes
				}
			}
			continue
		}

		// Handle CHANGE_COLOR_TO
		if glyphID == CHANGE_COLOR_TO {
			// Add current text before changing color
			addTextContent()
			// Next 2 bytes are color value
			if i+4 <= len(rawData) {
				colorValue := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				content = append(content, map[string]interface{}{
					"color": map[string]interface{}{
						"value": colorValue,
					},
				})
				i += 2 // Skip color value bytes
			}
			continue
		}

		// Handle PAUSE_FOR
		if glyphID == PAUSE_FOR {
			// Add current text before adding pause
			addTextContent()
			// Next 2 bytes are duration
			if i+4 <= len(rawData) {
				duration := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				content = append(content, map[string]interface{}{
					"pause": map[string]interface{}{
						"duration": duration,
					},
				})
				i += 2 // Skip duration bytes
			}
			continue
		}

		// Handle FFF2 command with single parameter
		if glyphID == FFF2 {
			// Add current text before adding fff2
			addTextContent()
			// Next 2 bytes are parameter value
			if i+4 <= len(rawData) {
				paramValue := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				content = append(content, map[string]interface{}{
					"fff2": map[string]interface{}{
						"value": paramValue,
					},
				})
				i += 2 // Skip parameter value bytes
			}
			continue
		}

		// Handle Termination markers
		if glyphID == TERMINATOR_1 || glyphID == TERMINATOR_2 {
			break
		}

		// Convert to glyph index (subtract GLYPH_ID_BASE)
		if glyphID >= GLYPH_ID_BASE && glyphID <= 0xFFF0 {
			actualGlyphID := glyphID - GLYPH_ID_BASE

			// Check glyph height and clut to determine font height and clut
			if glyphs != nil && int(actualGlyphID) < len(glyphs) {
				glyph := glyphs[actualGlyphID]
				if glyph.GlyphHeight == 16 {
					detectedFontHeight = 16
				} else if glyph.GlyphHeight == 24 {
					detectedFontHeight = 24
				}
				// Update font CLUT from the actual glyph data
				detectedFontClut = glyph.GlyphClut
			}

			// Try to decode character
			if glyphMapping != nil {
				if char, found := glyphMapping[actualGlyphID]; found {
					currentText += char
				} else {
					// Special handling for special commands
					if glyphID == C04D {
						currentText += "▼"
					} else if glyphID == C04E {
						currentText += "⏷"
					} else {
						currentText += fmt.Sprintf("[%04X]", glyphID)
					}
				}
			} else {
				// Special handling for special commands
				if glyphID == C04D {
					currentText += "▼"
				} else if glyphID == C04E {
					currentText += "⏷"
				} else {
					currentText += fmt.Sprintf("[%04X]", glyphID)
				}
			}
		} else {
			// Handle special control codes
			switch glyphID {
			case C04D:
				currentText += "▼" // Unicode down-pointing triangle for C04D
			case C04E:
				currentText += "⏷" // Unicode down-pointing triangle for C04E
			case WAIT_FOR_INPUT:
				currentText += "⧗" // Unicode hourglass for WAIT_FOR_INPUT
			case NEWLINE:
				currentText += "\n"
			case DOUBLE_NEWLINE:
				currentText += "\n\n"
			default:
				specialCode := getSpecialCharacterCode(glyphID)
				currentText += specialCode
			}
		}
	}

	// Add any remaining text
	addTextContent()

	return content, entryType, detectedFontHeight, detectedFontClut, terminator
}

// getSpecialCharacterCode returns the formatted string for special control codes
func getSpecialCharacterCode(code uint16) string {
	switch code {
	case FFF2:
		return "[FFF2]" // args: 1
	case HALT:
		return "[HALT]"
	case F4:
		return "[F4]"
	case PROMPT:
		return "[PROMPT]"
	case F6:
		return "[F6]" // args: 2
	case CHANGE_COLOR_TO:
		return "[CHANGE COLOR TO]" // args: 1
	case INIT_TAIL:
		return "[INIT TAIL]" // args: 2
	case PAUSE_FOR:
		return "[PAUSE FOR]" // args: 1
	case DOUBLE_NEWLINE:
		return "\n\n"
	case C04D:
		return "[C04D]"
	case C04E:
		return "[C04E]"
	case WAIT_FOR_INPUT:
		return "[WAIT FOR INPUT]"
	case NEWLINE:
		return "\n" // Convert [NEWLINE] to actual newline
	default:
		return fmt.Sprintf("<%04X>", code)
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
	var dialogueEntries []DialogueEntry
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
	var specialIDs []int

	// Debug: show first 32 bytes of Reserved section for analysis
	debugOutput := ""
	for i := 0; i < 32 && i < len(reservedData); i++ {
		debugOutput += fmt.Sprintf(common.DebugReservedSectionHex, reservedData[i])
	}
	common.LogDebug(common.DebugReservedSectionBytes + debugOutput)

	// Check if all 128 bytes are zero - if so, no special dialogues exist
	allZero := true
	for _, b := range reservedData {
		if b != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		common.LogInfo(common.InfoNoSpecialDialoguesInFile)
		return specialIDs
	}

	// Check if first uint16 is 0 but there are non-zero values after it
	firstID := uint16(reservedData[0]) | (uint16(reservedData[1]) << 8)
	hasNonZeroAfterFirst := false
	for i := 2; i < len(reservedData); i++ {
		if reservedData[i] != 0 {
			hasNonZeroAfterFirst = true
			break
		}
	}

	// If first ID is 0 and there are non-zero values after, include dialogue 0 as special
	if firstID == 0 && hasNonZeroAfterFirst {
		specialIDs = append(specialIDs, 0)
		common.LogDebug(common.DebugDialogueZeroIncluded)
	}

	// Parse uint16 IDs stored in little endian format
	for i := 0; i < len(reservedData)-1; i += 2 {
		// Extract uint16 little endian
		id := uint16(reservedData[i]) | (uint16(reservedData[i+1]) << 8)

		// Skip zero values but don't stop processing
		if id == 0 {
			continue
		}

		// Only include IDs that are within the valid range for dialogue IDs
		// IDs should be between 0 and totalDialogues-1
		if id < uint16(totalDialogues) {
			specialIDs = append(specialIDs, int(id))
		} else {
			common.LogWarn(common.WarnInvalidDialogueID, id, totalDialogues-1)
		}
	}

	if len(specialIDs) > 0 {
		common.LogInfo(common.InfoSpecialDialoguesDetected, specialIDs)
	} else {
		common.LogInfo(common.InfoNoValidSpecialDialogues)
	}

	return specialIDs
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
	mapping := make(map[uint16]string)

	// Check if font directory exists before proceeding
	if _, err := os.Stat(fontDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("font directory '%s' does not exist", fontDir)
	}

	// Get list of font files recursively from fonts directory and subdirectories
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

	// Calculate hashes for each font file
	fontHashes := make(map[string]string) // hash -> character name
	for _, fontFile := range fontFiles {
		hash, err := e.calculateImageHash(fontFile)
		if err != nil {
			continue // Skip files that can't be processed
		}

		// Extract character from filename (remove .png extension)
		baseName := filepath.Base(fontFile)
		fileName := strings.TrimSuffix(baseName, ".png")

		// Convert hexadecimal Unicode code to character
		var charName string
		if unicodeCode, err := strconv.ParseInt(fileName, 16, 32); err == nil {
			// Valid hexadecimal Unicode code point
			charName = string(rune(unicodeCode))
		} else {
			// Fallback to filename if not a valid hex code
			charName = fileName
		}

		fontHashes[hash] = charName
	}

	// Calculate hashes for each glyph file and find matches
	glyphFiles, err := filepath.Glob(filepath.Join(glyphsDir, "glyph_*.png"))
	if err != nil {
		return nil, fmt.Errorf("failed to list glyph files: %w", err)
	}

	for _, glyphFile := range glyphFiles {
		hash, err := e.calculateImageHash(glyphFile)
		if err != nil {
			continue // Skip files that can't be processed
		}

		// Extract glyph ID from filename
		baseName := filepath.Base(glyphFile)
		var glyphID int
		if _, err := fmt.Sscanf(baseName, "glyph_%04d.png", &glyphID); err != nil {
			continue
		}

		// Check if hash matches any font file
		if charName, found := fontHashes[hash]; found {
			mapping[uint16(glyphID)] = charName
			common.LogDebug(common.DebugGlyphMapped, glyphID, charName)
		}
	}

	common.LogInfo(common.InfoGlyphMappingBuilt, len(mapping))
	return mapping, nil
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
			binary.Write(hasher, binary.LittleEndian, uint16(r))
			binary.Write(hasher, binary.LittleEndian, uint16(g))
			binary.Write(hasher, binary.LittleEndian, uint16(b))
			binary.Write(hasher, binary.LittleEndian, uint16(a))
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
func (p *WFMFileProcessor) Process(inputFile string, outputDir string) error {
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
	if err := os.MkdirAll(outputDir, 0755); err != nil {
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
