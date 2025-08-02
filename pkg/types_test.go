// Package pkg provides tests for types and data structures
package pkg

import (
	"testing"
)

func TestDialogueContentItem_Interface(t *testing.T) {
	// Test that all content types implement the interface
	var _ DialogueContentItem = BoxContent{}
	var _ DialogueContentItem = TailContent{}
	var _ DialogueContentItem = F6Content{}
	var _ DialogueContentItem = ColorContent{}
	var _ DialogueContentItem = PauseContent{}
	var _ DialogueContentItem = TextContent{}
}

func TestBoxContent(t *testing.T) {
	box := BoxContent{Width: 100, Height: 50}

	if box.Width != 100 {
		t.Errorf("BoxContent.Width = %d, want 100", box.Width)
	}

	if box.Height != 50 {
		t.Errorf("BoxContent.Height = %d, want 50", box.Height)
	}

	// Test interface implementation
	box.isDialogueContentItem()
}

func TestTailContent(t *testing.T) {
	tail := TailContent{Width: 80, Height: 40}

	if tail.Width != 80 {
		t.Errorf("TailContent.Width = %d, want 80", tail.Width)
	}

	if tail.Height != 40 {
		t.Errorf("TailContent.Height = %d, want 40", tail.Height)
	}

	// Test interface implementation
	tail.isDialogueContentItem()
}

func TestF6Content(t *testing.T) {
	f6 := F6Content{Width: 120, Height: 60}

	if f6.Width != 120 {
		t.Errorf("F6Content.Width = %d, want 120", f6.Width)
	}

	if f6.Height != 60 {
		t.Errorf("F6Content.Height = %d, want 60", f6.Height)
	}

	// Test interface implementation
	f6.isDialogueContentItem()
}

func TestColorContent(t *testing.T) {
	color := ColorContent{Value: 5}

	if color.Value != 5 {
		t.Errorf("ColorContent.Value = %d, want 5", color.Value)
	}

	// Test interface implementation
	color.isDialogueContentItem()
}

func TestPauseContent(t *testing.T) {
	pause := PauseContent{Duration: 1000}

	if pause.Duration != 1000 {
		t.Errorf("PauseContent.Duration = %d, want 1000", pause.Duration)
	}

	// Test interface implementation
	pause.isDialogueContentItem()
}

func TestTextContent(t *testing.T) {
	text := TextContent{Text: "Hello World"}

	if text.Text != "Hello World" {
		t.Errorf("TextContent.Text = %q, want %q", text.Text, "Hello World")
	}

	// Test interface implementation
	text.isDialogueContentItem()
}

func TestDialogueEntry(t *testing.T) {
	dialogue := DialogueEntry{
		ID:         1,
		Type:       "dialogue",
		FontHeight: 16,
		FontClut:   0x1234,
		Terminator: 0xFFFE,
		Special:    false,
		Content:    []map[string]interface{}{},
	}

	if dialogue.ID != 1 {
		t.Errorf("DialogueEntry.ID = %d, want 1", dialogue.ID)
	}

	if dialogue.Type != "dialogue" {
		t.Errorf("DialogueEntry.Type = %q, want %q", dialogue.Type, "dialogue")
	}

	if dialogue.FontHeight != 16 {
		t.Errorf("DialogueEntry.FontHeight = %d, want 16", dialogue.FontHeight)
	}

	if dialogue.FontClut != 0x1234 {
		t.Errorf("DialogueEntry.FontClut = 0x%04X, want 0x1234", dialogue.FontClut)
	}

	if dialogue.Terminator != 0xFFFE {
		t.Errorf("DialogueEntry.Terminator = 0x%04X, want 0xFFFE", dialogue.Terminator)
	}

	if dialogue.Special != false {
		t.Errorf("DialogueEntry.Special = %t, want false", dialogue.Special)
	}

	if len(dialogue.Content) != 0 {
		t.Errorf("len(DialogueEntry.Content) = %d, want 0", len(dialogue.Content))
	}
}

