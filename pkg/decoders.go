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
	"sort"

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

		// Convert timecode to decimal string for comparison
		entry.TimecodeDecimal = entry.Timecode.ToDecimalString()
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

	// Find and extract MAIN0.EXE with LBA information
	exeData, main0LBA, err := p.extractMainExecutableWithLBA(reader, rootLBA, rootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to extract MAIN0.EXE: %w", err)
	}

	common.LogDebug("MAIN0.EXE extracted successfully, size: %d bytes", len(exeData))

	// Analyze the executable and extract FLA table with correct absolute offset
	table, err := p.extractFLAFromExecutableWithLBA(exeData, main0LBA)
	if err != nil {
		return nil, fmt.Errorf("failed to extract FLA table: %w", err)
	}

	// Collect all files from CD for linking
	cdFiles, err := p.collectAllCDFiles(reader, rootLBA, rootSize)
	if err != nil {
		common.LogDebug("Warning: could not collect CD files for linking: %v", err)
		// Continue without linking
	} else {
		// Link FLA entries with CD files
		p.linkFLAWithCDFiles(table, cdFiles)
	}

	return table, nil
}

// extractMainExecutableWithLBA finds and extracts MAIN0.EXE from the CD image, returning both data and LBA
func (p *FLAProcessor) extractMainExecutableWithLBA(reader *psx.CDReader, rootLBA uint32, rootSize uint32) ([]byte, uint32, error) {
	// Parse root directory entries
	files, err := reader.ParseDirectoryEntries(int64(rootLBA), rootSize)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse root directory: %w", err)
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
		return nil, 0, fmt.Errorf("EXE directory not found in CD image")
	}

	common.LogDebug("Found EXE directory at LBA %d", exeDirFile.LBA)

	// Parse EXE directory
	exeFiles, err := reader.ParseDirectoryEntries(int64(exeDirFile.LBA), exeDirFile.Size)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse EXE directory: %w", err)
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
		return nil, 0, fmt.Errorf("MAIN0.EXE not found in EXE directory")
	}

	common.LogDebug("Found MAIN0.EXE at LBA %d, size: %d bytes", main0File.LBA, main0File.Size)

	// Read the executable data
	exeData, err := p.readFileDataFromCD(reader, main0File.LBA, main0File.Size)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read MAIN0.EXE data: %w", err)
	}

	return exeData, main0File.LBA, nil
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

