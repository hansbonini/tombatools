// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains decoders for reading and parsing WFM file structures and data.
package pkg

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/hansbonini/tombatools/pkg/psx"
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

// NewCDProcessor creates a new CD processor instance
func NewCDProcessor() *CDFileProcessor {
	return &CDFileProcessor{}
}

// NewFLAProcessor creates a new FLA processor instance
func NewFLAProcessor() *FLAProcessor {
	return &FLAProcessor{}
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

// Dump extracts files from a CD image file (.bin format) using mkpsxiso-style parsing
func (p *CDFileProcessor) Dump(inputFile string, outputDir string) error {
	common.LogDebug("Starting CD dump operation: %s -> %s", inputFile, outputDir)

	// Create CD reader using the new mkpsxiso-style implementation
	reader, err := psx.NewCDReader(inputFile)
	if err != nil {
		return fmt.Errorf("failed to open CD image file: %w", err)
	}
	defer reader.Close()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Validate ISO9660 format
	if err := reader.ValidateISO9660(); err != nil {
		return fmt.Errorf("invalid ISO9660 image: %w", err)
	}

	// Read and validate ISO descriptor
	descriptor, err := reader.ReadISODescriptor()
	if err != nil {
		return fmt.Errorf("failed to read ISO descriptor: %w", err)
	}

	common.LogDebug("ISO9660 file system detected")
	common.LogDebug("Volume ID: %s", string(descriptor.VolumeID[:]))
	common.LogDebug("Volume size: %d sectors", descriptor.VolumeSpaceSizeLSB)

	// Parse root directory from descriptor using mkpsxiso method
	rootLBA := common.ExtractLBAFromDirRecord(descriptor.RootDirRecord[:])
	rootSize := common.ExtractSizeFromDirRecord(descriptor.RootDirRecord[:])

	common.LogDebug("Root directory: LBA %d, Size %d bytes", rootLBA, rootSize)

	// Extract files using the new directory parsing method
	files, err := p.extractAllFiles(reader, rootLBA, rootSize, outputDir)
	if err != nil {
		return fmt.Errorf("failed to extract files: %w", err)
	}

	fmt.Printf("\nExtracted %d files successfully!\n", len(files))

	return nil
}

// extractAllFiles extracts all files using mkpsxiso-style directory parsing
func (p *CDFileProcessor) extractAllFiles(reader *psx.CDReader, rootLBA uint32, rootSize uint32, outputDir string) ([]psx.CDFileEntry, error) {
	var allFiles []psx.CDFileEntry
	validFiles := 0
	extractedFiles := 0

	fmt.Printf("Parsing directory entries...\n")

	// Parse root directory using the new method
	files, err := reader.ParseDirectoryEntries(int64(rootLBA), rootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root directory: %w", err)
	}

	// Process all files found in root directory
	for _, file := range files {
		validFiles++

		if common.VerboseMode {
			fmt.Printf("ID: %04X | MSF: %s | LBA: %08d | Size: %10d | %s\n",
				validFiles, file.MSF, file.LBA, file.Size, file.Name)
		}

		if !file.IsDir && file.Size > 0 {
			// Extract regular file
			outputPath := filepath.Join(outputDir, file.Name)

			err := reader.ExtractFile(file.LBA, file.Size, outputPath)
			if err != nil {
				if common.VerboseMode {
					fmt.Printf("  WARNING: Failed to extract %s: %v\n", file.Name, err)
				} else {
					common.LogDebug("Failed to extract %s: %v", file.Name, err)
				}
				continue
			}

			extractedFiles++
			fmt.Printf("Extracted: %s\n", file.Name)

		} else if file.IsDir && file.Name != "." && file.Name != ".." {
			// Process subdirectory recursively
			common.LogDebug("Processing directory: %s", file.Name)

			dirPath := filepath.Join(outputDir, file.Name)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				common.LogDebug("Failed to create directory %s: %v", dirPath, err)
				continue
			}

			// Parse subdirectory entries
			subFiles, err := reader.ParseDirectoryEntries(int64(file.LBA), file.Size)
			if err != nil {
				common.LogDebug("Failed to parse subdirectory %s: %v", file.Name, err)
				continue
			}

			// Extract files from subdirectory
			for _, subFile := range subFiles {
				if subFile.Name == "." || subFile.Name == ".." {
					continue
				}

				validFiles++

				if common.VerboseMode {
					fmt.Printf("ID: %04X | MSF: %s | LBA: %08d | Size: %10d | %s/%s\n",
						validFiles, subFile.MSF, subFile.LBA, subFile.Size, file.Name, subFile.Name)
				}

				if !subFile.IsDir && subFile.Size > 0 {
					outputPath := filepath.Join(dirPath, subFile.Name)

					err := reader.ExtractFile(subFile.LBA, subFile.Size, outputPath)
					if err != nil {
						if common.VerboseMode {
							fmt.Printf("  WARNING: Failed to extract %s/%s: %v\n", file.Name, subFile.Name, err)
						} else {
							common.LogDebug("Failed to extract %s/%s: %v", file.Name, subFile.Name, err)
						}
						continue
					}

					extractedFiles++
					fmt.Printf("Extracted: %s/%s\n", file.Name, subFile.Name)
				}

				// Add to file list for tracking
				subFile.Path = file.Name
				allFiles = append(allFiles, subFile)
			}
		}

		// Add to file list for tracking
		allFiles = append(allFiles, file)
	}

	fmt.Printf("\nTotal valid entries found: %d\n", validFiles)
	fmt.Printf("Files extracted: %d\n", extractedFiles)

	return allFiles, nil
}

