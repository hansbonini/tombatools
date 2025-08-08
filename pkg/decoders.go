// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains decoders for reading and parsing WFM file structures and data.
package pkg

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/hansbonini/tombatools/pkg/common"
)

// WFMFileDecoder implements the WFMDecoder interface and provides
// functionality to decode WFM files into structured data.
type WFMFileDecoder struct{}

// NewWFMDecoder creates a new WFM decoder instance.
// Returns a pointer to a WFMFileDecoder ready for parsing WFM files.
func NewWFMDecoder() *WFMFileDecoder {
	return &WFMFileDecoder{}
}

// NewGAMProcessor creates a new GAM processor instance
func NewGAMProcessor() *GAMProcessor {
	return &GAMProcessor{}
}

// Decode reads and parses a complete WFM file from the provided reader.
// This is the main entry point for WFM file parsing, handling header, glyphs, and dialogues.
// Parameters:
//   - reader: io.Reader containing WFM file data to decode
//
// Returns a pointer to the decoded WFMFile structure, or an error if parsing fails.
func (d *WFMFileDecoder) Decode(reader io.Reader) (*WFMFile, error) {
	wfm := &WFMFile{}

	// Decode the WFM file header first
	header, err := d.DecodeHeader(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}
	wfm.Header = *header

	// Decode glyph data
	glyphPointers, glyphs, err := d.DecodeGlyphs(reader, header)
	if err != nil {
		return nil, fmt.Errorf("failed to decode glyphs: %w", err)
	}
	wfm.GlyphPointerTable = glyphPointers
	wfm.Glyphs = glyphs

	// Decode dialogue data
	dialoguePointers, dialogues, err := d.DecodeDialogues(reader, header)
	if err != nil {
		return nil, fmt.Errorf("failed to decode dialogue: %w", err)
	}
	wfm.DialoguePointerTable = dialoguePointers
	wfm.Dialogues = dialogues

	return wfm, nil
}

// DecodeHeader reads and parses the WFM file header structure.
// The header contains metadata about the file including magic signature,
// dialogue counts, glyph information, and pointer tables.
// Parameters:
//   - reader: io.Reader positioned at the start of the WFM file
//
// Returns a pointer to the decoded WFMHeader structure, or an error if parsing fails.
func (d *WFMFileDecoder) DecodeHeader(reader io.Reader) (*WFMHeader, error) {
	header := &WFMHeader{}

	// Read and validate magic header signature
	if err := binary.Read(reader, binary.LittleEndian, &header.Magic); err != nil {
		return nil, fmt.Errorf("failed to read magic header: %w", err)
	}

	// Validate magic header
	if string(header.Magic[:]) != common.WFMFileMagic {
		return nil, fmt.Errorf("invalid magic header: expected '%s', got '%s'", common.WFMFileMagic, string(header.Magic[:]))
	}

	// Read padding
	if err := binary.Read(reader, binary.LittleEndian, &header.Padding); err != nil {
		return nil, fmt.Errorf("failed to read padding: %w", err)
	}

	// Read dialog pointer table offset
	if err := binary.Read(reader, binary.LittleEndian, &header.DialoguePointerTable); err != nil {
		return nil, fmt.Errorf("failed to read dialogue pointer table: %w", err)
	}
	common.LogDebug(common.DebugHeaderPointerTable, header.DialoguePointerTable, header.DialoguePointerTable)

	// Read total dialogs count
	if err := binary.Read(reader, binary.LittleEndian, &header.TotalDialogues); err != nil {
		return nil, fmt.Errorf("failed to read total dialogues: %w", err)
	}

	// Read total glyphs count
	if err := binary.Read(reader, binary.LittleEndian, &header.TotalGlyphs); err != nil {
		return nil, fmt.Errorf("failed to read total glyphs: %w", err)
	}

	// Skip reserved 128 bytes
	if err := binary.Read(reader, binary.LittleEndian, &header.Reserved); err != nil {
		return nil, fmt.Errorf("failed to read reserved bytes: %w", err)
	}

	return header, nil
}

// DecodeGlyphs reads the glyph pointer table and glyph data
func (d *WFMFileDecoder) DecodeGlyphs(reader io.Reader, header *WFMHeader) ([]uint16, []Glyph, error) {
	glyphPointers, err := d.readGlyphPointers(reader, header.TotalGlyphs)
	if err != nil {
		return nil, nil, err
	}

	glyphs, err := d.readGlyphData(reader, header.TotalGlyphs)
	if err != nil {
		return nil, nil, err
	}

	return glyphPointers, glyphs, nil
}

// readGlyphPointers reads the glyph pointer table
func (d *WFMFileDecoder) readGlyphPointers(reader io.Reader, totalGlyphs uint16) ([]uint16, error) {
	glyphPointers := make([]uint16, totalGlyphs)

	for i := uint16(0); i < totalGlyphs; i++ {
		if err := binary.Read(reader, binary.LittleEndian, &glyphPointers[i]); err != nil {
			return nil, fmt.Errorf("failed to read glyph pointer %d: %w", i, err)
		}
	}

	return glyphPointers, nil
}