// extractFLAFromExecutableWithLBA analyzes a PlayStation executable and extracts the FLA table with correct absolute offset
func (p *FLAProcessor) extractFLAFromExecutableWithLBA(exeData []byte, main0LBA uint32) (*FileLinkAddressTable, error) {
	// For now, we'll implement a basic pattern search for FLA table
	// The FLA table typically starts with recognizable MSF patterns

	common.LogDebug("Analyzing executable for FLA table, size: %d bytes", len(exeData))

	// Look for potential FLA table by searching for MSF-like patterns
	// We'll search for sequences that look like valid MSF timecodes
	relativeOffset, count := p.findFLATableLocation(exeData)

	if relativeOffset == 0 || count == 0 {
		return nil, fmt.Errorf("FLA table not found in executable")
	}

	// Calculate absolute offset in CD image: (LBA * sector_size) + relative_offset_in_exe
	absoluteOffset := (main0LBA * 2048) + relativeOffset

	common.LogDebug("Found potential FLA table at relative offset 0x%X (absolute: 0x%X) with %d entries", relativeOffset, absoluteOffset, count)

	// Create a reader from the executable data at the found offset
	tableData := exeData[relativeOffset:]
	reader := bytes.NewReader(tableData)

	// Read the FLA table with the correct absolute offset
	table, err := p.ReadFLATable(reader, count, absoluteOffset)
	if err != nil {
		return nil, fmt.Errorf("failed to read FLA table: %w", err)
	}

	return table, nil
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

// collectAllCDFiles collects all files from the CD image for FLA linking
func (p *FLAProcessor) collectAllCDFiles(reader *psx.CDReader, rootLBA uint32, rootSize uint32) ([]CDFileInfo, error) {
	var allFiles []CDFileInfo

	common.LogDebug("Collecting all files from CD for FLA linking")

	// Parse root directory entries
	files, err := reader.ParseDirectoryEntries(int64(rootLBA), rootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to parse root directory: %w", err)
	}

	// Process all files and directories recursively
	for _, file := range files {
		if file.Name == "." || file.Name == ".." {
			continue
		}

		if file.IsDir {
			// Process subdirectory recursively
			subFiles, err := p.collectFilesFromDirectory(reader, file.Name, file.LBA, file.Size)
			if err != nil {
				common.LogDebug("Warning: failed to collect files from directory %s: %v", file.Name, err)
				continue
			}
			allFiles = append(allFiles, subFiles...)
		} else {
			// Add regular file
			cdFile := CDFileInfo{
				Name:     file.Name,
				FullPath: file.Name,
				LBA:      file.LBA,
				Size:     file.Size,
				MSF:      file.MSF,
			}
			allFiles = append(allFiles, cdFile)
		}
	}

	common.LogDebug("Collected %d files from CD image", len(allFiles))
	return allFiles, nil
}

// collectFilesFromDirectory recursively collects files from a directory
func (p *FLAProcessor) collectFilesFromDirectory(reader *psx.CDReader, parentPath string, lba uint32, size uint32) ([]CDFileInfo, error) {
	var files []CDFileInfo

	// Parse directory entries
	dirFiles, err := reader.ParseDirectoryEntries(int64(lba), size)
	if err != nil {
		return nil, err
	}

	for _, file := range dirFiles {
		if file.Name == "." || file.Name == ".." {
			continue
		}

		fullPath := parentPath + "/" + file.Name

		if file.IsDir {
			// Recurse into subdirectory
			subFiles, err := p.collectFilesFromDirectory(reader, fullPath, file.LBA, file.Size)
			if err != nil {
				common.LogDebug("Warning: failed to collect files from directory %s: %v", fullPath, err)
				continue
			}
			files = append(files, subFiles...)
		} else {
			// Add regular file
			cdFile := CDFileInfo{
				Name:     file.Name,
				FullPath: fullPath,
				LBA:      file.LBA,
				Size:     file.Size,
				MSF:      file.MSF,
			}
			files = append(files, cdFile)
		}
	}

	return files, nil
}

// linkFLAWithCDFiles links FLA entries with corresponding CD files based on MSF timecode
func (p *FLAProcessor) linkFLAWithCDFiles(table *FileLinkAddressTable, cdFiles []CDFileInfo) {
	common.LogDebug("Linking FLA entries with CD files")

	linkedCount := 0
	
	for i := range table.Entries {
		entry := &table.Entries[i]

		// Try to find matching file by MSF timecode
		for _, cdFile := range cdFiles {
			if entry.TimecodeDecimal == cdFile.MSF {
				// Found matching MSF timecode
				entry.LinkedFile = &CDFileInfo{
					Name:     cdFile.Name,
					FullPath: cdFile.FullPath,
					LBA:      cdFile.LBA,
					Size:     cdFile.Size,
					MSF:      cdFile.MSF,
				}
				linkedCount++
				common.LogDebug("Linked FLA entry %d (%s) with file: %s", i, entry.TimecodeDecimal, cdFile.FullPath)
				break
			}
		}
	}

	common.LogDebug("Successfully linked %d of %d FLA entries with CD files", linkedCount, len(table.Entries))
}

// CompareFLATables compares two FLA tables and returns a list of differences
func (p *FLAProcessor) CompareFLATables(originalTable, modifiedTable *FileLinkAddressTable) ([]FLADifference, error) {
	var differences []FLADifference

	// Check if tables have the same number of entries
	if originalTable.Count != modifiedTable.Count {
		return nil, fmt.Errorf("FLA tables have different entry counts: original=%d, modified=%d", 
			originalTable.Count, modifiedTable.Count)
	}

	common.LogDebug("Comparing %d FLA entries between original and modified tables", originalTable.Count)

	// Compare each entry
	for i := uint32(0); i < originalTable.Count; i++ {
		originalEntry := originalTable.Entries[i]
		modifiedEntry := modifiedTable.Entries[i]

		var diff FLADifference
		diff.EntryIndex = i
		hasChanges := false

		// Check if timecode changed
		if originalEntry.Timecode.Minutes != modifiedEntry.Timecode.Minutes ||
			originalEntry.Timecode.Seconds != modifiedEntry.Timecode.Seconds ||
			originalEntry.Timecode.Sectors != modifiedEntry.Timecode.Sectors {
			diff.TimecodeChanged = true
			hasChanges = true
		}

		// Check if file size changed in FLA table
		if originalEntry.FileSize != modifiedEntry.FileSize {
			diff.SizeChanged = true
			hasChanges = true
		}

		// Additional check: if files are linked, compare actual file sizes from CD
		if originalEntry.LinkedFile != nil && modifiedEntry.LinkedFile != nil {
			if originalEntry.LinkedFile.Size != modifiedEntry.LinkedFile.Size {
				common.LogDebug("Real file size difference detected for %s: original=%d, modified=%d", 
					originalEntry.LinkedFile.FullPath, originalEntry.LinkedFile.Size, modifiedEntry.LinkedFile.Size)
				
				// If the FLA table hasn't been updated to reflect the real file size difference
				if !diff.SizeChanged {
					diff.SizeChanged = true
					hasChanges = true
					common.LogDebug("FLA table needs update for file %s", originalEntry.LinkedFile.FullPath)
				}
			}
		}

		// If there are changes, add to differences list
		if hasChanges {
			var changes []string
			if diff.TimecodeChanged {
				changes = append(changes, fmt.Sprintf("MSF: %s → %s", 
					originalEntry.Timecode.String(), modifiedEntry.Timecode.String()))
			}
			if diff.SizeChanged {
				originalSize := originalEntry.FileSize
				modifiedSize := modifiedEntry.FileSize
				
				// Use real file sizes if available and different
				if originalEntry.LinkedFile != nil && modifiedEntry.LinkedFile != nil {
					if originalEntry.LinkedFile.Size != modifiedEntry.LinkedFile.Size {
						originalSize = originalEntry.LinkedFile.Size
						modifiedSize = modifiedEntry.LinkedFile.Size
					}
				}
				
				changes = append(changes, fmt.Sprintf("Size: %d → %d bytes", originalSize, modifiedSize))
			}
			
			diff.Description = fmt.Sprintf("Entry %04X: %s", i, fmt.Sprintf("%v", changes))
			differences = append(differences, diff)

			common.LogDebug("Found difference in entry %04X: %s", i, diff.Description)
		}
	}

	common.LogDebug("Found %d differences between FLA tables", len(differences))
	return differences, nil
}

// CompareCDFiles compares specific files between two CD images to detect size differences
func (p *FLAProcessor) CompareCDFiles(originalImagePath, modifiedImagePath string, originalTable, modifiedTable *FileLinkAddressTable) ([]FLADifference, error) {
	var differences []FLADifference
	
	common.LogDebug("Comparing actual files between CD images")
	
	// Open both CD readers
	originalReader, err := psx.NewCDReader(originalImagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open original CD image: %w", err)
	}
	defer originalReader.Close()
	
	modifiedReader, err := psx.NewCDReader(modifiedImagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open modified CD image: %w", err)
	}
	defer modifiedReader.Close()
	
	// Get file lists from both CDs
	originalDescriptor, err := originalReader.ReadISODescriptor()
	if err != nil {
		return nil, fmt.Errorf("failed to read original ISO descriptor: %w", err)
	}
	
	modifiedDescriptor, err := modifiedReader.ReadISODescriptor()
	if err != nil {
		return nil, fmt.Errorf("failed to read modified ISO descriptor: %w", err)
	}
	
	originalRootLBA := common.ExtractLBAFromDirRecord(originalDescriptor.RootDirRecord[:])
	originalRootSize := common.ExtractSizeFromDirRecord(originalDescriptor.RootDirRecord[:])
	
	modifiedRootLBA := common.ExtractLBAFromDirRecord(modifiedDescriptor.RootDirRecord[:])
	modifiedRootSize := common.ExtractSizeFromDirRecord(modifiedDescriptor.RootDirRecord[:])
	
	// Collect files from both CDs
	originalFiles, err := p.collectAllCDFiles(originalReader, originalRootLBA, originalRootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to collect original CD files: %w", err)
	}
	
	modifiedFiles, err := p.collectAllCDFiles(modifiedReader, modifiedRootLBA, modifiedRootSize)
	if err != nil {
		return nil, fmt.Errorf("failed to collect modified CD files: %w", err)
	}
	
	// Create maps for quick lookup
	originalFileMap := make(map[string]*CDFileInfo)
	modifiedFileMap := make(map[string]*CDFileInfo)
	
	for i := range originalFiles {
		originalFileMap[originalFiles[i].FullPath] = &originalFiles[i]
	}
	
	for i := range modifiedFiles {
		modifiedFileMap[modifiedFiles[i].FullPath] = &modifiedFiles[i]
	}
	
	common.LogDebug("Comparing file sizes and positions between CDs")
	
	// Check each FLA entry to see if its linked file has changed
	for i := uint32(0); i < originalTable.Count; i++ {
		originalEntry := originalTable.Entries[i]
		
		// Skip if not linked to a file
		if originalEntry.LinkedFile == nil {
			continue
		}
		
		originalPath := originalEntry.LinkedFile.FullPath
		
		// Get actual file info from both CDs
		originalFileInfo := originalFileMap[originalPath]
		modifiedFileInfo := modifiedFileMap[originalPath]
		
		if originalFileInfo == nil || modifiedFileInfo == nil {
			// File missing in one of the CDs
			if originalFileInfo != nil && modifiedFileInfo == nil {
				common.LogDebug("File removed in modified CD: %s", originalPath)
			} else if originalFileInfo == nil && modifiedFileInfo != nil {
				common.LogDebug("File added in modified CD: %s", originalPath)
			}
			continue
		}
		
		// Check if actual file sizes differ (this is what matters for recalculation)
		sizeChanged := originalFileInfo.Size != modifiedFileInfo.Size
		
		// Only include entries with real size changes that require FLA recalculation
		if sizeChanged {
			common.LogDebug("File size change detected: %s", originalPath)
			common.LogDebug("  Original: Size=%d", originalFileInfo.Size)
			common.LogDebug("  Modified: Size=%d", modifiedFileInfo.Size)
			
			diff := FLADifference{
				EntryIndex:      i,
				TimecodeChanged: originalFileInfo.MSF != modifiedFileInfo.MSF,
				SizeChanged:     true,
				Description:     fmt.Sprintf("Entry %04X: Size changed from %d to %d bytes for file %s", 
					i, originalFileInfo.Size, modifiedFileInfo.Size, originalPath),
			}
			differences = append(differences, diff)
			
			// Update the table entries with real file info for proper display
			if modifiedTable.Entries[i].LinkedFile != nil {
				modifiedTable.Entries[i].LinkedFile.Size = modifiedFileInfo.Size
				modifiedTable.Entries[i].LinkedFile.MSF = modifiedFileInfo.MSF
			}
		}
	}
	
	common.LogDebug("Found %d file differences between CDs", len(differences))
	return differences, nil
}

// RecalculateFLATable recalculates and updates the FLA table in the modified CD image
func (p *FLAProcessor) RecalculateFLATable(modifiedImagePath string, originalTable, modifiedTable *FileLinkAddressTable, differences []FLADifference) error {
	common.LogDebug("Starting FLA table recalculation for %s", modifiedImagePath)

	if len(differences) == 0 {
		common.LogDebug("No differences to recalculate")
		return nil
	}

	// Sort differences by entry index to process them in order
	sort.Slice(differences, func(i, j int) bool {
		return differences[i].EntryIndex < differences[j].EntryIndex
	})

	// Calculate cumulative offset for each file change
	var cumulativeOffset int64 = 0
	
	// Apply size changes and recalculate MSF positions
	for _, diff := range differences {
		originalEntry := originalTable.Entries[diff.EntryIndex]
		modifiedEntry := &modifiedTable.Entries[diff.EntryIndex]
		
		if originalEntry.LinkedFile != nil && modifiedEntry.LinkedFile != nil {
			// Calculate size difference
			sizeDiff := int64(modifiedEntry.LinkedFile.Size) - int64(originalEntry.LinkedFile.Size)
			cumulativeOffset += sizeDiff
			
			common.LogDebug("Entry %04X: Size changed by %d bytes, cumulative offset: %d", 
				diff.EntryIndex, sizeDiff, cumulativeOffset)
			
			// Update the file size in the current entry
			modifiedEntry.FileSize = modifiedEntry.LinkedFile.Size
			common.LogDebug("Updated entry %04X: FileSize %d -> %d", 
				diff.EntryIndex, originalEntry.FileSize, modifiedEntry.FileSize)
			
			// Convert sectors to bytes for calculation (each sector = 2048 bytes)
			sectorOffset := cumulativeOffset / 2048
			if cumulativeOffset%2048 != 0 {
				sectorOffset++ // Round up to next sector
			}
			
			// Update MSF positions for all subsequent entries
			for i := diff.EntryIndex + 1; i < originalTable.Count; i++ {
				if modifiedTable.Entries[i].LinkedFile != nil {
					originalMSF := originalTable.Entries[i].Timecode
					
					// Calculate new MSF by adding sector offset
					newTotalSectors := int64(originalMSF.ToSectors()) + sectorOffset
					if newTotalSectors < 0 {
						newTotalSectors = 0
					}
					
					// Convert back to MSF
					newMSF := MSFFromSectors(uint32(newTotalSectors))
					modifiedTable.Entries[i].Timecode = newMSF
					
					common.LogDebug("Updated entry %04X: MSF %s -> %s", 
						i, originalMSF.String(), newMSF.String())
				}
			}
		}
	}

	// Write the updated FLA table back to the CD image
	err := p.writeFLATableToCD(modifiedImagePath, modifiedTable)
	if err != nil {
		return fmt.Errorf("failed to write updated FLA table: %w", err)
	}

	common.LogDebug("Successfully updated FLA table with %d changes", len(differences))
	return nil
}

// writeFLATableToCD writes the updated FLA table back to the MAIN0.EXE within the CD image
func (p *FLAProcessor) writeFLATableToCD(imagePath string, table *FileLinkAddressTable) error {
	common.LogInfo("=== Starting FLA Table Write Operation ===")
	common.LogInfo("Target CD image: %s", imagePath)
	common.LogInfo("FLA table entries to write: %d", table.Count)
	
	// Step 1: Find MAIN0.EXE location in the CD
	reader, err := psx.NewCDReader(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open CD image for reading: %w", err)
	}
	defer reader.Close()

	// Validate ISO9660 format
	if err := reader.ValidateISO9660(); err != nil {
		return fmt.Errorf("invalid ISO9660 image: %w", err)
	}

	// Read ISO descriptor
	descriptor, err := reader.ReadISODescriptor()
	if err != nil {
		return fmt.Errorf("failed to read ISO descriptor: %w", err)
	}

	// Parse root directory
	rootLBA := common.ExtractLBAFromDirRecord(descriptor.RootDirRecord[:])
	rootSize := common.ExtractSizeFromDirRecord(descriptor.RootDirRecord[:])

	// Find MAIN0.EXE location
	_, main0LBA, err := p.extractMainExecutableWithLBA(reader, rootLBA, rootSize)
	if err != nil {
		return fmt.Errorf("failed to find MAIN0.EXE: %w", err)
	}

	// Calculate absolute offset within the CD image
	main0ExeOffset := (main0LBA * 2048) + 0x6E6F0
	
	common.LogInfo("MAIN0.EXE located at LBA: %d (byte offset: 0x%X)", main0LBA, main0LBA*2048)
	common.LogInfo("FLA table offset within MAIN0.EXE: 0x6E6F0")
	common.LogInfo("Calculated absolute FLA table offset in CD: 0x%X", main0ExeOffset)
	
	// Step 2: Close the reader since we'll need write access
	reader.Close()
	
	// Step 3: Prepare new FLA table data
	var newData []byte
	for i := uint32(0); i < table.Count; i++ {
		entry := table.Entries[i]
		
		// Create MSF bytes (4 bytes: MM:SS:FF:00)
		msfBytes := []byte{
			entry.Timecode.Minutes,
			entry.Timecode.Seconds, 
			entry.Timecode.Sectors,
			entry.Timecode.Unused,
		}
		
		// Create file size bytes (4 bytes, little-endian)
		sizeBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(sizeBytes, entry.FileSize)
		
		// Combine MSF and size
		entryData := append(msfBytes, sizeBytes...)
		newData = append(newData, entryData...)
		
		// Log specific entries for debugging
		if i < 5 || i == 0x15A || i >= table.Count-5 {
			common.LogDebug("Entry %04X: MSF %02X:%02X:%02X:00, Size %d (0x%08X)", 
				i, entry.Timecode.Minutes, entry.Timecode.Seconds, entry.Timecode.Sectors, entry.FileSize, entry.FileSize)
		}
	}
	
	common.LogInfo("Prepared %d bytes of FLA table data", len(newData))
	
	// Step 4: Get file info before opening for write
	fileInfo, err := os.Stat(imagePath)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	
	common.LogInfo("CD image file size: %d bytes, write target offset: 0x%X", fileInfo.Size(), main0ExeOffset)
	
	if int64(main0ExeOffset) >= fileInfo.Size() {
		return fmt.Errorf("target offset 0x%X is beyond file size %d", main0ExeOffset, fileInfo.Size())
	}
	
	// Step 5: Open the CD image file for writing with proper flags
	file, err := os.OpenFile(imagePath, os.O_RDWR|os.O_SYNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open CD image for writing: %w", err)
	}
	defer func() {
		// Ensure proper cleanup
		if syncErr := file.Sync(); syncErr != nil {
			common.LogDebug("Error during final sync: %v", syncErr)
		}
		file.Close()
	}()
	
	// Step 6: Seek to the target position
	seekPos, err := file.Seek(int64(main0ExeOffset), io.SeekStart)
	if err != nil {
		return fmt.Errorf("failed to seek to FLA table offset: %w", err)
	}
	
	common.LogInfo("Seeked to position: 0x%X (target: 0x%X)", seekPos, main0ExeOffset)
	
	// Step 7: Write the entire FLA table data at once
	bytesWritten, err := file.Write(newData)
	if err != nil {
		return fmt.Errorf("failed to write FLA table data: %w", err)
	}
	
	common.LogInfo("Successfully wrote %d bytes of FLA table data", bytesWritten)
	
	if bytesWritten != len(newData) {
		return fmt.Errorf("incomplete write: expected %d bytes, wrote %d bytes", len(newData), bytesWritten)
	}
	
	// Step 8: Force immediate sync to disk
	err = file.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync FLA table data to disk: %w", err)
	}
	
	common.LogInfo("Data successfully synced to disk")
	
	// Step 9: Verify the write by reading back the data
	_, err = file.Seek(int64(main0ExeOffset), io.SeekStart)
	if err != nil {
		common.LogDebug("Warning: Could not seek back for verification: %v", err)
	} else {
		verifyData := make([]byte, len(newData))
		readBytes, readErr := file.Read(verifyData)
		if readErr != nil {
			common.LogDebug("Warning: Could not read back for verification: %v", readErr)
		} else if readBytes != len(newData) {
			common.LogDebug("Warning: Verification read incomplete: %d/%d bytes", readBytes, len(newData))
		} else {
			// Compare written data with read-back data
			verifyMatches := true
			for i := 0; i < len(newData); i++ {
				if newData[i] != verifyData[i] {
					verifyMatches = false
					break
				}
			}
			
			if verifyMatches {
				common.LogInfo("✓ Verification successful: Written data matches read-back data")
			} else {
				common.LogInfo("✗ Verification failed: Written data does not match read-back data")
			}
		}
	}
	
	common.LogInfo("=== FLA Table Write Operation Complete ===")
	common.LogInfo("Result: %d FLA entries written to offset 0x%X in %s", table.Count, main0ExeOffset, imagePath)
	
	return nil
}

// SaveFLATableToFile saves the FLA table data to a binary file
func (p *FLAProcessor) SaveFLATableToFile(table *FileLinkAddressTable, filename string) error {
	common.LogDebug("Saving FLA table to file: %s", filename)
	
	// Create the output file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create FLA table file: %w", err)
	}
	defer file.Close()
	
	// Write each FLA entry
	for i := uint32(0); i < table.Count; i++ {
		entry := table.Entries[i]
		
		// Write MSF timecode (4 bytes: MM:SS:FF:00)
		msfBytes := []byte{
			entry.Timecode.Minutes,
			entry.Timecode.Seconds, 
			entry.Timecode.Sectors,
			entry.Timecode.Unused,
		}
		
		_, err = file.Write(msfBytes)
		if err != nil {
			return fmt.Errorf("failed to write MSF for entry %d: %w", i, err)
		}
		
		// Write file size (4 bytes, little-endian)
		err = binary.Write(file, binary.LittleEndian, entry.FileSize)
		if err != nil {
			return fmt.Errorf("failed to write file size for entry %d: %w", i, err)
		}
	}
	
	common.LogDebug("Successfully saved %d FLA entries to file %s", table.Count, filename)
	return nil
}
