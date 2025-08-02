// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains decoders for reading and parsing WFM file structures and data.
package pkg

import (
	"encoding/binary"
	"fmt"
	"io"

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
	glyphPointers := make([]uint16, header.TotalGlyphs)
	glyphs := make([]Glyph, header.TotalGlyphs)

	// Read glyph pointer table
	for i := uint16(0); i < header.TotalGlyphs; i++ {
		if err := binary.Read(reader, binary.LittleEndian, &glyphPointers[i]); err != nil {
			return nil, nil, fmt.Errorf("failed to read glyph pointer %d: %w", i, err)
		}
	}

	// Read glyph data - each glyph has a specific structure
	// Since we don't have the exact specification, we'll read what we can
	for i := uint16(0); i < header.TotalGlyphs; i++ {
		glyph := Glyph{}

		// Try to read glyph structure: GlyphClut, GlyphHeight, GlyphWidth, GlyphHandakuten
		if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphClut); err != nil {
			// If we can't read the structure, create empty glyph
			glyph = Glyph{
				GlyphClut:       0,
				GlyphHeight:     0,
				GlyphWidth:      0,
				GlyphHandakuten: 0,
				GlyphImage:      []byte{},
			}
		} else {
			// Continue reading the rest of the glyph structure
			if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphHeight); err != nil {
				glyph.GlyphHeight = 0
			}
			if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphWidth); err != nil {
				glyph.GlyphWidth = 0
			}
			if err := binary.Read(reader, binary.LittleEndian, &glyph.GlyphHandakuten); err != nil {
				glyph.GlyphHandakuten = 0
			}

			// Calculate expected image size (4bpp = 4 bits per pixel = 0.5 bytes per pixel)
			if glyph.GlyphWidth > 0 && glyph.GlyphHeight > 0 {
				imageSize := (int(glyph.GlyphWidth)*int(glyph.GlyphHeight) + 1) / 2
				if imageSize > 0 && imageSize < 10000 { // Reasonable size limit
					glyph.GlyphImage = make([]byte, imageSize)
					if _, err := io.ReadFull(reader, glyph.GlyphImage); err != nil {
						glyph.GlyphImage = []byte{}
					}
				}
			}
		}

		glyphs[i] = glyph
	}

	return glyphPointers, glyphs, nil
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
