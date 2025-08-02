package pkg

import "io"

// Special control codes constants
const (
	// Text box initialization
	INIT_TEXT_BOX = 0xFFFA

	// Special control codes
	FFF2            = 0xFFF2 // args: 1
	HALT            = 0xFFF3
	F4              = 0xFFF4
	PROMPT          = 0xFFF5
	F6              = 0xFFF6 // args: 2
	CHANGE_COLOR_TO = 0xFFF7 // args: 1
	INIT_TAIL       = 0xFFF8 // args: 2
	PAUSE_FOR       = 0xFFF9 // args: 1
	DOUBLE_NEWLINE  = 0xFFFB
	WAIT_FOR_INPUT  = 0xFFFC
	NEWLINE         = 0xFFFD

	// Additional special commands
	C04D = 0xC04D // Special character
	C04E = 0xC04E // Special character

	// Termination markers
	TERMINATOR_1 = 0xFFFE
	TERMINATOR_2 = 0xFFFF

	// Glyph ID base offset
	GLYPH_ID_BASE = 0x8000
)

// Default CLUT (Color Look-Up Table) palettes for glyph rendering
// Each palette contains 16 colors in PlayStation PSX 15-bit format
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

// New dialogue content structures
type DialogueContentItem interface {
	isDialogueContentItem()
}

type BoxContent struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

func (b BoxContent) isDialogueContentItem() {}

type TailContent struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

func (t TailContent) isDialogueContentItem() {}

type F6Content struct {
	Width  int `yaml:"width"`
	Height int `yaml:"height"`
}

func (f F6Content) isDialogueContentItem() {}

type ColorContent struct {
	Value int `yaml:"value"`
}

func (c ColorContent) isDialogueContentItem() {}

type PauseContent struct {
	Duration int `yaml:"duration"`
}

func (p PauseContent) isDialogueContentItem() {}

type TextContent struct {
	Text string `yaml:",inline"`
}

func (t TextContent) isDialogueContentItem() {}

// DialogueEntry represents a single dialogue with the new structure
type DialogueEntry struct {
	ID         int                      `yaml:"id"`
	Type       string                   `yaml:"type"`
	FontHeight int                      `yaml:"font_height"`
	FontClut   uint16                   `yaml:"font_clut"`
	Terminator uint16                   `yaml:"terminator"`
	Special    bool                     `yaml:"special,omitempty"`
	Content    []map[string]interface{} `yaml:"content"`
}

// WFMHeader represents the main header of a WFM file structure
type WFMHeader struct {
	Magic                [4]byte // Always "WFM3"
	Padding              uint32
	DialoguePointerTable uint32
	TotalDialogues       uint16
	TotalGlyphs          uint16
	Reserved             [128]byte // Reserved section (may contain special dialogue IDs)
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
	OriginalSize         int64 // Size of the original WFM file in bytes
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

// WFMEncoder interface defines methods for encoding WFM files from extracted data
type WFMEncoder interface {
	Encode(yamlFile string, outputFile string) error
	LoadDialogues(yamlFile string) ([]DialogueEntry, error)
	LoadGlyphs(glyphsDir string, fontHeight int) ([]Glyph, error)
	BuildWFMFile(dialogues []DialogueEntry, glyphs []Glyph) (*WFMFile, error)
	WriteWFMFile(wfm *WFMFile, outputFile string) error
}

// WFMProcessor combines decoder and exporter functionality
type WFMProcessor interface {
	WFMDecoder
	WFMExporter
	Process(inputFile string, outputDir string) error
}
