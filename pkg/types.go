package pkg

import "io"

// Default CLUT palettes for glyphs (converted from byte arrays to uint16)
var DialogueClut = [16]uint16{
	0x0000, 0x0400, 0x4E73, 0x2529,
	0x35AD, 0x4210, 0x14A5, 0x7E4D,
	0x03E0, 0x421F, 0x297F, 0x5319,
	0x4674, 0x3A11, 0x0000, 0x0000,
}

var EventClut = [16]uint16{
	0x01FF, 0x8400, 0x7FFF, 0x3DEF,
	0x2529, 0x56B5, 0x00F0, 0x0198,
	0x6739, 0x0134, 0x01FF, 0x7C00,
	0x7C00, 0x7C00, 0x7C00, 0x7C00,
}

// WFMHeader represents the main header of a WFM file
type WFMHeader struct {
	Magic                [4]byte // Always "WFM3"
	Padding              uint32
	DialoguePointerTable uint32
	TotalDialogues       uint16
	TotalGlyphs          uint16
	Reserved             [128]byte // Skip next 128 bytes
}

// Glyph represents the data for a single glyph
type Glyph struct {
	GlyphClut       uint16 // Color lookup table data
	GlyphHeight     uint16 // Height of the glyph
	GlyphWidth      uint16 // Width of the glyph
	GlyphHandakuten uint16 // Handakuten marker (Japanese diacritical mark)
	GlyphImage      []byte // Raw image data
}

// Dialogue represents a dialog entry in the WFM file
type Dialogue struct {
	Data []byte
}

// WFMFile represents the complete structure of a WFM file
type WFMFile struct {
	Header               WFMHeader
	GlyphPointerTable    []uint16
	Glyphs               []Glyph
	DialoguePointerTable []uint16
	Dialogues            []Dialogue
}

// WFMDecoder interface defines methods for decoding WFM files
type WFMDecoder interface {
	Decode(reader io.Reader) (*WFMFile, error)
	DecodeHeader(reader io.Reader) (*WFMHeader, error)
	DecodeGlyphs(reader io.Reader, header *WFMHeader) ([]uint16, []Glyph, error)
	DecodeDialogues(reader io.Reader, header *WFMHeader) ([]uint16, []Dialogue, error)
}

// WFMExporter interface defines methods for exporting WFM data
type WFMExporter interface {
	ExportToJSON(wfm *WFMFile, writer io.Writer) error
	ExportGlyphs(wfm *WFMFile, outputDir string) error
	ExportDialogues(wfm *WFMFile, outputDir string) error
}

// WFMProcessor combines decoder and exporter functionality
type WFMProcessor interface {
	WFMDecoder
	WFMExporter
	Process(inputFile string, outputDir string) error
}