// ReadFLAEntry reads a single File Link Address entry from the reader
// Each entry is 8 bytes: 4-byte MSF timecode (big-endian) + 4-byte file size (little-endian)
func (p *FLAProcessor) ReadFLAEntry(reader io.Reader) (*FileLinkAddressEntry, error) {
	entry := &FileLinkAddressEntry{}

	// Read MSF timecode (4 bytes, big-endian)
	if err := binary.Read(reader, binary.BigEndian, &entry.Timecode); err != nil {
		return nil, fmt.Errorf("failed to read MSF timecode: %w", err)
	}

	// Read file size (4 bytes, little-endian)
	if err := binary.Read(reader, binary.LittleEndian, &entry.FileSize); err != nil {
		return nil, fmt.Errorf("failed to read file size: %w", err)
	}

	return entry, nil
}

// ReadFLATable reads multiple FLA entries from the reader
func (p *FLAProcessor) ReadFLATable(reader io.Reader, count uint32, offset uint32) (*FileLinkAddressTable, error) {
	table := &FileLinkAddressTable{
		Offset:  offset,
		Count:   count,
		Entries: make([]FileLinkAddressEntry, count),
	}

	common.LogDebug("Reading FLA table: %d entries at offset 0x%X", count, offset)

	for i := uint32(0); i < count; i++ {
		entry, err := p.ReadFLAEntry(reader)
		if err != nil {
			return nil, fmt.Errorf("failed to read FLA entry %d: %w", i, err)
		}

		table.Entries[i] = *entry

		if common.VerboseMode {
			common.LogDebug("FLA Entry %d: %s", i, entry.String())
		}
	}

	return table, nil
}

// AnalyzeCDImage analyzes a CD image and extracts the FLA table from MAIN0.EXE
func (p *FLAProcessor) AnalyzeCDImage(imagePath string) (*FileLinkAddressTable, error) {
	common.LogDebug("Opening CD image: %s", imagePath)

	// Create CD reader
	reader, err := psx.NewCDReader(imagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CD image: %w", err)
	}
	defer reader.Close()

	// Validate ISO9660 format
	if err := reader.ValidateISO9660(); err != nil {
		return nil, fmt.Errorf("invalid ISO9660 image: %w", err)
	}

	// Read ISO descriptor
	descriptor, err := reader.ReadISODescriptor()
	if err != nil {
		return nil, fmt.Errorf("failed to read ISO descriptor: %w", err)
	}

	common.LogDebug("ISO9660 validated successfully")

	// Parse root directory
	rootLBA := common.ExtractLBAFromDirRecord(descriptor.RootDirRecord[:])
	rootSize := common.ExtractSizeFromDirRecord(descriptor.RootDirRecord[:])

	// Find and extract MAIN0.EXE
	exeData, err := p.extractMainExecutable(reader, rootLBA, rootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to extract MAIN0.EXE: %w", err)
	}

	common.LogDebug("MAIN0.EXE extracted successfully, size: %d bytes", len(exeData))

	// Analyze the executable and extract FLA table
	table, err := p.extractFLAFromExecutable(exeData)
	if err != nil {
		return nil, fmt.Errorf("failed to extract FLA table: %w", err)
	}

	return table, nil
}

