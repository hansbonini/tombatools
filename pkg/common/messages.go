package common

import (
	"fmt"
	"log"
)

// Global variable to control debug output
var VerboseMode bool = false

// SetVerboseMode enables or disables verbose/debug output
func SetVerboseMode(verbose bool) {
	VerboseMode = verbose
}

// Error messages
const (
	ErrFailedToLoadDialogues        = "failed to load dialogues"
	ErrFailedToReadYAMLFile         = "failed to read YAML file"
	ErrFailedToParseYAML            = "failed to parse YAML"
	ErrFailedToMapGlyphs            = "failed to map glyphs"
	ErrFailedToRecodeDialogues      = "failed to recode dialogue texts"
	ErrFailedToBuildWFM             = "failed to build WFM file"
	ErrFailedToWriteWFM             = "failed to write WFM file"
	ErrFailedToLoadPNG              = "failed to load PNG"
	ErrFailedToConvertTo4bpp        = "failed to convert to 4bpp"
	ErrFailedToCreateOutputFile     = "failed to create output file"
	ErrFailedToWriteHeader          = "failed to write header"
	ErrFailedToWriteGlyphPointer    = "failed to write glyph pointer"
	ErrFailedToWriteGlyphClut       = "failed to write glyph clut"
	ErrFailedToWriteGlyphHeight     = "failed to write glyph height"
	ErrFailedToWriteGlyphWidth      = "failed to write glyph width"
	ErrFailedToWriteGlyphHandakuten = "failed to write glyph handakuten"
	ErrFailedToWriteGlyphImage      = "failed to write glyph image"
	ErrFailedToWriteGlyphPadding    = "failed to write glyph padding"
	ErrFailedToWriteDialoguePointer = "failed to write dialogue pointer"
	ErrFailedToWriteDialogueData    = "failed to write dialogue data"
	ErrFailedToWriteDialoguePadding = "failed to write dialogue padding"
	ErrFailedToWritePadding         = "failed to write padding"
	ErrFailedToGetFilePosition      = "failed to get current file position"
	ErrGlyphFileNotFound            = "glyph file not found for character"
	ErrCharacterIgnored             = "character is ignored - no glyph needed"
	ErrCharacterIgnoredNoGlyph      = "character is ignored - no glyph loaded"
	ErrReservedDataSize             = "reservedData must be exactly 128 bytes"
)

// Info messages
const (
	InfoUniqueCharactersFound   = "Unique characters found"
	InfoTotalUniqueCharacters   = "Total unique characters"
	InfoUnmappedBytesFound      = "Unmapped bytes found"
	InfoTotalUnmappedBytes      = "Total unmapped bytes"
	InfoNoteUnmappedBytes       = "Note: These bytes need to be manually added to the font in the future"
	InfoGlyphMappingByHeight    = "Glyph mapping by font height"
	InfoEncodeValuesAssigned    = "Encode values assigned"
	InfoRecodedTexts            = "Recoded texts"
	InfoRecodingStatistics      = "Recoding statistics"
	InfoTotalDialoguesProcessed = "Total dialogues processed"
	InfoTotalEncodedBytes       = "Total encoded bytes"
	InfoWFMFileCreated          = "WFM file created successfully"
	InfoSpecialDialoguesFound   = "Special dialogues found"
	InfoReservedSectionBuilt    = "Reserved section built with special dialogue IDs"
	InfoReservedSectionUsed     = "Reserved section bytes used in header"
	InfoPaddingAdded            = "Added bytes of 0xFF padding to maintain original file size"
	InfoNoSpecialDialogues      = "No special dialogues found - Reserved section will be zero-filled"
	InfoGlyphLoaded             = "Loaded glyph for character at font height"

	// Exporter info messages
	InfoGlyphsExported           = "Successfully exported %d individual glyph PNG files to: %s"
	InfoDialoguesExported        = "Exported %d dialogues to YAML: %s"
	InfoSpecialDialoguesDetected = "Detected special dialogues from Reserved section: %v"
	InfoGlyphMappingBuilt        = "Built glyph mapping: %d glyphs mapped to characters"
	InfoNoSpecialDialoguesInFile = "All Reserved section bytes are zero - no special dialogues in file"
	InfoNoValidSpecialDialogues  = "No valid special dialogue IDs found in Reserved section"
)