func TestWFMHeader(t *testing.T) {
	header := WFMHeader{
		Magic:                [4]byte{'W', 'F', 'M', '3'},
		Padding:              0,
		DialoguePointerTable: 0x1000,
		TotalDialogues:       50,
		TotalGlyphs:          200,
	}

	expectedMagic := [4]byte{'W', 'F', 'M', '3'}
	if header.Magic != expectedMagic {
		t.Errorf("WFMHeader.Magic = %v, want %v", header.Magic, expectedMagic)
	}

	if header.Padding != 0 {
		t.Errorf("WFMHeader.Padding = %d, want 0", header.Padding)
	}

	if header.DialoguePointerTable != 0x1000 {
		t.Errorf("WFMHeader.DialoguePointerTable = 0x%X, want 0x1000", header.DialoguePointerTable)
	}

	if header.TotalDialogues != 50 {
		t.Errorf("WFMHeader.TotalDialogues = %d, want 50", header.TotalDialogues)
	}

	if header.TotalGlyphs != 200 {
		t.Errorf("WFMHeader.TotalGlyphs = %d, want 200", header.TotalGlyphs)
	}
}

func TestGlyph(t *testing.T) {
	imageData := []byte{0x01, 0x23, 0x45, 0x67}
	glyph := Glyph{
		GlyphClut:       0xABCD,
		GlyphHeight:     16,
		GlyphWidth:      8,
		GlyphHandakuten: 0,
		GlyphImage:      imageData,
	}

	if glyph.GlyphClut != 0xABCD {
		t.Errorf("Glyph.GlyphClut = 0x%04X, want 0xABCD", glyph.GlyphClut)
	}

	if glyph.GlyphHeight != 16 {
		t.Errorf("Glyph.GlyphHeight = %d, want 16", glyph.GlyphHeight)
	}

	if glyph.GlyphWidth != 8 {
		t.Errorf("Glyph.GlyphWidth = %d, want 8", glyph.GlyphWidth)
	}

	if glyph.GlyphHandakuten != 0 {
		t.Errorf("Glyph.GlyphHandakuten = %d, want 0", glyph.GlyphHandakuten)
	}

	if len(glyph.GlyphImage) != 4 {
		t.Errorf("len(Glyph.GlyphImage) = %d, want 4", len(glyph.GlyphImage))
	}

	// Check image data content
	for i, expected := range imageData {
		if glyph.GlyphImage[i] != expected {
			t.Errorf("Glyph.GlyphImage[%d] = 0x%02X, want 0x%02X", i, glyph.GlyphImage[i], expected)
		}
	}
}

func TestDialogue(t *testing.T) {
	dialogueData := []byte{0xFF, 0xFA, 0x00, 0x10, 0x00, 0x08}
	dialogue := Dialogue{Data: dialogueData}

	if len(dialogue.Data) != 6 {
		t.Errorf("len(Dialogue.Data) = %d, want 6", len(dialogue.Data))
	}

	// Check dialogue data content
	for i, expected := range dialogueData {
		if dialogue.Data[i] != expected {
			t.Errorf("Dialogue.Data[%d] = 0x%02X, want 0x%02X", i, dialogue.Data[i], expected)
		}
	}
}