// readGlyphData reads glyph data for all glyphs
func (d *WFMFileDecoder) readGlyphData(reader io.Reader, totalGlyphs uint16) ([]Glyph, error) {
	glyphs := make([]Glyph, totalGlyphs)

	for i := uint16(0); i < totalGlyphs; i++ {
		glyph, err := d.readSingleGlyph(reader)
		if err != nil {
			// Create empty glyph on error
			glyph = d.createEmptyGlyph()
		}
		glyphs[i] = glyph
	}

	return glyphs, nil
}

// readSingleGlyph reads a single glyph structure
func (d *WFMFileDecoder) readSingleGlyph(reader io.Reader) (Glyph, error) {
	glyph := Glyph{}

	// Read glyph header
	if err := d.readGlyphHeader(reader, &glyph); err != nil {
		return glyph, err
	}

	// Read glyph image data
	if err := d.readGlyphImage(reader, &glyph); err != nil {
		return glyph, err
	}

	return glyph, nil
}

// readGlyphHeader reads the glyph header (clut, height, width, handakuten)
func (d *WFMFileDecoder) readGlyphHeader(reader io.Reader, glyph *Glyph) error {
	if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphClut); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphHeight); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphWidth); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphHandakuten); err != nil {
		return err
	}
	return nil
}

// readGlyphImage reads the glyph image data
func (d *WFMFileDecoder) readGlyphImage(reader io.Reader, glyph *Glyph) error {
	// Calculate expected image size (4bpp = 4 bits per pixel = 0.5 bytes per pixel)
	if glyph.GlyphWidth == 0 || glyph.GlyphHeight == 0 {
		glyph.GlyphImage = []byte{}
		return nil
	}

	imageSize := (int(glyph.GlyphWidth)*int(glyph.GlyphHeight) + 1) / 2
	if imageSize <= 0 || imageSize >= 10000 { // Reasonable size limit
		glyph.GlyphImage = []byte{}
		return nil
	}

	glyph.GlyphImage = make([]byte, imageSize)
	if _, err := io.ReadFull(reader, glyph.GlyphImage); err != nil {
		glyph.GlyphImage = []byte{}
		return err
	}

	return nil
}

// createEmptyGlyph creates an empty glyph structure
func (d *WFMFileDecoder) createEmptyGlyph() Glyph {
	return Glyph{
		GlyphClut:       0,
		GlyphHeight:     0,
		GlyphWidth:      0,
		GlyphHandakuten: 0,
		GlyphImage:      []byte{},
	}
}

// DecodeDialogs reads the dialog pointer table and dialog data
func (d *WFMFileDecoder) DecodeDialogues(reader io.Reader, header *WFMHeader) ([]uint16, []Dialogue, error) {
	dialoguePointers := make([]uint16, header.TotalDialogues)
	dialogues := make([]Dialogue, header.TotalDialogues)

	common.LogDebug(common.DebugReadingDialoguePointers, header.TotalDialogues)

	// Read dialog pointer table
	for i := uint16(0); i < header.TotalDialogues; i++ {
		if err := binary.Read(reader, binary.LittleEndian, &dialoguePointers[i]); err != nil {
			return nil, nil, fmt.Errorf("failed to read dialog pointer %d: %w", i, err)
		}
		if i < 10 { // Show first 10 pointers for debugging
			common.LogDebug(common.DebugDialoguePointer, i, dialoguePointers[i], dialoguePointers[i])
		}
	}

	// Calculate base offset for dialogue data (start of dialogue pointer table)
	dialogueTableStart := int64(header.DialoguePointerTable)

	// Read dialogue data using pointers
	for i := uint16(0); i < header.TotalDialogues; i++ {
		pointer := dialoguePointers[i]

		// Skip null pointers
		if pointer == 0 {
			dialogues[i] = Dialogue{Data: []byte{}}
			continue
		}

		// Calculate absolute offset: base address + relative pointer
		absoluteOffset := dialogueTableStart + int64(pointer)

		// Create a seeker from the reader if possible
		if seeker, ok := reader.(io.ReadSeeker); ok {
			// Seek to dialogue position
			_, err := seeker.Seek(absoluteOffset, io.SeekStart)
			if err != nil {
				common.LogWarn(common.WarnSeekToDialogue, i, absoluteOffset, err)
				dialogues[i] = Dialogue{Data: []byte{}}
				continue
			}

			// Read dialogue data until 0xFFFF terminator
			var dialogueData []byte
			for {
				var word uint16
				if err := binary.Read(reader, binary.LittleEndian, &word); err != nil {
					break // End of file or read error
				}

				// Check for terminator
				if word == 0xFFFF {
					break
				}

				// Add word to dialogue data (little endian)
				dialogueData = append(dialogueData, byte(word&0xFF), byte((word>>8)&0xFF))
			}

			dialogues[i] = Dialogue{Data: dialogueData}
		} else {
			// If we can't seek, create empty dialogue
			dialogues[i] = Dialogue{Data: []byte{}}
		}
	}

	return dialoguePointers, dialogues, nil
}

