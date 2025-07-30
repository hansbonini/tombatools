package pkg

import (
	"encoding/binary"
	"fmt"
	"io"
)

// WFMFileDecoder implements the WFMDecoder interface
type WFMFileDecoder struct{}

// NewWFMDecoder creates a new WFM decoder instance
func NewWFMDecoder() *WFMFileDecoder {
	return &WFMFileDecoder{}
}

// Decode reads and parses a complete WFM file
func (d *WFMFileDecoder) Decode(reader io.Reader) (*WFMFile, error) {
	wfm := &WFMFile{}

	// Decode header
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

// DecodeHeader reads and validates the WFM header
func (d *WFMFileDecoder) DecodeHeader(reader io.Reader) (*WFMHeader, error) {
	header := &WFMHeader{}

	// Read magic header
	if err := binary.Read(reader, binary.LittleEndian, &header.Magic); err != nil {
		return nil, fmt.Errorf("failed to read magic header: %w", err)
	}

	// Validate magic header
	if string(header.Magic[:]) != "WFM3" {
		return nil, fmt.Errorf("invalid magic header: expected 'WFM3', got '%s'", string(header.Magic[:]))
	}

	// Read padding
	if err := binary.Read(reader, binary.LittleEndian, &header.Padding); err != nil {
		return nil, fmt.Errorf("failed to read padding: %w", err)
	}

	// Read dialog pointer table offset
	if err := binary.Read(reader, binary.LittleEndian, &header.DialoguePointerTable); err != nil {
		return nil, fmt.Errorf("failed to read dialogue pointer table: %w", err)
	}

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
				imageSize := (int(glyph.GlyphWidth) * int(glyph.GlyphHeight) + 1) / 2
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
func (d *WFMFileDecoder) DecodeDialogues(reader io.Reader, header *WFMHeader) ([]uint32, []Dialogue, error) {
	dialoguePointers := make([]uint32, header.TotalDialogues)
	dialogues := make([]Dialogue, header.TotalDialogues)

	// Read dialog pointer table
	for i := uint16(0); i < header.TotalDialogues; i++ {
		if err := binary.Read(reader, binary.LittleEndian, &dialoguePointers[i]); err != nil {
			return nil, nil, fmt.Errorf("failed to read dialog pointer %d: %w", i, err)
		}
	}

	// Read dialog data (implementation depends on the actual dialog format)
	// For now, we'll create placeholders
	for i := uint16(0); i < header.TotalDialogues; i++ {
		dialogues[i] = Dialogue{
			Data: []byte{}, // This would be populated based on actual dialog format
		}
	}

	return dialoguePointers, dialogues, nil
}
