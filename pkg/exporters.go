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

	"gopkg.in/yaml.v3"
)

// WFMFileExporter implements the WFMExporter interface
type WFMFileExporter struct{}

// NewWFMExporter creates a new WFM exporter instance
func NewWFMExporter() *WFMFileExporter {
	return &WFMFileExporter{}
}

// ExportGlyphs exports each glyph as an individual PNG file
func (e *WFMFileExporter) ExportGlyphs(wfm *WFMFile, outputDir string) error {
	// Create glyphs subdirectory
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
			fmt.Printf("Skipping glyph %d: invalid dimensions or empty image data\n", glyphIndex)
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

		fmt.Printf("Exported glyph %d: %dx%d pixels (CLUT: %d, Handakuten: %d) -> %s\n",
			glyphIndex, glyph.GlyphWidth, glyph.GlyphHeight,
			glyph.GlyphClut, glyph.GlyphHandakuten, filename)
		exportedCount++
	}

	fmt.Printf("Successfully exported %d individual glyph PNG files to: %s\n", exportedCount, glyphsDir)
	return nil
}

// DialogueEntry represents a single dialogue with decoded text
type DialogueEntry struct {
	ID         int    `yaml:"id"`
	Type       string `yaml:"type"`
	BoxWidth   *int   `yaml:"box_width,omitempty"`
	BoxHeight  *int   `yaml:"box_height,omitempty"`
	FontHeight int    `yaml:"font_height"`
	Text       string `yaml:"text"`
}

// DialoguesYAML represents the complete dialogues structure for YAML export
type DialoguesYAML struct {
	TotalDialogues int             `yaml:"total_dialogues"`
	Dialogues      []DialogueEntry `yaml:"dialogues"`
}

// processDialogueText processes dialogue text, extracting box dimensions and cleaning control codes
func processDialogueText(rawData []byte, glyphMapping map[uint16]string, glyphs []Glyph) (text string, dialogueType string, boxWidth *int, boxHeight *int, fontHeight int) {
	decodedText := ""
	var width, height *int
	entryType := "event"    // Default to event type
	detectedFontHeight := 8 // Default to 8, will be updated when we find actual glyphs

	// Process dialogue data in 2-byte chunks
	for i := 0; i+1 < len(rawData); i += 2 {
		// Read 2 bytes as little endian uint16
		glyphID := binary.LittleEndian.Uint16(rawData[i : i+2])

		// Check for termination
		if glyphID == 0xFFFF {
			break
		}

		// Handle [INIT TEXT BOX] with width and height parameters
		if glyphID == 0xFFFA { // [INIT TEXT BOX]
			entryType = "dialogue" // Set type to dialogue when INIT TEXT BOX is found
			// Next 2 bytes are width
			if i+4 <= len(rawData) {
				w := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				width = &w
				i += 2 // Skip width bytes
			}
			// Next 2 bytes are height
			if i+4 <= len(rawData) {
				h := int(binary.LittleEndian.Uint16(rawData[i+2 : i+4]))
				height = &h
				i += 2 // Skip height bytes
			}
			continue // Don't add [INIT TEXT BOX] to text
		}

		// Handle Termination markers
		if glyphID == 0xFFFE || glyphID == 0xFFFF {
			break
		}

		// Convert to glyph index (subtract 0x8000 base)
		if glyphID >= 0x8000 && glyphID <= 0xFFF0 {
			actualGlyphID := glyphID - 0x8000

			// Check glyph height to determine font height
			if glyphs != nil && int(actualGlyphID) < len(glyphs) {
				glyph := glyphs[actualGlyphID]
				if glyph.GlyphHeight == 16 {
					detectedFontHeight = 16
				} else if glyph.GlyphHeight == 24 {
					detectedFontHeight = 24
				}
			}

			// Try to decode character
			if glyphMapping != nil {
				if char, found := glyphMapping[actualGlyphID]; found {
					decodedText += char
				} else {
					decodedText += fmt.Sprintf("[%04X]", glyphID)
				}
			} else {
				decodedText += fmt.Sprintf("[%04X]", glyphID)
			}
		} else {
			// Handle special control codes
			specialCode := getSpecialCharacterCode(glyphID)
			decodedText += specialCode
		}
	}

	return decodedText, entryType, width, height, detectedFontHeight
}