// Debug messages
const (
	DebugCharacterFound   = "Char %d: '%c' (U+%04X)"
	DebugUnmappedByte     = "Unmapped %d: %s"
	DebugFontHeightGlyphs = "Font Height %d: %d glifos"
	DebugEncodeValue      = "0x%04X -> '%c' (U+%04X) at font height %d"
	DebugDialogueEncoded  = "Dialogue %d ('%s'):"
	DebugEncodedText      = "  Encoded: %s"
	DebugEncodedLength    = "  Length: %d bytes"
	DebugMoreDialogues    = "... e mais %d diálogos recodificados"
	DebugGlyphLoaded      = "%s '%c' (U+%04X) at font height %d"
	DebugHeaderInfo       = "Header: Magic=%s, Diálogos=%d, Glifos=%d"

	// Exporter debug messages
	DebugGlyphSkipped            = "Skipping glyph %d: invalid dimensions or empty image data"
	DebugGlyphExported           = "Exported glyph %d: %dx%d pixels (CLUT: %d, Handakuten: %d) -> %s"
	DebugDialogueMarkedSpecial   = "Marked dialogue %d as special"
	DebugReservedSectionBytes    = "Reserved section debug (first 32 bytes): "
	DebugDialogueZeroIncluded    = "First ID is 0 with non-zero values after - including dialogue 0 as special"
	DebugGlyphMapped             = "Mapped glyph %d to character '%s'"
	DebugHeaderPointerTable      = "Header DialoguePointerTable offset: %d (0x%X)"
	DebugReadingDialoguePointers = "Reading %d dialogue pointers starting from current position"
	DebugDialoguePointer         = "Dialogue pointer %d: %d (0x%X)"
	DebugReservedSectionHex      = "%02X "
)

// Warning messages
const (
	WarnCouldNotLoadGlyph       = "Could not load glyph for character"
	WarnNoEncodeMapping         = "No encode mapping found for character in dialogue"
	WarnSkippingUnmappedByte    = "Skipping unmapped byte in dialogue"
	WarnTooManySpecialDialogues = "Too many special dialogues, only first %d will be stored"
	WarnEncodedFileLarger       = "Encoded file (%d bytes) is larger than original (%d bytes)"

	// Exporter warning messages
	WarnCouldNotBuildGlyphMapping = "Could not build glyph mapping from font directory: %v"
	WarnDialoguesWithoutDecoding  = "Dialogues will be exported without text decoding"
	WarnInvalidDialogueID         = "Found invalid dialogue ID %d in Reserved section (max valid ID: %d)"
	WarnSeekToDialogue            = "Could not seek to dialogue %d at offset %d: %v"
)

// LogInfo logs an informational message
func LogInfo(message string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[INFO] "+message, args...)
	} else {
		log.Printf("[INFO] %s", message)
	}
}

// LogWarn logs a warning message
func LogWarn(message string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[WARN] "+message, args...)
	} else {
		log.Printf("[WARN] %s", message)
	}
}

// LogError logs an error message
func LogError(message string, args ...interface{}) {
	if len(args) > 0 {
		log.Printf("[ERROR] "+message, args...)
	} else {
		log.Printf("[ERROR] %s", message)
	}
}

// LogDebug logs a debug message (only if VerboseMode is enabled)
func LogDebug(message string, args ...interface{}) {
	if !VerboseMode {
		return
	}
	if len(args) > 0 {
		log.Printf("[DEBUG] "+message, args...)
	} else {
		log.Printf("[DEBUG] %s", message)
	}
}

// FormatError creates a formatted error with additional context
func FormatError(baseMessage string, details interface{}) error {
	if err, ok := details.(error); ok {
		return fmt.Errorf("%s: %w", baseMessage, err)
	}
	return fmt.Errorf("%s: %v", baseMessage, details)
}

// FormatErrorString creates a formatted error with string details
func FormatErrorString(baseMessage, details string, args ...interface{}) error {
	if len(args) > 0 {
		return fmt.Errorf("%s: "+details, append([]interface{}{baseMessage}, args...)...)
	}
	return fmt.Errorf("%s: %s", baseMessage, details)
}
