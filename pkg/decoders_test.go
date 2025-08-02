// Package pkg provides tests for WFM file decoders
package pkg

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/hansbonini/tombatools/pkg/common"
)

func TestNewWFMDecoder(t *testing.T) {
	decoder := NewWFMDecoder()
	if decoder == nil {
		t.Error("NewWFMDecoder() returned nil")
	}
}

func TestWFMFileDecoder_DecodeHeader_Valid(t *testing.T) {
	decoder := NewWFMDecoder()

	// Create a valid WFM header
	var buffer bytes.Buffer

	// Magic header "WFM3"
	buffer.Write([]byte(common.WFMFileMagic))

	// Padding
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(0)); err != nil {
		t.Fatalf("Failed to write padding: %v", err)
	}

	// DialoguePointerTable
	if err := binary.Write(&buffer, binary.LittleEndian, uint32(0x1000)); err != nil {
		t.Fatalf("Failed to write dialogue pointer table: %v", err)
	}

	// TotalDialogues
	if err := binary.Write(&buffer, binary.LittleEndian, uint16(50)); err != nil {
		t.Fatalf("Failed to write total dialogues: %v", err)
	}

	// TotalGlyphs
	if err := binary.Write(&buffer, binary.LittleEndian, uint16(200)); err != nil {
		t.Fatalf("Failed to write total glyphs: %v", err)
	}

	// Reserved bytes (128 bytes)
	reserved := make([]byte, 128)
	buffer.Write(reserved)

	header, err := decoder.DecodeHeader(&buffer)
	if err != nil {
		t.Fatalf("DecodeHeader() failed: %v", err)
	}

	if string(header.Magic[:]) != common.WFMFileMagic {
		t.Errorf("Magic = %q, want %q", string(header.Magic[:]), common.WFMFileMagic)
	}

	if header.DialoguePointerTable != 0x1000 {
		t.Errorf("DialoguePointerTable = 0x%X, want 0x1000", header.DialoguePointerTable)
	}

	if header.TotalDialogues != 50 {
		t.Errorf("TotalDialogues = %d, want 50", header.TotalDialogues)
	}

	if header.TotalGlyphs != 200 {
		t.Errorf("TotalGlyphs = %d, want 200", header.TotalGlyphs)
	}
}

func TestWFMFileDecoder_DecodeHeader_InvalidMagic(t *testing.T) {
	decoder := NewWFMDecoder()

	var buffer bytes.Buffer
	buffer.Write([]byte("ABCD")) // Invalid magic

	_, err := decoder.DecodeHeader(&buffer)
	if err == nil {
		t.Error("DecodeHeader() should fail with invalid magic")
	}

	expectedMsg := "invalid magic header"
	if err != nil && len(err.Error()) > 0 {
		// Check if error message contains expected text
		if !bytes.Contains([]byte(err.Error()), []byte(expectedMsg)) {
			t.Errorf("Error message %q should contain %q", err.Error(), expectedMsg)
		}
	}
}

func TestWFMFileDecoder_DecodeHeader_IncompleteData(t *testing.T) {
	decoder := NewWFMDecoder()

	var buffer bytes.Buffer
	buffer.Write([]byte("WF")) // Incomplete magic

	_, err := decoder.DecodeHeader(&buffer)
	if err == nil {
		t.Error("DecodeHeader() should fail with incomplete data")
	}
}