// UnpackGAM extracts data from a GAM file using LZ decompression
func (p *GAMProcessor) UnpackGAM(inputFile, outputFile string) error {
	// Open input GAM file
	file, err := os.Open(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open GAM file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Read and parse GAM file
	gam, err := p.readGAMFile(file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to read GAM file: %w", err)
	}

	// Decompress the data
	if err := p.decompressLZ(gam); err != nil {
		return fmt.Errorf("failed to decompress GAM data: %w", err)
	}

	// Write decompressed data to output file
	if err := p.writeDecompressedData(gam, outputFile); err != nil {
		return fmt.Errorf("failed to write decompressed data: %w", err)
	}

	common.LogInfo("GAM file unpacked successfully: %s -> %s", inputFile, outputFile)
	common.LogInfo("Original size: %d bytes, Decompressed size: %d bytes",
		len(gam.CompressedData), len(gam.UncompressedData))

	return nil
}

// readGAMFile reads and parses a GAM file
func (p *GAMProcessor) readGAMFile(file *os.File, fileSize int64) (*GAMFile, error) {
	gam := &GAMFile{
		OriginalSize: fileSize,
	}

	// Read header (8 bytes)
	if err := binary.Read(file, binary.LittleEndian, &gam.Header); err != nil {
		return nil, fmt.Errorf("failed to read GAM header: %w", err)
	}

	// Verify magic
	if string(gam.Header.Magic[:]) != "GAM" {
		return nil, fmt.Errorf("invalid GAM magic: expected 'GAM', got '%s'", string(gam.Header.Magic[:]))
	}

	// Read compressed data (rest of file)
	compressedSize := fileSize - 8
	gam.CompressedData = make([]byte, compressedSize)
	if _, err := io.ReadFull(file, gam.CompressedData); err != nil {
		return nil, fmt.Errorf("failed to read compressed data: %w", err)
	}

	common.LogDebug("GAM header read: magic=%s, uncompressed_size=%d",
		string(gam.Header.Magic[:]), gam.Header.UncompressedSize)

	return gam, nil
}

// decompressLZ implements the LZ decompression algorithm from the Python script
func (p *GAMProcessor) decompressLZ(gam *GAMFile) error {
	compressed := gam.CompressedData
	targetSize := int(gam.Header.UncompressedSize)

	// Initialize output buffer
	output := make([]byte, 0, targetSize)

	compPos := 0 // Position in compressed data

	common.LogDebug("Starting LZ decompression: target size = %d bytes", targetSize)

	for len(output) < targetSize && compPos < len(compressed) {
		// Check if we have enough bytes for bitmask
		if compPos+1 >= len(compressed) {
			break
		}

		// Read 2-byte bitmask (little endian)
		bitmaskBytes := binary.LittleEndian.Uint16(compressed[compPos : compPos+2])
		compPos += 2

		common.LogDebug("Bitmask at offset %d: 0x%04X", compPos-2, bitmaskBytes)

		// Process 16 bits of the bitmask
		for bit := 0; bit < 16 && len(output) < targetSize && compPos < len(compressed); bit++ {
			if (bitmaskBytes & (1 << bit)) != 0 {
				// Bit is 1: LZ reference
				if compPos+1 >= len(compressed) {
					break
				}

				lzByte1 := compressed[compPos]
				lzByte2 := compressed[compPos+1]
				compPos += 2

				// Calculate offset and length
				offset := int(lzByte1)
				length := int(lzByte2)

				common.LogDebug("LZ reference at %d: offset=%d, length=%d", compPos-2, offset, length)

				// Validate offset
				if offset > len(output) {
					return fmt.Errorf("invalid LZ offset: %d (output size: %d)", offset, len(output))
				}

				// Copy data from previous position
				srcPos := len(output) - offset
				for i := 0; i < length && len(output) < targetSize; i++ {
					if srcPos+i >= len(output) {
						return fmt.Errorf("invalid LZ reference: srcPos=%d, i=%d, output_len=%d", srcPos, i, len(output))
					}
					output = append(output, output[srcPos+i])
				}
			} else {
				// Bit is 0: literal byte
				if compPos >= len(compressed) {
					break
				}

				literal := compressed[compPos]
				compPos++
				output = append(output, literal)

				common.LogDebug("Literal byte at %d: 0x%02X", compPos-1, literal)
			}
		}
	}

	// Handle padding if output is smaller than expected
	if len(output) < targetSize {
		padding := targetSize - len(output)
		common.LogDebug("Adding %d bytes of padding", padding)
		for i := 0; i < padding; i++ {
			output = append(output, 0x00)
		}
	}

	// Truncate if output is larger than expected
	if len(output) > targetSize {
		common.LogDebug("Truncating output from %d to %d bytes", len(output), targetSize)
		output = output[:targetSize]
	}

	gam.UncompressedData = output
	common.LogDebug("LZ decompression completed: %d -> %d bytes", len(gam.CompressedData), len(output))

	return nil
}

// writeDecompressedData writes decompressed data to file
func (p *GAMProcessor) writeDecompressedData(gam *GAMFile, outputFile string) error {
	return os.WriteFile(outputFile, gam.UncompressedData, 0644)
}