// getSpecialCharacterCode returns the formatted string for special control codes
func getSpecialCharacterCode(code uint16) string {
	switch code {
	case 0xFFF3:
		return "[HALT]"
	case 0xFFF4:
		return "[F4]"
	case 0xFFF5:
		return "[PROMPT]"
	case 0xFFF6:
		return "[F6]" // args: 2
	case 0xFFF7:
		return "[CHANGE COLOR TO]" // args: 1
	case 0xFFF8:
		return "[INIT TAIL]" // args: 2
	case 0xFFF9:
		return "[PAUSE FOR]" // args: 1
	case 0xFFFB:
		return "\n\n"
	case 0xFFFC:
		return "[WAIT FOR INPUT]"
	case 0xFFFD:
		return "\n" // Convert [NEWLINE] to actual newline
	default:
		return fmt.Sprintf("<%04X>", code)
	}
}

// ExportDialogues exports dialogues as a single YAML file with text decoding
func (e *WFMFileExporter) ExportDialogues(wfm *WFMFile, outputDir string) error {
	// Validate that we have the expected number of dialogues
	expectedDialogues := int(wfm.Header.TotalDialogues)
	actualDialogues := len(wfm.Dialogues)
	if actualDialogues != expectedDialogues {
		return fmt.Errorf("dialogue count mismatch: expected %d, got %d", expectedDialogues, actualDialogues)
	}

	// Build glyph hash to character mapping from font files
	glyphsDir := filepath.Join(outputDir, "glyphs")
	fontDir := "fonts" // User should have a 'fonts' directory with character-named PNG files
	glyphMapping, err := e.buildGlyphMapping(glyphsDir, fontDir)
	if err != nil {
		fmt.Printf("Warning: Could not build glyph mapping from font directory: %v\n", err)
		fmt.Printf("Dialogues will be exported without text decoding\n")
	}

	// Process each dialogue using data already extracted in DecodeDialogues
	var dialogueEntries []DialogueEntry
	for i, dialogue := range wfm.Dialogues {
		// Process dialogue text and extract box dimensions
		text, dialogueType, boxWidth, boxHeight, fontHeight := processDialogueText(dialogue.Data, glyphMapping, wfm.Glyphs)

		dialogueEntry := DialogueEntry{
			ID:         i,
			Type:       dialogueType,
			BoxWidth:   boxWidth,
			BoxHeight:  boxHeight,
			FontHeight: fontHeight,
			Text:       text,
		}
		dialogueEntries = append(dialogueEntries, dialogueEntry)
	}

	// Create YAML structure
	dialoguesYAML := DialoguesYAML{
		TotalDialogues: expectedDialogues,
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

	fmt.Printf("Exported %d dialogues to YAML: %s\n", len(dialogueEntries), yamlFile)
	return nil
}

// buildGlyphMapping creates a mapping from glyph ID to character by comparing glyph images
func (e *WFMFileExporter) buildGlyphMapping(glyphsDir, fontDir string) (map[uint16]string, error) {
	mapping := make(map[uint16]string)

	// Check if font directory exists
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
			fmt.Printf("Mapped glyph %d to character '%s'\n", glyphID, charName)
		}
	}

	fmt.Printf("Built glyph mapping: %d glyphs mapped to characters\n", len(mapping))
	return mapping, nil
}

// calculateImageHash calculates a simple hash of an image file for comparison
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

	// Calculate hash based on image content
	hasher := sha256.New()
	bounds := img.Bounds()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// Write pixel data to hasher
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

	// Decode WFM file
	wfm, err := p.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode WFM file: %w", err)
	}

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