// extractMainExecutable finds and extracts MAIN0.EXE from the CD image
func (p *FLAProcessor) extractMainExecutable(reader *psx.CDReader, rootLBA uint32, rootSize uint32) ([]byte, error) {
	// Parse root directory entries
	files, err := reader.ParseDirectoryEntries(int64(rootLBA), rootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root directory: %w", err)
	}

	// Look for EXE directory
	var exeDirFile *psx.CDFileEntry
	for _, file := range files {
		if file.IsDir && file.Name == "EXE" {
			exeDirFile = &file
			break
		}
	}

	if exeDirFile == nil {
		return nil, fmt.Errorf("EXE directory not found in CD image")
	}

	common.LogDebug("Found EXE directory at LBA %d", exeDirFile.LBA)

	// Parse EXE directory
	exeFiles, err := reader.ParseDirectoryEntries(int64(exeDirFile.LBA), exeDirFile.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to parse EXE directory: %w", err)
	}

	// Look for MAIN0.EXE
	var main0File *psx.CDFileEntry
	for _, file := range exeFiles {
		if !file.IsDir && file.Name == "MAIN0.EXE" {
			main0File = &file
			break
		}
	}

	if main0File == nil {
		return nil, fmt.Errorf("MAIN0.EXE not found in EXE directory")
	}

	common.LogDebug("Found MAIN0.EXE at LBA %d, size: %d bytes", main0File.LBA, main0File.Size)

	// Read the executable data
	exeData, err := p.readFileDataFromCD(reader, main0File.LBA, main0File.Size)
	if err != nil {
		return nil, fmt.Errorf("failed to read MAIN0.EXE data: %w", err)
	}

	return exeData, nil
}

// extractFLAFromExecutable analyzes a PlayStation executable and extracts the FLA table
func (p *FLAProcessor) extractFLAFromExecutable(exeData []byte) (*FileLinkAddressTable, error) {
	// For now, we'll implement a basic pattern search for FLA table
	// The FLA table typically starts with recognizable MSF patterns
	// This is a simplified implementation that looks for potential FLA entries

	common.LogDebug("Analyzing executable for FLA table, size: %d bytes", len(exeData))

	// Look for potential FLA table by searching for MSF-like patterns
	// We'll search for sequences that look like valid MSF timecodes
	offset, count := p.findFLATableLocation(exeData)

	if offset == 0 || count == 0 {
		return nil, fmt.Errorf("FLA table not found in executable")
	}

	common.LogDebug("Found potential FLA table at offset 0x%X with %d entries", offset, count)

	// Create a reader from the executable data at the found offset
	tableData := exeData[offset:]
	reader := bytes.NewReader(tableData)

	// Read the FLA table
	table, err := p.ReadFLATable(reader, count, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to read FLA table: %w", err)
	}

	return table, nil
}

// findFLATableLocation searches for the FLA table location in the executable
// For the EU version, the FLA table is located at offset 0x6E6F0 in MAIN0.EXE
func (p *FLAProcessor) findFLATableLocation(exeData []byte) (uint32, uint32) {
	// Known offset for EU version MAIN0.EXE
	tableOffset := uint32(0x6E6F0)

	common.LogDebug("Using known FLA table offset: 0x%X", tableOffset)

	// Check if the offset is within the executable bounds
	if int(tableOffset) >= len(exeData) {
		common.LogDebug("FLA table offset 0x%X is beyond executable size %d", tableOffset, len(exeData))
		return 0, 0
	}

	// Debug: Show the raw bytes at the known offset
	if int(tableOffset)+32 <= len(exeData) {
		rawBytes := exeData[tableOffset : tableOffset+32]
		common.LogDebug("Raw bytes at offset 0x%X: %02X", tableOffset, rawBytes)
	}

	// Try to count valid entries from the known offset (more permissive)
	count := p.countValidFLAEntries(exeData[tableOffset:])

	if count >= 1 {
		common.LogDebug("Found FLA table at known offset 0x%X with %d entries", tableOffset, count)
		return tableOffset, count
	}

	common.LogDebug("Data at offset 0x%X doesn't have valid FLA entries, trying pattern search", tableOffset)

	return tableOffset, count
}

// findFLATableByPattern is a fallback method that searches for FLA table patterns
func (p *FLAProcessor) findFLATableByPattern(exeData []byte) (uint32, uint32) {
	// Start searching from a reasonable offset in the executable
	startOffset := 0x2000 // Skip PSX-EXE header and initial code
	entrySize := 8        // Each FLA entry is 8 bytes

	common.LogDebug("Falling back to pattern search starting from offset 0x%X", startOffset)

	// Look for the first valid-looking MSF sequence
	for i := startOffset; i < len(exeData)-entrySize*10; i += 4 { // Align to 4-byte boundaries
		// Check if this could be the start of an FLA table
		if p.looksLikeFLATable(exeData[i:], 10) { // Check first 10 entries
			// Count how many consecutive valid entries we have
			count := p.countValidFLAEntries(exeData[i:])
			if count >= 5 { // Need at least 5 valid entries to consider it a table
				common.LogDebug("Found FLA table by pattern at offset 0x%X with %d entries", i, count)
				return uint32(i), count
			}
		}
	}

	return 0, 0
}