func TestWFMFile(t *testing.T) {
	header := WFMHeader{
		Magic:          [4]byte{'W', 'F', 'M', '3'},
		TotalDialogues: 2,
		TotalGlyphs:    3,
	}

	glyphPointers := []uint16{0x1000, 0x2000, 0x3000}
	glyphs := []Glyph{
		{GlyphWidth: 8, GlyphHeight: 16},
		{GlyphWidth: 12, GlyphHeight: 16},
		{GlyphWidth: 10, GlyphHeight: 16},
	}

	dialoguePointers := []uint16{0x4000, 0x5000}
	dialogues := []Dialogue{
		{Data: []byte{0xFF, 0xFA}},
		{Data: []byte{0xFF, 0xFB}},
	}

	wfm := WFMFile{
		Header:               header,
		GlyphPointerTable:    glyphPointers,
		Glyphs:               glyphs,
		DialoguePointerTable: dialoguePointers,
		Dialogues:            dialogues,
		OriginalSize:         1024,
	}

	if len(wfm.GlyphPointerTable) != 3 {
		t.Errorf("len(WFMFile.GlyphPointerTable) = %d, want 3", len(wfm.GlyphPointerTable))
	}

	if wfm.Header.TotalDialogues != 2 {
		t.Errorf("WFMFile.Header.TotalDialogues = %d, want 2", wfm.Header.TotalDialogues)
	}

	if wfm.Header.TotalGlyphs != 3 {
		t.Errorf("WFMFile.Header.TotalGlyphs = %d, want 3", wfm.Header.TotalGlyphs)
	}

	if len(wfm.Glyphs) != 3 {
		t.Errorf("len(WFMFile.Glyphs) = %d, want 3", len(wfm.Glyphs))
	}

	if len(wfm.DialoguePointerTable) != 2 {
		t.Errorf("len(WFMFile.DialoguePointerTable) = %d, want 2", len(wfm.DialoguePointerTable))
	}

	if len(wfm.Dialogues) != 2 {
		t.Errorf("len(WFMFile.Dialogues) = %d, want 2", len(wfm.Dialogues))
	}

	if wfm.OriginalSize != 1024 {
		t.Errorf("WFMFile.OriginalSize = %d, want 1024", wfm.OriginalSize)
	}
}

func TestConstants(t *testing.T) {
	// Test control code constants
	testCases := []struct {
		name     string
		constant uint16
		expected uint16
	}{
		{"INIT_TEXT_BOX", INIT_TEXT_BOX, 0xFFFA},
		{"FFF2", FFF2, 0xFFF2},
		{"HALT", HALT, 0xFFF3},
		{"F4", F4, 0xFFF4},
		{"PROMPT", PROMPT, 0xFFF5},
		{"F6", F6, 0xFFF6},
		{"CHANGE_COLOR_TO", CHANGE_COLOR_TO, 0xFFF7},
		{"INIT_TAIL", INIT_TAIL, 0xFFF8},
		{"PAUSE_FOR", PAUSE_FOR, 0xFFF9},
		{"DOUBLE_NEWLINE", DOUBLE_NEWLINE, 0xFFFB},
		{"WAIT_FOR_INPUT", WAIT_FOR_INPUT, 0xFFFC},
		{"NEWLINE", NEWLINE, 0xFFFD},
		{"C04D", C04D, 0xC04D},
		{"C04E", C04E, 0xC04E},
		{"TERMINATOR_1", TERMINATOR_1, 0xFFFE},
		{"TERMINATOR_2", TERMINATOR_2, 0xFFFF},
		{"GLYPH_ID_BASE", GLYPH_ID_BASE, 0x8000},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.constant != tc.expected {
				t.Errorf("%s = 0x%04X, want 0x%04X", tc.name, tc.constant, tc.expected)
			}
		})
	}
}

func TestDefaultPalettes(t *testing.T) {
	// Test DialogueClut palette
	if len(DialogueClut) != 16 {
		t.Errorf("len(DialogueClut) = %d, want 16", len(DialogueClut))
	}

	// Test first color (transparent)
	if DialogueClut[0] != 0x0000 {
		t.Errorf("DialogueClut[0] = 0x%04X, want 0x0000", DialogueClut[0])
	}

	// Test EventClut palette
	if len(EventClut) != 16 {
		t.Errorf("len(EventClut) = %d, want 16", len(EventClut))
	}

	// Test first color
	if EventClut[0] != 0x01FF {
		t.Errorf("EventClut[0] = 0x%04X, want 0x01FF", EventClut[0])
	}
}