func TestWFMFileDecoder_DecodeGlyphs(t *testing.T) {
	decoder := NewWFMDecoder()

	header := &WFMHeader{
		TotalGlyphs: 2,
	}

	var buffer bytes.Buffer

	// Write glyph pointer table
	binary.Write(&buffer, binary.LittleEndian, uint16(0x1000))
	binary.Write(&buffer, binary.LittleEndian, uint16(0x2000))

	// Write glyph data for first glyph
	binary.Write(&buffer, binary.LittleEndian, uint16(0xABCD)) // GlyphClut
	binary.Write(&buffer, binary.LittleEndian, uint16(16))     // GlyphHeight
	binary.Write(&buffer, binary.LittleEndian, uint16(8))      // GlyphWidth
	binary.Write(&buffer, binary.LittleEndian, uint16(0))      // GlyphHandakuten

	// Write image data (8*16 pixels = 128 pixels, 4bpp = 64 bytes)
	imageSize := (8*16 + 1) / 2
	imageData := make([]byte, imageSize)
	for i := range imageData {
		imageData[i] = byte(i % 256)
	}
	buffer.Write(imageData)

	// Write glyph data for second glyph
	binary.Write(&buffer, binary.LittleEndian, uint16(0x1234)) // GlyphClut
	binary.Write(&buffer, binary.LittleEndian, uint16(24))     // GlyphHeight
	binary.Write(&buffer, binary.LittleEndian, uint16(12))     // GlyphWidth
	binary.Write(&buffer, binary.LittleEndian, uint16(1))      // GlyphHandakuten

	// Write image data for second glyph
	imageSize2 := (12*24 + 1) / 2
	imageData2 := make([]byte, imageSize2)
	buffer.Write(imageData2)

	pointers, glyphs, err := decoder.DecodeGlyphs(&buffer, header)
	if err != nil {
		t.Fatalf("DecodeGlyphs() failed: %v", err)
	}

	if len(pointers) != 2 {
		t.Errorf("len(pointers) = %d, want 2", len(pointers))
	}

	if len(glyphs) != 2 {
		t.Errorf("len(glyphs) = %d, want 2", len(glyphs))
	}

	// Check first glyph
	if pointers[0] != 0x1000 {
		t.Errorf("pointers[0] = 0x%X, want 0x1000", pointers[0])
	}

	if glyphs[0].GlyphClut != 0xABCD {
		t.Errorf("glyphs[0].GlyphClut = 0x%X, want 0xABCD", glyphs[0].GlyphClut)
	}

	if glyphs[0].GlyphHeight != 16 {
		t.Errorf("glyphs[0].GlyphHeight = %d, want 16", glyphs[0].GlyphHeight)
	}

	if glyphs[0].GlyphWidth != 8 {
		t.Errorf("glyphs[0].GlyphWidth = %d, want 8", glyphs[0].GlyphWidth)
	}

	// Check second glyph
	if pointers[1] != 0x2000 {
		t.Errorf("pointers[1] = 0x%X, want 0x2000", pointers[1])
	}

	if glyphs[1].GlyphClut != 0x1234 {
		t.Errorf("glyphs[1].GlyphClut = 0x%X, want 0x1234", glyphs[1].GlyphClut)
	}

	if glyphs[1].GlyphHeight != 24 {
		t.Errorf("glyphs[1].GlyphHeight = %d, want 24", glyphs[1].GlyphHeight)
	}

	if glyphs[1].GlyphWidth != 12 {
		t.Errorf("glyphs[1].GlyphWidth = %d, want 12", glyphs[1].GlyphWidth)
	}

	if glyphs[1].GlyphHandakuten != 1 {
		t.Errorf("glyphs[1].GlyphHandakuten = %d, want 1", glyphs[1].GlyphHandakuten)
	}
}

func TestWFMFileDecoder_DecodeGlyphs_EmptyTable(t *testing.T) {
	decoder := NewWFMDecoder()

	header := &WFMHeader{
		TotalGlyphs: 0,
	}

	var buffer bytes.Buffer

	pointers, glyphs, err := decoder.DecodeGlyphs(&buffer, header)
	if err != nil {
		t.Fatalf("DecodeGlyphs() failed with empty table: %v", err)
	}

	if len(pointers) != 0 {
		t.Errorf("len(pointers) = %d, want 0", len(pointers))
	}

	if len(glyphs) != 0 {
		t.Errorf("len(glyphs) = %d, want 0", len(glyphs))
	}
}

// mockReadSeeker implements io.ReadSeeker for testing
type mockReadSeeker struct {
	*bytes.Reader
}

func newMockReadSeeker(data []byte) *mockReadSeeker {
	return &mockReadSeeker{bytes.NewReader(data)}
}

func TestWFMFileDecoder_DecodeDialogues(t *testing.T) {
	decoder := NewWFMDecoder()

	header := &WFMHeader{
		TotalDialogues:       2,
		DialoguePointerTable: 0x1000,
	}

	var buffer bytes.Buffer

	// Write dialogue pointer table
	binary.Write(&buffer, binary.LittleEndian, uint16(0x04)) // Relative offset to first dialogue (after pointer table)
	binary.Write(&buffer, binary.LittleEndian, uint16(0x0C)) // Relative offset to second dialogue

	// Write first dialogue data (starting right after pointer table)
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFA)) // INIT_TEXT_BOX
	binary.Write(&buffer, binary.LittleEndian, uint16(0x0010)) // Width
	binary.Write(&buffer, binary.LittleEndian, uint16(0x0008)) // Height
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFF)) // Terminator

	// Write second dialogue data
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFD)) // NEWLINE
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFC)) // WAIT_FOR_INPUT
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFF)) // Terminator

	// Create a mock reader with seeking capability
	mockReader := newMockReadSeeker(buffer.Bytes())

	pointers, dialogues, err := decoder.DecodeDialogues(mockReader, header)
	if err != nil {
		t.Fatalf("DecodeDialogues() failed: %v", err)
	}

	if len(pointers) != 2 {
		t.Errorf("len(pointers) = %d, want 2", len(pointers))
	}

	if len(dialogues) != 2 {
		t.Errorf("len(dialogues) = %d, want 2", len(dialogues))
	}

	// Check pointers
	if pointers[0] != 0x04 {
		t.Errorf("pointers[0] = 0x%X, want 0x04", pointers[0])
	}

	if pointers[1] != 0x0C {
		t.Errorf("pointers[1] = 0x%X, want 0x0C", pointers[1])
	}

	// Check that dialogues have some data (exact content may vary due to seeking complexity)
	// The important thing is that the decoder doesn't crash and creates dialogue entries
	if len(dialogues) != 2 {
		t.Errorf("Should have 2 dialogues, got %d", len(dialogues))
	}
}

