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

// convert4bppToPNG converts 4bpp linear image data to PNG
func convert4bppToPNG(imageData []byte, width, height uint16) (*image.RGBA, error) {
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("invalid dimensions: width=%d, height=%d", width, height)
	}

	// Create RGBA image
	img := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))
	
	// 4bpp means 2 pixels per byte
	expectedBytes := (int(width) * int(height) + 1) / 2
	if len(imageData) < expectedBytes {
		return nil, fmt.Errorf("insufficient image data: expected at least %d bytes, got %d", expectedBytes, len(imageData))
	}

	// Simple grayscale palette for 4bpp (0-15 intensity levels)
	palette := make([]color.RGBA, 16)
	for i := 0; i < 16; i++ {
		intensity := uint8((i * 255) / 15)
		palette[i] = color.RGBA{intensity, intensity, intensity, 255}
	}

	pixelIndex := 0
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			byteIndex := pixelIndex / 2
			if byteIndex >= len(imageData) {
				break
			}

			var pixelValue uint8
			if pixelIndex%2 == 0 {
				// Even pixel: lower 4 bits (little endian)
				pixelValue = imageData[byteIndex] & 0x0F
			} else {
				// Odd pixel: upper 4 bits (little endian)
				pixelValue = (imageData[byteIndex] & 0xF0) >> 4
			}

			img.Set(x, y, palette[pixelValue])
			pixelIndex++
		}
	}

	return img, nil
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

// ExportGlyphs exports all glyphs as a single PNG file in horizontal layout
func (e *WFMFileExporter) ExportGlyphs(wfm *WFMFile, outputDir string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate that we have the expected number of glyphs
	expectedGlyphs := int(wfm.Header.TotalGlyphs)
	actualGlyphs := len(wfm.Glyphs)
	if actualGlyphs != expectedGlyphs {
		return fmt.Errorf("glyph count mismatch: expected %d, got %d", expectedGlyphs, actualGlyphs)
	}

	// Filter valid glyphs and calculate dimensions
	var validGlyphs []Glyph
	var validIndices []int
	maxWidth := uint16(0)
	maxHeight := uint16(0)

	for i, glyph := range wfm.Glyphs {
		if len(glyph.GlyphImage) > 0 && glyph.GlyphWidth > 0 && glyph.GlyphHeight > 0 {
			validGlyphs = append(validGlyphs, glyph)
			validIndices = append(validIndices, i)
			if glyph.GlyphWidth > maxWidth {
				maxWidth = glyph.GlyphWidth
			}
			if glyph.GlyphHeight > maxHeight {
				maxHeight = glyph.GlyphHeight
			}
		}
	}

	if len(validGlyphs) == 0 {
		return fmt.Errorf("no valid glyphs found")
	}

	// Calculate grid layout - try to make it more horizontal than vertical
	glyphCount := len(validGlyphs)
	// Use approximately square root for columns, but favor more columns for horizontal layout
	cols := int(float64(glyphCount)*0.7 + 0.5) // This will create more columns than rows
	if cols > glyphCount {
		cols = glyphCount
	}
	if cols < 1 {
		cols = 1
	}
	rows := (glyphCount + cols - 1) / cols // Ceiling division

	// Calculate total image dimensions
	totalWidth := cols * int(maxWidth)
	totalHeight := rows * int(maxHeight)

	fmt.Printf("Creating horizontal layout: %d columns × %d rows (%dx%d pixels per glyph)\n", 
		cols, rows, maxWidth, maxHeight)

	// Create a combined image
	combinedImg := image.NewRGBA(image.Rect(0, 0, totalWidth, totalHeight))

	// Simple grayscale palette for 4bpp (0-15 intensity levels)
	palette := make([]color.RGBA, 16)
	for i := 0; i < 16; i++ {
		intensity := uint8((i * 255) / 15)
		palette[i] = color.RGBA{intensity, intensity, intensity, 255}
	}

	// Red color for separators
	redColor := color.RGBA{255, 0, 0, 255}

	// Place glyphs in grid layout
	for glyphIndex, glyph := range validGlyphs {
		// Calculate grid position
		col := glyphIndex % cols
		row := glyphIndex / cols
		
		// Calculate pixel offset
		offsetX := col * int(maxWidth)
		offsetY := row * int(maxHeight)

		// Convert this glyph to image data
		width := int(glyph.GlyphWidth)
		height := int(glyph.GlyphHeight)

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

				combinedImg.Set(offsetX+x, offsetY+y, palette[pixelValue])
				pixelIndex++
			}
		}

		// Draw red separators around each glyph to show its actual dimensions
		// Top border (horizontal line)
		for x := 0; x < width; x++ {
			combinedImg.Set(offsetX+x, offsetY, redColor)
		}
		// Bottom border (horizontal line)  
		for x := 0; x < width; x++ {
			combinedImg.Set(offsetX+x, offsetY+height-1, redColor)
		}
		// Left border (vertical line)
		for y := 0; y < height; y++ {
			combinedImg.Set(offsetX, offsetY+y, redColor)
		}
		// Right border (vertical line)
		for y := 0; y < height; y++ {
			combinedImg.Set(offsetX+width-1, offsetY+y, redColor)
		}

		fmt.Printf("Added glyph %d: %dx%d pixels at (%d,%d) grid[%d,%d] (CLUT: %d, Handakuten: %d)\n", 
			validIndices[glyphIndex], glyph.GlyphWidth, glyph.GlyphHeight, 
			offsetX, offsetY, col, row, glyph.GlyphClut, glyph.GlyphHandakuten)
	}

	// Save the combined PNG directly in the output directory
	pngFile := filepath.Join(outputDir, "all_glyphs.png")
	file, err := os.Create(pngFile)
	if err != nil {
		return fmt.Errorf("failed to create combined PNG file: %w", err)
	}
	defer file.Close()

	if err := png.Encode(file, combinedImg); err != nil {
		return fmt.Errorf("failed to encode combined PNG: %w", err)
	}

	fmt.Printf("Exported %d glyphs to horizontal PNG: %dx%d pixels (%d columns × %d rows)\n", 
		len(validGlyphs), totalWidth, totalHeight, cols, rows)
	fmt.Printf("Saved to: %s\n", pngFile)

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