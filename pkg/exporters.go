package pkg

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
)

// WFMFileExporter implements the WFMExporter interface
type WFMFileExporter struct{}

// NewWFMExporter creates a new WFM exporter instance
func NewWFMExporter() *WFMFileExporter {
	return &WFMFileExporter{}
}

// WFMFileJSON represents the JSON structure for exporting WFM files
type WFMFileJSON struct {
	Header struct {
		Magic                 string `json:"magic"`
		Padding               uint32 `json:"padding"`
		DialoguePointerTable  uint32 `json:"dialogue_pointer_table"`
		TotalDialogues        uint16 `json:"total_dialogues"`
		TotalGlyphs           uint16 `json:"total_glyphs"`
	} `json:"header"`
	GlyphPointerTable     []uint16 `json:"glyph_pointer_table"`
	GlyphCount            int      `json:"glyph_count"`
	DialoguePointerTable  []uint32 `json:"dialogue_pointer_table"`
	DialogueCount         int      `json:"dialogue_count"`
}

// ExportToJSON exports the WFM file structure to JSON format
func (e *WFMFileExporter) ExportToJSON(wfm *WFMFile, writer io.Writer) error {
	// Validate that arrays match header counts
	expectedGlyphs := int(wfm.Header.TotalGlyphs)
	actualGlyphs := len(wfm.Glyphs)
	if actualGlyphs != expectedGlyphs {
		return fmt.Errorf("glyph count mismatch in JSON export: expected %d, got %d", expectedGlyphs, actualGlyphs)
	}

	expectedDialogues := int(wfm.Header.TotalDialogues)
	actualDialogues := len(wfm.Dialogues)
	if actualDialogues != expectedDialogues {
		return fmt.Errorf("dialogue count mismatch in JSON export: expected %d, got %d", expectedDialogues, actualDialogues)
	}

	jsonData := WFMFileJSON{
		GlyphPointerTable:    wfm.GlyphPointerTable,
		GlyphCount:           int(wfm.Header.TotalGlyphs),
		DialoguePointerTable: wfm.DialoguePointerTable,
		DialogueCount:        int(wfm.Header.TotalDialogues),
	}

	// Convert header
	jsonData.Header.Magic = string(wfm.Header.Magic[:])
	jsonData.Header.Padding = wfm.Header.Padding
	jsonData.Header.DialoguePointerTable = wfm.Header.DialoguePointerTable
	jsonData.Header.TotalDialogues = wfm.Header.TotalDialogues
	jsonData.Header.TotalGlyphs = wfm.Header.TotalGlyphs

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(jsonData); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	return nil
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
		r := uint8((psxColor & 0x1F) << 3)        // Red: bits 0-4
		g := uint8(((psxColor >> 5) & 0x1F) << 3) // Green: bits 5-9  
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

// ExportDialogues exports individual dialog data to separate files
func (e *WFMFileExporter) ExportDialogues(wfm *WFMFile, outputDir string) error {
	dialogDir := filepath.Join(outputDir, "dialogues")
	if err := os.MkdirAll(dialogDir, 0755); err != nil {
		return fmt.Errorf("failed to create dialogue directory: %w", err)
	}

	// Validate that we have the expected number of dialogues
	expectedDialogues := int(wfm.Header.TotalDialogues)
	actualDialogues := len(wfm.Dialogues)
	if actualDialogues != expectedDialogues {
		return fmt.Errorf("dialogue count mismatch: expected %d, got %d", expectedDialogues, actualDialogues)
	}

	for i, dialogue := range wfm.Dialogues {
		filename := filepath.Join(dialogDir, fmt.Sprintf("dialogue_%04d.bin", i))
		if err := os.WriteFile(filename, dialogue.Data, 0644); err != nil {
			return fmt.Errorf("failed to write dialogue %d: %w", i, err)
		}
	}

	return nil
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

	// Export to JSON
	jsonFile := filepath.Join(outputDir, "info.json")
	jsonWriter, err := os.Create(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer jsonWriter.Close()

	if err := p.ExportToJSON(wfm, jsonWriter); err != nil {
		return fmt.Errorf("failed to export JSON: %w", err)
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