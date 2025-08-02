// Package pkg provides functionality for processing GAM files from the Tomba! PlayStation game.
// This file contains structures and functions for GAM file format handling.
package pkg

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/hansbonini/tombatools/pkg/common"
)

// GAMHeader represents the 8-byte header of a GAM file
type GAMHeader struct {
	Magic           [3]byte // "GAM"
	Reserved        byte    // Padding byte (typically 0x00)
	UncompressedSize uint32  // Size of the decompressed data
}

// GAMFile represents a complete GAM file structure
type GAMFile struct {
	Header           GAMHeader
	CompressedData   []byte
	UncompressedData []byte
	OriginalSize     int64
}

// GAMProcessor handles GAM file operations (unpack/pack)
type GAMProcessor struct{}

// NewGAMProcessor creates a new GAM processor instance
func NewGAMProcessor() *GAMProcessor {
	return &GAMProcessor{}
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

// PackGAM creates a GAM file from uncompressed data using LZ compression
func (p *GAMProcessor) PackGAM(inputFile, outputFile string) error {
	// Read uncompressed data
	uncompressedData, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Create GAM structure
	gam := &GAMFile{
		Header: GAMHeader{
			Magic:           [3]byte{'G', 'A', 'M'},
			Reserved:        0x00,
			UncompressedSize: uint32(len(uncompressedData)),
		},
		UncompressedData: uncompressedData,
	}

	// Compress the data
	if err := p.compressLZ(gam); err != nil {
		return fmt.Errorf("failed to compress data: %w", err)
	}

	// Write GAM file
	if err := p.writeGAMFile(gam, outputFile); err != nil {
		return fmt.Errorf("failed to write GAM file: %w", err)
	}

	common.LogInfo("GAM file packed successfully: %s -> %s", inputFile, outputFile)
	common.LogInfo("Uncompressed size: %d bytes, Compressed size: %d bytes", 
		len(gam.UncompressedData), len(gam.CompressedData))

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
		bitmaskBytes := binary.LittleEndian.Uint16(compressed[compPos:compPos+2])
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

// compressLZ implements LZ compression (reverse of decompression)
func (p *GAMProcessor) compressLZ(gam *GAMFile) error {
	input := gam.UncompressedData
	output := make([]byte, 0)
	
	pos := 0
	
	common.LogDebug("Starting LZ compression: input size = %d bytes", len(input))
	
	for pos < len(input) {
		bitmask := uint16(0)
		bitmaskPos := len(output)
		output = append(output, 0, 0) // Reserve space for bitmask
		
		// Process up to 16 bytes/references
		for bit := 0; bit < 16 && pos < len(input); bit++ {
			// Find best match in previous data
			bestOffset, bestLength := p.findBestMatch(input, pos)
			
			if bestLength >= 2 && bestOffset <= 255 && bestLength <= 255 {
				// Use LZ reference
				bitmask |= (1 << bit)
				output = append(output, byte(bestOffset), byte(bestLength))
				pos += bestLength
				
				common.LogDebug("LZ reference: offset=%d, length=%d", bestOffset, bestLength)
			} else {
				// Use literal byte
				output = append(output, input[pos])
				pos++
				
				common.LogDebug("Literal byte: 0x%02X", input[pos-1])
			}
		}
		
		// Write bitmask in little endian
		binary.LittleEndian.PutUint16(output[bitmaskPos:bitmaskPos+2], bitmask)
		common.LogDebug("Bitmask: 0x%04X", bitmask)
	}
	
	gam.CompressedData = output
	common.LogDebug("LZ compression completed: %d -> %d bytes", len(input), len(output))
	
	return nil
}

// findBestMatch finds the best LZ match for current position
func (p *GAMProcessor) findBestMatch(data []byte, pos int) (offset, length int) {
	bestOffset := 0
	bestLength := 0
	
	// Search backwards for matches (up to 255 bytes back)
	maxOffset := pos
	if maxOffset > 255 {
		maxOffset = 255
	}
	
	for o := 1; o <= maxOffset; o++ {
		srcPos := pos - o
		matchLength := 0
		
		// Count matching bytes
		for matchLength < 255 && pos+matchLength < len(data) && 
			data[srcPos+matchLength%o] == data[pos+matchLength] {
			matchLength++
		}
		
		// Keep best match
		if matchLength > bestLength {
			bestOffset = o
			bestLength = matchLength
		}
	}
	
	return bestOffset, bestLength
}

// writeDecompressedData writes decompressed data to file
func (p *GAMProcessor) writeDecompressedData(gam *GAMFile, outputFile string) error {
	return os.WriteFile(outputFile, gam.UncompressedData, 0644)
}

// writeGAMFile writes a complete GAM file
func (p *GAMProcessor) writeGAMFile(gam *GAMFile, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()
	
	// Write header
	if err := binary.Write(file, binary.LittleEndian, gam.Header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	
	// Write compressed data
	if _, err := file.Write(gam.CompressedData); err != nil {
		return fmt.Errorf("failed to write compressed data: %w", err)
	}
	
	return nil
}