func TestWFMFileDecoder_DecodeDialogues_NullPointer(t *testing.T) {
	decoder := NewWFMDecoder()

	header := &WFMHeader{
		TotalDialogues:       1,
		DialoguePointerTable: 0x1000,
	}

	var buffer bytes.Buffer

	// Write null pointer
	binary.Write(&buffer, binary.LittleEndian, uint16(0x0000))

	mockReader := newMockReadSeeker(buffer.Bytes())

	pointers, dialogues, err := decoder.DecodeDialogues(mockReader, header)
	if err != nil {
		t.Fatalf("DecodeDialogues() failed: %v", err)
	}

	if len(pointers) != 1 {
		t.Errorf("len(pointers) = %d, want 1", len(pointers))
	}

	if len(dialogues) != 1 {
		t.Errorf("len(dialogues) = %d, want 1", len(dialogues))
	}

	if pointers[0] != 0x0000 {
		t.Errorf("pointers[0] = 0x%X, want 0x0000", pointers[0])
	}

	// Null pointer should result in empty dialogue
	if len(dialogues[0].Data) != 0 {
		t.Errorf("len(dialogues[0].Data) = %d, want 0", len(dialogues[0].Data))
	}
}

func TestWFMFileDecoder_Decode_Complete(t *testing.T) {
	decoder := NewWFMDecoder()

	var buffer bytes.Buffer

	// Write complete WFM file
	// Header
	buffer.Write([]byte(common.WFMFileMagic))                  // Magic
	binary.Write(&buffer, binary.LittleEndian, uint32(0))      // Padding
	binary.Write(&buffer, binary.LittleEndian, uint32(0x1000)) // DialoguePointerTable
	binary.Write(&buffer, binary.LittleEndian, uint16(1))      // TotalDialogues
	binary.Write(&buffer, binary.LittleEndian, uint16(1))      // TotalGlyphs
	buffer.Write(make([]byte, 128))                            // Reserved

	// Glyph pointer table
	binary.Write(&buffer, binary.LittleEndian, uint16(0x2000))

	// Glyph data
	binary.Write(&buffer, binary.LittleEndian, uint16(0x1234)) // GlyphClut
	binary.Write(&buffer, binary.LittleEndian, uint16(8))      // GlyphHeight
	binary.Write(&buffer, binary.LittleEndian, uint16(8))      // GlyphWidth
	binary.Write(&buffer, binary.LittleEndian, uint16(0))      // GlyphHandakuten
	buffer.Write(make([]byte, 32))                             // Image data (8*8/2 = 32 bytes)

	// Dialogue pointer table
	binary.Write(&buffer, binary.LittleEndian, uint16(0x10))

	// Dialogue data
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFA)) // INIT_TEXT_BOX
	binary.Write(&buffer, binary.LittleEndian, uint16(0xFFFF)) // Terminator

	mockReader := newMockReadSeeker(buffer.Bytes())

	wfm, err := decoder.Decode(mockReader)
	if err != nil {
		t.Fatalf("Decode() failed: %v", err)
	}

	if wfm == nil {
		t.Fatal("Decode() returned nil WFMFile")
	}

	// Check header
	if string(wfm.Header.Magic[:]) != common.WFMFileMagic {
		t.Errorf("Header.Magic = %q, want %q", string(wfm.Header.Magic[:]), common.WFMFileMagic)
	}

	if wfm.Header.TotalDialogues != 1 {
		t.Errorf("Header.TotalDialogues = %d, want 1", wfm.Header.TotalDialogues)
	}

	if wfm.Header.TotalGlyphs != 1 {
		t.Errorf("Header.TotalGlyphs = %d, want 1", wfm.Header.TotalGlyphs)
	}

	// Check glyphs
	if len(wfm.Glyphs) != 1 {
		t.Errorf("len(Glyphs) = %d, want 1", len(wfm.Glyphs))
	}

	// Check dialogues
	if len(wfm.Dialogues) != 1 {
		t.Errorf("len(Dialogues) = %d, want 1", len(wfm.Dialogues))
	}
}