// looksLikeFLATable checks if data at offset looks like an FLA table
func (p *FLAProcessor) looksLikeFLATable(data []byte, maxEntries int) bool {
	if len(data) < 8*maxEntries {
		return false
	}

	validEntries := 0
	for i := 0; i < maxEntries && i*8+8 <= len(data); i++ {
		offset := i * 8

		// Extract MSF components (big-endian)
		minutes := data[offset]
		seconds := data[offset+1]
		sectors := data[offset+2]

		// Extract file size (little-endian)
		size := binary.LittleEndian.Uint32(data[offset+4 : offset+8])

		// Check if this looks like a valid MSF timecode and file size
		if p.isValidMSF(minutes, seconds, sectors) && p.isReasonableFileSize(size) {
			validEntries++
		}
	}

	// Consider it a valid FLA table if at least 70% of entries look valid
	return float64(validEntries)/float64(maxEntries) >= 0.7
}

// countValidFLAEntries counts consecutive valid FLA entries
func (p *FLAProcessor) countValidFLAEntries(data []byte) uint32 {
	count := uint32(0)

	for i := 0; i*8+8 <= len(data); i++ {
		offset := i * 8

		// Extract MSF components (big-endian)
		minutes := data[offset]
		seconds := data[offset+1]
		sectors := data[offset+2]

		// Extract file size (little-endian)
		size := binary.LittleEndian.Uint32(data[offset+4 : offset+8])

		// Check if this looks like a valid entry
		if p.isValidMSF(minutes, seconds, sectors) && p.isReasonableFileSize(size) {
			count++
		} else {
			break // Stop at first invalid entry
		}
	}

	return count
}

// isValidMSF checks if MSF components are valid (in BCD format)
func (p *FLAProcessor) isValidMSF(minutes, seconds, sectors byte) bool {
	// Convert BCD to decimal for validation
	minutesBCD := int(minutes>>4)*10 + int(minutes&0x0F)
	secondsBCD := int(seconds>>4)*10 + int(seconds&0x0F)
	sectorsBCD := int(sectors>>4)*10 + int(sectors&0x0F)

	return minutesBCD <= 99 && secondsBCD <= 59 && sectorsBCD <= 74
}

// isReasonableFileSize checks if file size is reasonable for a CD file
func (p *FLAProcessor) isReasonableFileSize(size uint32) bool {
	// File size should be reasonable (not 0, not too large for a CD)
	return size > 0 && size <= 700*1024*1024 // Max 700MB (CD capacity)
}

// readFileDataFromCD reads file data from CD image into memory
// This method reads directly from sectors to avoid extraction issues
func (p *FLAProcessor) readFileDataFromCD(reader *psx.CDReader, lba uint32, fileSize uint32) ([]byte, error) {
	common.LogDebug("Reading file data from LBA %d, size %d bytes", lba, fileSize)

	// Calculate number of sectors needed (each sector has 2048 bytes of data)
	sectorsNeeded := (fileSize + 2047) / 2048

	common.LogDebug("Need to read %d sectors starting from LBA %d", sectorsNeeded, lba)

	// Allocate buffer for all data
	data := make([]byte, 0, fileSize)

	// Read sector by sector
	for i := uint32(0); i < sectorsNeeded; i++ {
		currentLBA := lba + i

		// Seek to the sector
		if err := reader.SeekToSector(int64(currentLBA)); err != nil {
			return nil, fmt.Errorf("failed to seek to sector %d: %w", currentLBA, err)
		}

		// Read the sector data (2048 bytes per sector)
		sectorData := make([]byte, 2048)
		bytesRead, err := reader.ReadBytes(sectorData)
		if err != nil {
			return nil, fmt.Errorf("failed to read sector %d: %w", currentLBA, err)
		}

		// Determine how much data to take from this sector
		bytesToTake := uint32(bytesRead)
		if uint32(len(data))+bytesToTake > fileSize {
			bytesToTake = fileSize - uint32(len(data))
		}

		// Append data to our buffer
		data = append(data, sectorData[:bytesToTake]...)

		common.LogDebug("Read sector %d: %d bytes, total so far: %d bytes", currentLBA, bytesToTake, len(data))

		// Break if we have enough data
		if uint32(len(data)) >= fileSize {
			break
		}
	}

	common.LogDebug("Successfully read %d bytes from CD", len(data))

	return data, nil
}
