// Package pkg provides functionality for processing WFM font files from the Tomba! PlayStation game.
// This file contains encoders for converting YAML dialogue files back to WFM format.
package pkg

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/hansbonini/tombatools/pkg/psx"
	"gopkg.in/yaml.v3"
)

// WFMFileEncoder implements the WFMEncoder interface and provides
// functionality to encode YAML dialogue data back into WFM file format.
type WFMFileEncoder struct {
	originalSize int64 // Store original file size for proper padding
}

// GlyphEncodeInfo holds information about a glyph and its assigned encode value.
// This structure is used during the encoding process to map characters to glyph IDs.
type GlyphEncodeInfo struct {
	Character  rune
	FontHeight int
	Glyph      Glyph
}

// RecodedDialogue represents a dialogue with recoded text for WFM encoding.
// This structure contains both the original text and the encoded glyph sequence.
type RecodedDialogue struct {
	ID           int      // Dialogue identifier
	Type         string   // Type of dialogue (event, dialogue, etc.)
	FontHeight   uint16   // Font height used for this dialogue
	OriginalText string   // Original text content
	EncodedText  []uint16 // Encoded glyph IDs representing the text
}

// Encode creates a WFM file from a YAML dialogue file and associated glyph directory.
// This is the main entry point for converting YAML dialogue data back to WFM format.
// Parameters:
//   - yamlFile: Path to the YAML file containing dialogue data
//   - outputFile: Path where the encoded WFM file will be written
//
// Returns an error if the encoding process fails.
func (e *WFMFileEncoder) Encode(yamlFile, outputFile string) error {
	// Load dialogues from YAML file
	dialogues, reservedData, err := e.LoadDialogues(yamlFile)
	if err != nil {
		return common.FormatError(common.ErrFailedToLoadDialogues, err)
	}

	// Process characters and build mappings
	glyphEncodeMap, encodeValueMap, encodeOrder, err := e.processCharactersAndBuildMappings(dialogues)
	if err != nil {
		return err
	}

	// Recode dialogues and build WFM file
	wfmFile, err := e.recodeAndBuildWFM(dialogues, glyphEncodeMap, encodeValueMap, encodeOrder, reservedData)
	if err != nil {
		return err
	}

	// Write the WFM file
	if err := e.writeWFMFile(wfmFile, outputFile); err != nil {
		return common.FormatError(common.ErrFailedToWriteWFM, err)
	}

	e.logFinalResults(outputFile, wfmFile)
	return nil
}

// processCharactersAndBuildMappings handles character analysis and glyph mapping
func (e *WFMFileEncoder) processCharactersAndBuildMappings(dialogues []DialogueEntry) (glyphEncodeMap map[int]map[rune]uint16, glyphInfoMap map[uint16]GlyphEncodeInfo, glyphPointers []uint16, err error) {
	// Step 1: Collect all unique characters used in dialogue text attributes
	uniqueChars, unmappedBytes := e.collectUniqueCharacters(dialogues)
	e.logCharacterAnalysis(uniqueChars, unmappedBytes)

	// Step 2: Map glyphs by dialogue considering font_height
	glyphMap, err := e.mapGlyphsByDialogue(dialogues)
	if err != nil {
		return nil, nil, nil, common.FormatError(common.ErrFailedToMapGlyphs, err)
	}

	// Step 3: Assign encode values for each mapped glyph
	glyphEncodeMap, encodeValueMap, encodeOrder := e.assignEncodeValues(glyphMap)
	e.logGlyphMapping(glyphMap, encodeValueMap, encodeOrder)

	return glyphEncodeMap, encodeValueMap, encodeOrder, nil
}

// recodeAndBuildWFM handles dialogue recoding and WFM file building
func (e *WFMFileEncoder) recodeAndBuildWFM(dialogues []DialogueEntry, glyphEncodeMap map[int]map[rune]uint16, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16, reservedData []byte) (*WFMFile, error) {
	// Step 4: Re-encode dialogue texts using the mapping
	recodedDialogues, err := e.recodeDialogueTexts(dialogues, glyphEncodeMap)
	if err != nil {
		return nil, common.FormatError(common.ErrFailedToRecodeDialogues, err)
	}

	e.logRecodingResults(recodedDialogues)

	// Step 5: Build the final WFM file
	wfmFile, err := e.buildWFMFile(make(map[int]map[rune]Glyph), encodeValueMap, encodeOrder, recodedDialogues, reservedData)
	if err != nil {
		return nil, common.FormatError(common.ErrFailedToBuildWFM, err)
	}

	return wfmFile, nil
}

// logCharacterAnalysis logs character analysis results
func (e *WFMFileEncoder) logCharacterAnalysis(uniqueChars []rune, unmappedBytes []string) {
	common.LogInfo("%s:", common.InfoUniqueCharactersFound)
	common.LogInfo("%s: %d", common.InfoTotalUniqueCharacters, len(uniqueChars))

	// Display characters in sorted order
	for i, char := range uniqueChars {
		common.LogDebug(common.DebugCharacterFound, i, char, char)
	}

	// Display unmapped bytes found
	if len(unmappedBytes) > 0 {
		common.LogInfo("\n%s:", common.InfoUnmappedBytesFound)
		common.LogInfo("%s: %d", common.InfoTotalUnmappedBytes, len(unmappedBytes))
		for i, unmappedByte := range unmappedBytes {
			common.LogDebug(common.DebugUnmappedByte, i, unmappedByte)
		}
		common.LogInfo("\n%s", common.InfoNoteUnmappedBytes)
	}
}

// logGlyphMapping logs glyph mapping results
func (e *WFMFileEncoder) logGlyphMapping(glyphMap map[int]map[rune]Glyph, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16) {
	common.LogInfo("\n%s:", common.InfoGlyphMappingByHeight)
	for fontHeight, glyphs := range glyphMap {
		common.LogDebug(common.DebugFontHeightGlyphs, fontHeight, len(glyphs))
	}

	encodeMapSize, err := common.SafeIntToUint16(len(encodeValueMap))
	if err != nil {
		common.LogWarn("Encode value map size exceeds uint16 range: %v", err)
		encodeMapSize = 65535
	}
	common.LogInfo("\n%s (0x8000-0x%04X) na ordem de adição:", common.InfoEncodeValuesAssigned, 0x8000+encodeMapSize-1)

	// Display in the order they were added
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		common.LogDebug(common.DebugEncodeValue, encodeValue, glyphInfo.Character, glyphInfo.Character, glyphInfo.FontHeight)
	}
}

// logRecodingResults logs dialogue recoding results
func (e *WFMFileEncoder) logRecodingResults(recodedDialogues []RecodedDialogue) {
	common.LogInfo("\n%s:", common.InfoRecodedTexts)
	for i, dialogue := range recodedDialogues {
		if i < 5 { // Show only the first 5 with more detail
			common.LogDebug(common.DebugDialogueEncoded, dialogue.ID, dialogue.OriginalText)
			common.LogDebug(common.DebugEncodedText, e.formatEncodedText(dialogue.EncodedText))
			common.LogDebug(common.DebugEncodedLength, len(dialogue.EncodedText)*2) // each uint16 = 2 bytes
		}
	}
	if len(recodedDialogues) > 5 {
		common.LogDebug(common.DebugMoreDialogues, len(recodedDialogues)-5)
	}

	common.LogInfo("\n%s:", common.InfoRecodingStatistics)
	common.LogInfo("%s: %d", common.InfoTotalDialoguesProcessed, len(recodedDialogues))

	totalEncodedBytes := 0
	for _, dialogue := range recodedDialogues {
		totalEncodedBytes += len(dialogue.EncodedText) * 2 // each uint16 = 2 bytes
	}
	common.LogInfo("%s: %d", common.InfoTotalEncodedBytes, totalEncodedBytes)
}

// logFinalResults logs final encoding results
func (e *WFMFileEncoder) logFinalResults(outputFile string, wfmFile *WFMFile) {
	common.LogInfo("\n%s: %s", common.InfoWFMFileCreated, outputFile)
	common.LogDebug(common.DebugHeaderInfo,
		string(wfmFile.Header.Magic[:]), wfmFile.Header.TotalDialogues, wfmFile.Header.TotalGlyphs)
}

// LoadDialogues loads dialogue entries from YAML file
func (e *WFMFileEncoder) LoadDialogues(yamlFile string) ([]DialogueEntry, []byte, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, nil, common.FormatError(common.ErrFailedToReadYAMLFile, err)
	}

	var yamlData struct {
		TotalDialogues int             `yaml:"total_dialogues"`
		OriginalSize   int64           `yaml:"original_size"`
		Dialogues      []DialogueEntry `yaml:"dialogues"`
	}

	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, nil, common.FormatError(common.ErrFailedToParseYAML, err)
	}

	// Build reserved data based on special dialogues
	reservedData := e.buildReservedData(yamlData.Dialogues)

	// Store original size for later use in padding
	e.originalSize = yamlData.OriginalSize

	return yamlData.Dialogues, reservedData, nil
}

// buildReservedData constructs the 128-byte Reserved section based on special dialogues
func (e *WFMFileEncoder) buildReservedData(dialogues []DialogueEntry) []byte {
	// Create 128-byte reserved section - ALWAYS 128 bytes
	reservedData := make([]byte, 128)

	// Find all dialogues marked as special
	var specialDialogueIDs []int
	for _, dialogue := range dialogues {
		if dialogue.Special {
			specialDialogueIDs = append(specialDialogueIDs, dialogue.ID)
		}
	}

	// If no special dialogues found, return zero-filled array
	if len(specialDialogueIDs) == 0 {
		common.LogInfo("%s (128 bytes)", common.InfoNoSpecialDialogues)
		return reservedData
	}

	// Sort special dialogue IDs to ensure consistent order
	sort.Ints(specialDialogueIDs)

	// Pack special dialogue IDs into the reserved section
	// Each ID is stored as uint16 (2 bytes) in little endian format
	byteIndex := 0
	maxEntries := 128 / 2 // Maximum 64 entries (128 bytes / 2 bytes per ID)

	for i, id := range specialDialogueIDs {
		if i >= maxEntries {
			common.LogWarn(common.WarnTooManySpecialDialogues, len(specialDialogueIDs), maxEntries)
			break
		}

		if byteIndex+1 < len(reservedData) {
			// Store ID as uint16 little endian
			reservedData[byteIndex] = byte(id & 0xFF)          // Low byte
			reservedData[byteIndex+1] = byte((id >> 8) & 0xFF) // High byte
			byteIndex += 2
		}
	}

	common.LogInfo("%s: %v", common.InfoSpecialDialoguesFound, specialDialogueIDs)
	common.LogInfo("%s %d special dialogue IDs (128 bytes total)", common.InfoReservedSectionBuilt, len(specialDialogueIDs))

	// Ensure we always return exactly 128 bytes
	if len(reservedData) != 128 {
		panic(fmt.Sprintf("Reserved section must be exactly 128 bytes, got %d", len(reservedData)))
	}

	return reservedData
}

// collectUniqueCharacters collects all unique characters from dialogue content and returns unmapped bytes
func (e *WFMFileEncoder) collectUniqueCharacters(dialogues []DialogueEntry) (uniqueChars []rune, unmappedBytes []string) {
	charSet := make(map[rune]bool)
	unmappedSet := make(map[string]bool)

	// Regex to identify unmapped bytes (format [XXXX] with 4 uppercase hex digits)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	// List of known special tags that should be removed
	specialTags := []string{
		"[FFF2]", "[HALT]", "[F4]", "[PROMPT]", "[F6]", "[CHANGE COLOR TO]",
		"[INIT TAIL]", "[PAUSE FOR]", "[WAIT FOR INPUT]", "[INIT TEXT BOX]",
	}

	for _, dialogue := range dialogues {
		// Process content items to extract text
		for _, contentItem := range dialogue.Content {
			if textValue, exists := contentItem["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					// Convert special commands to unicode before processing
					textStr = strings.ReplaceAll(textStr, "[C04D]", "▼")
					textStr = strings.ReplaceAll(textStr, "[C04E]", "⏷")

					originalText := textStr

					// First, collect unmapped bytes before removing them
					unmappedMatches := unmappedByteRegex.FindAllString(originalText, -1)
					for _, match := range unmappedMatches {
						unmappedSet[match] = true
					}

					cleanText := originalText

					// Remove tags especiais conhecidas
					for _, tag := range specialTags {
						cleanText = strings.ReplaceAll(cleanText, tag, "")
					}

					// Remove unmapped bytes like [8030], [8031], etc. (format %04X)
					cleanText = unmappedByteRegex.ReplaceAllString(cleanText, "")

					// Remove line breaks that may come from tags
					cleanText = strings.ReplaceAll(cleanText, "\n", "")

					// Now count only the actual characters that need mapping
					for _, char := range cleanText {
						charSet[char] = true
					}
				}
			}
		}
	}

	// Convert char map to slice
	uniqueChars = make([]rune, 0, len(charSet))
	for char := range charSet {
		uniqueChars = append(uniqueChars, char)
	}

	// Sort for consistent output
	sort.Slice(uniqueChars, func(i, j int) bool {
		return uniqueChars[i] < uniqueChars[j]
	})

	// Convert unmapped map to slice
	unmappedBytes = make([]string, 0, len(unmappedSet))
	for unmapped := range unmappedSet {
		unmappedBytes = append(unmappedBytes, unmapped)
	}

	// Sort unmapped bytes for consistent output
	sort.Strings(unmappedBytes)

	return uniqueChars, unmappedBytes
}

// mapGlyphsByDialogue maps glyphs by dialogue considering font_height with global caching
func (e *WFMFileEncoder) mapGlyphsByDialogue(dialogues []DialogueEntry) (map[int]map[rune]Glyph, error) {
	// Global dictionary to avoid remapping: [fontHeight][char] = glyph
	globalGlyphCache := make(map[int]map[rune]Glyph)

	for _, dialogue := range dialogues {
		if err := e.processDialogueForGlyphMapping(dialogue, globalGlyphCache); err != nil {
			return nil, err
		}
	}

	return globalGlyphCache, nil
}

// processDialogueForGlyphMapping processes a single dialogue for glyph mapping
func (e *WFMFileEncoder) processDialogueForGlyphMapping(dialogue DialogueEntry, globalGlyphCache map[int]map[rune]Glyph) error {
	fontHeight := dialogue.FontHeight
	fontClut := dialogue.FontClut

	// Initialize the map for this font height if it doesn't exist
	if globalGlyphCache[fontHeight] == nil {
		globalGlyphCache[fontHeight] = make(map[rune]Glyph)
	}

	// Process content items to extract text
	for _, contentItem := range dialogue.Content {
		if textValue, exists := contentItem["text"]; exists {
			if textStr, ok := textValue.(string); ok {
				if err := e.processTextForGlyphMapping(textStr, fontHeight, fontClut, globalGlyphCache); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// processTextForGlyphMapping processes text content for glyph mapping
func (e *WFMFileEncoder) processTextForGlyphMapping(textStr string, fontHeight int, fontClut uint16, globalGlyphCache map[int]map[rune]Glyph) error {
	// Clean the dialogue text
	cleanText := e.cleanTextForGlyphMapping(textStr)

	// Process each character
	for _, char := range cleanText {
		// Check if the character has already been mapped for this font height
		if _, exists := globalGlyphCache[fontHeight][char]; !exists {
			if err := e.tryLoadGlyph(char, fontHeight, fontClut, globalGlyphCache); err != nil {
				return err
			}
		}
	}

	return nil
}

// cleanTextForGlyphMapping cleans text by removing special tags and unmapped bytes
func (e *WFMFileEncoder) cleanTextForGlyphMapping(textStr string) string {
	// Regex to identify unmapped bytes (format [XXXX] with 4 uppercase hex digits)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	// List of known special tags that should be removed
	specialTags := []string{
		"[FFF2]", "[HALT]", "[F4]", "[PROMPT]", "[F6]", "[CHANGE COLOR TO]",
		"[INIT TAIL]", "[PAUSE FOR]", "[WAIT FOR INPUT]", "[INIT TEXT BOX]",
	}

	cleanText := textStr

	// Remove known special tags
	for _, tag := range specialTags {
		cleanText = strings.ReplaceAll(cleanText, tag, "")
	}

	// Remove unmapped bytes
	cleanText = unmappedByteRegex.ReplaceAllString(cleanText, "")

	// Remove line breaks
	cleanText = strings.ReplaceAll(cleanText, "\n", "")

	return cleanText
}

// tryLoadGlyph attempts to load a glyph and store it in the cache
func (e *WFMFileEncoder) tryLoadGlyph(char rune, fontHeight int, fontClut uint16, globalGlyphCache map[int]map[rune]Glyph) error {
	// Try to load the glyph
	glyph, err := e.loadSingleGlyph(char, fontHeight, fontClut)
	if err != nil {
		// Check if this is an ignored character
		if char == '⧗' {
			// Silently skip ignored characters
			return nil
		}
		common.LogWarn("%s '%c' (U+%04X) at font height %d: %v", common.WarnCouldNotLoadGlyph, char, char, fontHeight, err)
		return nil
	}

	// Store in global cache
	globalGlyphCache[fontHeight][char] = glyph
	common.LogDebug(common.DebugGlyphLoaded, common.InfoGlyphLoaded, char, char, fontHeight)
	return nil
}

// assignEncodeValues assigns sequential encode values starting from 0x8000 to each mapped glyph
// Each combination of character + font height gets a unique encode value
func (e *WFMFileEncoder) assignEncodeValues(glyphMap map[int]map[rune]Glyph) (glyphEncodeMap map[int]map[rune]uint16, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16) {
	// Map to store encode value for each glyph: [fontHeight][char] = encodeValue
	glyphEncodeMap = make(map[int]map[rune]uint16)

	// Reverse map for lookup: [encodeValue] = GlyphEncodeInfo
	encodeValueMap = make(map[uint16]GlyphEncodeInfo)

	// Calculate total number of glyphs for pre-allocation
	totalGlyphs := 0
	for _, glyphs := range glyphMap {
		totalGlyphs += len(glyphs)
	}

	// List to maintain order of encode value additions
	encodeOrder = make([]uint16, 0, totalGlyphs)

	// Counter for sequential values starting at 0x8000
	currentEncodeValue := uint16(0x8000)

	// Create a list of all combinations (fontHeight, char) for consistent ordering
	type glyphKey struct {
		fontHeight int
		char       rune
	}

	var allGlyphKeys []glyphKey
	for fontHeight, glyphs := range glyphMap {
		for char := range glyphs {
			allGlyphKeys = append(allGlyphKeys, glyphKey{fontHeight: fontHeight, char: char})
		}
	}

	// Sort by font height first, then by character
	// This ensures that glyphs of the same height are grouped, but each char+height is unique
	sort.Slice(allGlyphKeys, func(i, j int) bool {
		if allGlyphKeys[i].fontHeight != allGlyphKeys[j].fontHeight {
			return allGlyphKeys[i].fontHeight < allGlyphKeys[j].fontHeight
		}
		return allGlyphKeys[i].char < allGlyphKeys[j].char
	})

	// Assign sequential values for each unique char + fontHeight combination
	for _, key := range allGlyphKeys {
		fontHeight := key.fontHeight
		char := key.char
		glyph := glyphMap[fontHeight][char]

		// Initialize the map for this font height if it doesn't exist
		if glyphEncodeMap[fontHeight] == nil {
			glyphEncodeMap[fontHeight] = make(map[rune]uint16)
		}

		// Assign the encode value (each char+height is treated as a unique glyph)
		glyphEncodeMap[fontHeight][char] = currentEncodeValue

		// Store information in the reverse map
		encodeValueMap[currentEncodeValue] = GlyphEncodeInfo{
			Character:  char,
			FontHeight: fontHeight,
			Glyph:      glyph,
		}

		// Add to order list
		encodeOrder = append(encodeOrder, currentEncodeValue)

		// Increment for next value
		currentEncodeValue++
	}

	return glyphEncodeMap, encodeValueMap, encodeOrder
}

// recodeDialogueTexts recodes dialogue content using the glyph encode mapping and handles content structure
func (e *WFMFileEncoder) recodeDialogueTexts(dialogues []DialogueEntry, glyphEncodeMap map[int]map[rune]uint16) ([]RecodedDialogue, error) {
	recodedDialogues := make([]RecodedDialogue, 0, len(dialogues))

	for _, dialogue := range dialogues {
		recodedDialogue, err := e.recodeDialogue(dialogue, glyphEncodeMap)
		if err != nil {
			return nil, err
		}
		recodedDialogues = append(recodedDialogues, recodedDialogue)
	}

	return recodedDialogues, nil
}

// recodeDialogue recodes a single dialogue entry
func (e *WFMFileEncoder) recodeDialogue(dialogue DialogueEntry, glyphEncodeMap map[int]map[rune]uint16) (RecodedDialogue, error) {
	fontHeight := dialogue.FontHeight

	// Check if we have mapping for this font height
	// Note: Allow empty mapping when dialogue only contains special codes
	if glyphEncodeMap[fontHeight] == nil {
		// Initialize empty mapping if it doesn't exist
		glyphEncodeMap[fontHeight] = make(map[rune]uint16)
	}

	var encodedText []uint16
	var fullOriginalText strings.Builder

	// Process content items sequentially
	for _, contentItem := range dialogue.Content {
		contentEncoded, originalText, err := e.processContentItem(contentItem, fontHeight, glyphEncodeMap, dialogue.ID)
		if err != nil {
			return RecodedDialogue{}, err
		}
		encodedText = append(encodedText, contentEncoded...)
		fullOriginalText.WriteString(originalText)
	}

	// Add termination marker
	terminatorHex := e.getTerminatorHex(dialogue.Terminator)
	encodedText = append(encodedText, terminatorHex)

	safeFontHeight, err := common.SafeIntToUint16(dialogue.FontHeight)
	if err != nil {
		return RecodedDialogue{}, fmt.Errorf("invalid font height %d: %w", dialogue.FontHeight, err)
	}

	recodedDialogue := RecodedDialogue{
		ID:           dialogue.ID,
		Type:         dialogue.Type,
		FontHeight:   safeFontHeight,
		OriginalText: fullOriginalText.String(),
		EncodedText:  encodedText,
	}

	return recodedDialogue, nil
}

// processContentItem processes a single content item and returns encoded text and original text
func (e *WFMFileEncoder) processContentItem(contentItem map[string]interface{}, fontHeight int, glyphEncodeMap map[int]map[rune]uint16, dialogueID int) (encodedText []uint16, originalText string, err error) {
	// Handle box content
	if boxValue, exists := contentItem["box"]; exists {
		encodedText, originalText, err = e.processBoxContent(boxValue)
		return
	}

	// Handle tail content
	if tailValue, exists := contentItem["tail"]; exists {
		encodedText, originalText, err = e.processTailContent(tailValue)
		return
	}

	// Handle f6 content
	if f6Value, exists := contentItem["f6"]; exists {
		encodedText, originalText, err = e.processF6Content(f6Value)
		return
	}

	// Handle color content
	if colorValue, exists := contentItem["color"]; exists {
		encodedText, originalText, err = e.processColorContent(colorValue)
		return
	}

	// Handle pause content
	if pauseValue, exists := contentItem["pause"]; exists {
		encodedText, originalText, err = e.processPauseContent(pauseValue)
		return
	}

	// Handle fff2 content
	if fff2Value, exists := contentItem["fff2"]; exists {
		encodedText, originalText, err = e.processFff2Content(fff2Value)
		return
	}

	// Handle text content
	if textValue, exists := contentItem["text"]; exists {
		encodedText, originalText, err = e.processTextContent(textValue, fontHeight, glyphEncodeMap, dialogueID)
		return
	}

	return nil, "", nil
}

// processBoxContent handles box content items
func (e *WFMFileEncoder) processBoxContent(boxValue interface{}) (encodedText []uint16, originalText string, err error) {
	boxMap, ok := boxValue.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, INIT_TEXT_BOX)

	if width, hasWidth := boxMap["width"]; hasWidth {
		if w, ok := width.(int); ok {
			safeWidth, err := common.SafeIntToUint16(w)
			if err != nil {
				return nil, "", fmt.Errorf("invalid width value %d: %w", w, err)
			}
			encodedText = append(encodedText, safeWidth)
		}
	}

	if height, hasHeight := boxMap["height"]; hasHeight {
		if h, ok := height.(int); ok {
			safeHeight, err := common.SafeIntToUint16(h)
			if err != nil {
				return nil, "", fmt.Errorf("invalid height value %d: %w", h, err)
			}
			encodedText = append(encodedText, safeHeight)
		}
	}

	return encodedText, "", nil
}

// processTailContent handles tail content items
func (e *WFMFileEncoder) processTailContent(tailValue interface{}) (encodedText []uint16, originalText string, err error) {
	tailMap, ok := tailValue.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, INIT_TAIL)

	if width, hasWidth := tailMap["width"]; hasWidth {
		if w, ok := width.(int); ok {
			safeWidth, err := common.SafeIntToUint16(w)
			if err != nil {
				return nil, "", fmt.Errorf("invalid tail width value %d: %w", w, err)
			}
			encodedText = append(encodedText, safeWidth)
		}
	}

	if height, hasHeight := tailMap["height"]; hasHeight {
		if h, ok := height.(int); ok {
			safeHeight, err := common.SafeIntToUint16(h)
			if err != nil {
				return nil, "", fmt.Errorf("invalid tail height value %d: %w", h, err)
			}
			encodedText = append(encodedText, safeHeight)
		}
	}

	return encodedText, "", nil
}

// processF6Content handles f6 content items
func (e *WFMFileEncoder) processF6Content(f6Value interface{}) (encodedText []uint16, originalText string, err error) {
	f6Map, ok := f6Value.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, F6)

	if width, hasWidth := f6Map["width"]; hasWidth {
		if w, ok := width.(int); ok {
			safeWidth, err := common.SafeIntToUint16(w)
			if err != nil {
				return nil, "", fmt.Errorf("invalid f6 width value %d: %w", w, err)
			}
			encodedText = append(encodedText, safeWidth)
		}
	}

	if height, hasHeight := f6Map["height"]; hasHeight {
		if h, ok := height.(int); ok {
			safeHeight, err := common.SafeIntToUint16(h)
			if err != nil {
				return nil, "", fmt.Errorf("invalid f6 height value %d: %w", h, err)
			}
			encodedText = append(encodedText, safeHeight)
		}
	}

	return encodedText, "", nil
}

// processColorContent handles color content items
func (e *WFMFileEncoder) processColorContent(colorValue interface{}) (encodedText []uint16, originalText string, err error) {
	colorMap, ok := colorValue.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, CHANGE_COLOR_TO)

	if value, hasValue := colorMap["value"]; hasValue {
		if v, ok := value.(int); ok {
			safeValue, err := common.SafeIntToUint16(v)
			if err != nil {
				return nil, "", fmt.Errorf("invalid color value %d: %w", v, err)
			}
			encodedText = append(encodedText, safeValue)
		}
	}

	return encodedText, "", nil
}

// processPauseContent handles pause content items
func (e *WFMFileEncoder) processPauseContent(pauseValue interface{}) (encodedText []uint16, originalText string, err error) {
	pauseMap, ok := pauseValue.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, PAUSE_FOR)

	if duration, hasDuration := pauseMap["duration"]; hasDuration {
		if d, ok := duration.(int); ok {
			safeDuration, err := common.SafeIntToUint16(d)
			if err != nil {
				return nil, "", fmt.Errorf("invalid pause duration value %d: %w", d, err)
			}
			encodedText = append(encodedText, safeDuration)
		}
	}

	return encodedText, "", nil
}

// processFff2Content handles fff2 content items
func (e *WFMFileEncoder) processFff2Content(fff2Value interface{}) (encodedText []uint16, originalText string, err error) {
	fff2Map, ok := fff2Value.(map[string]interface{})
	if !ok {
		return nil, "", nil
	}

	encodedText = append(encodedText, FFF2)

	if value, hasValue := fff2Map["value"]; hasValue {
		if v, ok := value.(int); ok {
			safeValue, err := common.SafeIntToUint16(v)
			if err != nil {
				return nil, "", fmt.Errorf("invalid fff2 value %d: %w", v, err)
			}
			encodedText = append(encodedText, safeValue)
		}
	}

	return encodedText, "", nil
}

// processTextContent handles text content items
func (e *WFMFileEncoder) processTextContent(textValue interface{}, fontHeight int, glyphEncodeMap map[int]map[rune]uint16, dialogueID int) (encodedText []uint16, originalText string, err error) {
	textStr, ok := textValue.(string)
	if !ok {
		return nil, "", nil
	}

	// Process text character by character and tag by tag
	runes := []rune(textStr)
	i := 0

	for i < len(runes) {
		processed, codes, advance, err := e.processTextRune(runes, i, fontHeight, glyphEncodeMap, dialogueID)
		if err != nil {
			return nil, "", err
		}
		if processed {
			encodedText = append(encodedText, codes...)
			i += advance
		} else {
			i++
		}
	}

	return encodedText, textStr, nil
}

// processTextRune processes a single rune or tag in text content
func (e *WFMFileEncoder) processTextRune(runes []rune, i, fontHeight int, glyphEncodeMap map[int]map[rune]uint16, dialogueID int) (isProcessed bool, encodedPart []uint16, nextIndex int, err error) {
	if i >= len(runes) {
		return false, nil, 0, nil
	}

	// Check if it's a special tag
	if runes[i] == '[' {
		return e.handleSpecialTag(runes, i, dialogueID)
	}

	// Handle special unicode characters
	return e.handleUnicodeCharacter(runes, i, fontHeight, glyphEncodeMap, dialogueID)
}

// handleSpecialTag processes special tags like [FFF2], [HALT], etc.
func (e *WFMFileEncoder) handleSpecialTag(runes []rune, i, dialogueID int) (isTag bool, encodedPart []uint16, nextIndex int, err error) {
	specialTagMap := map[string]uint16{
		"[FFF2]":            FFF2,
		"[HALT]":            HALT,
		"[F4]":              F4,
		"[PROMPT]":          PROMPT,
		"[F6]":              F6,
		"[CHANGE COLOR TO]": CHANGE_COLOR_TO,
		"[INIT TAIL]":       INIT_TAIL,
		"[PAUSE FOR]":       PAUSE_FOR,
		"[C04D]":            C04D,
		"[C04E]":            C04E,
		"[WAIT FOR INPUT]":  WAIT_FOR_INPUT,
		"[INIT TEXT BOX]":   INIT_TEXT_BOX,
	}

	// Check known special tags
	for tag, code := range specialTagMap {
		if found, advance := e.matchesTag(runes, i, tag); found {
			return true, []uint16{code}, advance, nil
		}
	}

	// Check if it's an unmapped byte [XXXX]
	return e.handleUnmappedByte(runes, i, dialogueID)
}

// matchesTag checks if the runes at position i match the given tag
func (e *WFMFileEncoder) matchesTag(runes []rune, i int, tag string) (matches bool, nextIndex int) {
	tagRunes := []rune(tag)
	if i+len(tagRunes) > len(runes) {
		return false, 0
	}

	for j, tagRune := range tagRunes {
		if runes[i+j] != tagRune {
			return false, 0
		}
	}
	return true, len(tagRunes)
}

// handleUnmappedByte processes unmapped byte sequences like [XXXX]
func (e *WFMFileEncoder) handleUnmappedByte(runes []rune, i, dialogueID int) (isUnmapped bool, encodedPart []uint16, nextIndex int, err error) {
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	remainingText := string(runes[i:])
	if len(remainingText) >= 6 {
		possibleUnmapped := remainingText[:6]
		if unmappedByteRegex.MatchString(possibleUnmapped) {
			// Skip unmapped bytes (don't include in encode)
			common.LogWarn("%s %s in dialogue %d", common.WarnSkippingUnmappedByte, possibleUnmapped, dialogueID)
			return true, nil, 6, nil
		}
	}

	return false, nil, 0, nil
}

// handleUnicodeCharacter processes regular unicode characters and special symbols
func (e *WFMFileEncoder) handleUnicodeCharacter(runes []rune, i, fontHeight int, glyphEncodeMap map[int]map[rune]uint16, dialogueID int) (isProcessed bool, encodedPart []uint16, nextIndex int, err error) {
	char := runes[i]

	// Handle special unicode symbols
	if code, found := e.getSpecialUnicodeCode(char); found {
		return true, []uint16{code}, 1, nil
	}

	// Handle newlines
	if char == '\n' {
		return e.handleNewline(runes, i)
	}

	// Check if we have mapping for this character
	return e.handleMappedCharacter(char, fontHeight, glyphEncodeMap, dialogueID)
}

// getSpecialUnicodeCode returns the code for special unicode characters
func (e *WFMFileEncoder) getSpecialUnicodeCode(char rune) (uint16, bool) {
	switch char {
	case '▼':
		return C04D, true
	case '⏷':
		return C04E, true
	case '⧗':
		return WAIT_FOR_INPUT, true
	default:
		return 0, false
	}
}

// handleNewline processes newline characters (single or double)
func (e *WFMFileEncoder) handleNewline(runes []rune, i int) (isNewline bool, encodedPart []uint16, nextIndex int, err error) {
	// Check if this is a double newline (\n\n)
	if i+1 < len(runes) && runes[i+1] == '\n' {
		return true, []uint16{DOUBLE_NEWLINE}, 2, nil
	}
	return true, []uint16{NEWLINE}, 1, nil
}

// handleMappedCharacter processes characters that should be mapped to glyphs
func (e *WFMFileEncoder) handleMappedCharacter(char rune, fontHeight int, glyphEncodeMap map[int]map[rune]uint16, dialogueID int) (isMapped bool, encodedPart []uint16, nextIndex int, err error) {
	if encodeValue, exists := glyphEncodeMap[fontHeight][char]; exists {
		return true, []uint16{encodeValue}, 1, nil
	}

	common.LogWarn("%s '%c' (U+%04X) in dialogue %d", common.WarnNoEncodeMapping, char, char, dialogueID)
	return false, nil, 0, nil
}

// getTerminatorHex converts terminator value to hex
func (e *WFMFileEncoder) getTerminatorHex(terminator uint16) uint16 {
	switch terminator {
	case 1:
		return 0xFFFE // TERMINATOR_1
	case 2:
		return 0xFFFF // TERMINATOR_2
	default:
		return 0xFFFF // Default to TERMINATOR_2
	}
}

// formatEncodedText formats encoded text as a readable hex string
func (e *WFMFileEncoder) formatEncodedText(encodedText []uint16) string {
	if len(encodedText) == 0 {
		return "(empty)"
	}

	var result strings.Builder
	for i, value := range encodedText {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(fmt.Sprintf("0x%04X", value))

		// Limit number of displayed values to avoid polluting output
		if i >= 9 {
			if len(encodedText) > 10 {
				result.WriteString(fmt.Sprintf(" ... (+%d more)", len(encodedText)-10))
			}
			break
		}
	}

	return result.String()
}

// alignToBytes ensures a value is aligned to the specified byte boundary
func alignToBytes(value, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return ((value + alignment - 1) / alignment) * alignment
}

// alignToBytes16 ensures a value is aligned to the specified byte boundary for uint16
func alignToBytes16(value, alignment uint16) uint16 {
	if alignment == 0 {
		return value
	}
	return ((value + alignment - 1) / alignment) * alignment
}

// buildWFMFile constructs a complete WFM file from the processed data
func (e *WFMFileEncoder) buildWFMFile(glyphMap map[int]map[rune]Glyph, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16, recodedDialogues []RecodedDialogue, reservedData []byte) (*WFMFile, error) {
	// Create ordered list of glyphs and dialogues
	glyphs := e.buildGlyphList(encodeValueMap, encodeOrder)
	dialogues, err := e.buildDialogueList(recodedDialogues)
	if err != nil {
		return nil, err
	}

	// Calculate pointer tables
	glyphPointerTable, err := e.calculateGlyphPointers(glyphs)
	if err != nil {
		return nil, err
	}

	dialoguePointerTable, err := e.calculateDialoguePointers(dialogues)
	if err != nil {
		return nil, err
	}

	// Calculate dialogue pointer table offset
	dialoguePointerTableOffset, err := e.calculateDialoguePointerTableOffset(glyphs)
	if err != nil {
		return nil, err
	}

	// Create header
	header, err := e.buildHeader(dialogues, glyphs, dialoguePointerTableOffset, reservedData)
	if err != nil {
		return nil, err
	}

	// Create complete WFM file
	wfmFile := &WFMFile{
		Header:               header,
		GlyphPointerTable:    glyphPointerTable,
		Glyphs:               glyphs,
		DialoguePointerTable: dialoguePointerTable,
		Dialogues:            dialogues,
	}

	return wfmFile, nil
}

// buildGlyphList creates ordered list of glyphs using encodeOrder
func (e *WFMFileEncoder) buildGlyphList(encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16) []Glyph {
	glyphs := make([]Glyph, 0, len(encodeOrder))
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		glyphs = append(glyphs, glyphInfo.Glyph)
	}
	return glyphs
}

// buildDialogueList converts recoded dialogues to WFM format
func (e *WFMFileEncoder) buildDialogueList(recodedDialogues []RecodedDialogue) ([]Dialogue, error) {
	// First, sort dialogues by ID to ensure correct sequence
	sort.Slice(recodedDialogues, func(i, j int) bool {
		return recodedDialogues[i].ID < recodedDialogues[j].ID
	})

	dialogues := make([]Dialogue, 0, len(recodedDialogues))
	for _, recodedDialogue := range recodedDialogues {
		// Convert uint16 values to bytes (little endian)
		var dialogueData []byte
		for _, value := range recodedDialogue.EncodedText {
			// Add value in little endian (2 bytes)
			dialogueData = append(dialogueData, byte(value&0xFF), byte((value>>8)&0xFF)) // little endian
		}

		dialogue := Dialogue{
			Data: dialogueData,
		}
		dialogues = append(dialogues, dialogue)
	}

	return dialogues, nil
}

// calculateGlyphPointers calculates glyph pointers relative to WFM file start
func (e *WFMFileEncoder) calculateGlyphPointers(glyphs []Glyph) ([]uint16, error) {
	glyphPointerTable := make([]uint16, 0, len(glyphs))
	headerSize := uint32(4 + 4 + 4 + 2 + 2 + 128) // Magic + Padding + DialoguePointerTable + TotalDialogues + TotalGlyphs + Reserved

	// Safe conversion: len(glyphs) * 2 should not overflow uint32 in reasonable use cases
	if len(glyphs) > (1<<31-1)/2 {
		return nil, fmt.Errorf("too many glyphs: %d", len(glyphs))
	}
	glyphTableSize, err := common.SafeIntToUint32(len(glyphs) * 2)
	if err != nil {
		return nil, fmt.Errorf("glyph table size calculation failed: %w", err)
	}
	currentGlyphOffset := headerSize + glyphTableSize // Start of glyph data

	for _, glyph := range glyphs {
		// Ensure glyph offset fits in uint16
		if currentGlyphOffset > 65535 {
			return nil, fmt.Errorf("glyph offset too large: %d", currentGlyphOffset)
		}
		glyphPointerTable = append(glyphPointerTable, uint16(currentGlyphOffset)) // Safe: checked above

		// Each glyph has: 2+2+2+2 = 8 bytes of attributes + image size
		glyphSize := 8 + len(glyph.GlyphImage)
		// Safe conversion: glyphSize should not cause overflow in reasonable use cases
		if glyphSize > (1<<31-1) || len(glyph.GlyphImage) > (1<<31-1)-8 {
			return nil, fmt.Errorf("glyph image too large: %d bytes", len(glyph.GlyphImage))
		}
		safeGlyphSize, err := common.SafeIntToUint32(glyphSize)
		if err != nil {
			return nil, fmt.Errorf("glyph size conversion failed: %w", err)
		}
		currentGlyphOffset += safeGlyphSize
	}

	return glyphPointerTable, nil
}

// calculateDialoguePointers calculates dialogue pointers relative to start of dialogue pointer table
func (e *WFMFileEncoder) calculateDialoguePointers(dialogues []Dialogue) ([]uint16, error) {
	dialoguePointerTable := make([]uint16, 0, len(dialogues))
	// Safe conversion: ensure len(dialogues) * 2 fits in uint16
	if len(dialogues) > 32767 {
		return nil, fmt.Errorf("too many dialogues: %d", len(dialogues))
	}
	safeDialogueOffset, err := common.SafeIntToUint16(len(dialogues) * 2)
	if err != nil {
		return nil, fmt.Errorf("dialogue offset calculation failed: %w", err)
	}
	currentDialogueOffset := safeDialogueOffset // Size of pointer table, safe: checked above

	// Ensure dialogue data is byte-aligned (2-byte alignment)
	currentDialogueOffset = alignToBytes16(currentDialogueOffset, 2)

	for _, dialogue := range dialogues {
		dialoguePointerTable = append(dialoguePointerTable, currentDialogueOffset)
		// Safe conversion: ensure dialogue data size fits in uint16
		if len(dialogue.Data) > 65535 {
			return nil, fmt.Errorf("dialogue data too large: %d bytes", len(dialogue.Data))
		}
		safeDialogueSize, err := common.SafeIntToUint16(len(dialogue.Data))
		if err != nil {
			return nil, fmt.Errorf("dialogue size conversion failed: %w", err)
		}
		dialogueSize := safeDialogueSize
		// Ensure each dialogue is byte-aligned
		alignedDialogueSize := alignToBytes16(dialogueSize, 2)
		currentDialogueOffset += alignedDialogueSize
	}

	return dialoguePointerTable, nil
}

// calculateDialoguePointerTableOffset calculates position of dialogue pointer table
func (e *WFMFileEncoder) calculateDialoguePointerTableOffset(glyphs []Glyph) (uint32, error) {
	headerSize := uint32(4 + 4 + 4 + 2 + 2 + 128) // Magic + Padding + DialoguePointerTable + TotalDialogues + TotalGlyphs + Reserved
	safeGlyphTableSize, err := common.SafeIntToUint32(len(glyphs) * 2)
	if err != nil {
		return 0, fmt.Errorf("glyph table size calculation failed: %w", err)
	}
	glyphTableSize := safeGlyphTableSize // Size of glyph pointer table

	totalGlyphsSize := uint32(0)
	for _, glyph := range glyphs {
		// Safe conversion: ensure glyph image size doesn't cause overflow
		if len(glyph.GlyphImage) > (1<<31-1)-8 {
			return 0, fmt.Errorf("glyph image too large: %d bytes", len(glyph.GlyphImage))
		}
		safeGlyphSize, err := common.SafeIntToUint32(8 + len(glyph.GlyphImage))
		if err != nil {
			return 0, fmt.Errorf("glyph size calculation failed: %w", err)
		}
		glyphSize := safeGlyphSize
		// Ensure each glyph is byte-aligned
		alignedGlyphSize := alignToBytes(glyphSize, 2)
		totalGlyphsSize += alignedGlyphSize
	}

	dialoguePointerTableOffset := headerSize + glyphTableSize + totalGlyphsSize
	// Ensure dialogue pointer table is byte-aligned
	dialoguePointerTableOffset = alignToBytes(dialoguePointerTableOffset, 2)

	return dialoguePointerTableOffset, nil
}

// buildHeader creates the WFM header
func (e *WFMFileEncoder) buildHeader(dialogues []Dialogue, glyphs []Glyph, dialoguePointerTableOffset uint32, reservedData []byte) (WFMHeader, error) {
	var reservedBytes [128]byte
	if len(reservedData) > 0 {
		// Ensure reservedData is exactly 128 bytes
		if len(reservedData) != 128 {
			return WFMHeader{}, common.FormatErrorString(common.ErrReservedDataSize, "got %d", len(reservedData))
		}
		// Use reserved data from special dialogues
		copy(reservedBytes[:], reservedData)
	}
	// If reservedData is empty or nil, reservedBytes remains zero-filled

	common.LogInfo("%s: %d bytes", common.InfoReservedSectionUsed, len(reservedBytes))

	safeTotalDialogues, err := common.SafeIntToUint16(len(dialogues))
	if err != nil {
		return WFMHeader{}, fmt.Errorf("total dialogues conversion failed: %w", err)
	}
	safeTotalGlyphs, err := common.SafeIntToUint16(len(glyphs))
	if err != nil {
		return WFMHeader{}, fmt.Errorf("total glyphs conversion failed: %w", err)
	}

	header := WFMHeader{
		Magic:                [4]byte{'W', 'F', 'M', '3'},
		Padding:              0,
		DialoguePointerTable: dialoguePointerTableOffset,
		TotalDialogues:       safeTotalDialogues,
		TotalGlyphs:          safeTotalGlyphs,
		Reserved:             reservedBytes,
	}

	return header, nil
}

// writeWFMFile writes the WFM file to disk
func (e *WFMFileEncoder) writeWFMFile(wfm *WFMFile, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return common.FormatError(common.ErrFailedToCreateOutputFile, err)
	}
	defer file.Close()

	// Write header
	if err := e.writeHeader(file, &wfm.Header); err != nil {
		return err
	}

	// Write glyph pointer table
	if err := e.writeGlyphPointerTable(file, wfm.GlyphPointerTable); err != nil {
		return err
	}

	// Write glyphs
	if err := e.writeGlyphs(file, wfm.Glyphs); err != nil {
		return err
	}

	// Ensure alignment before dialogue pointer table
	if err := e.ensureDialogueAlignment(file); err != nil {
		return err
	}

	// Write dialogue pointer table
	if err := e.writeDialoguePointerTable(file, wfm.DialoguePointerTable); err != nil {
		return err
	}

	// Write dialogues
	if err := e.writeDialogues(file, wfm.Dialogues); err != nil {
		return err
	}

	// Apply final padding if necessary
	if err := e.applyFinalPadding(file); err != nil {
		return err
	}

	return nil
}

// writeHeader writes the WFM header to file
func (e *WFMFileEncoder) writeHeader(file *os.File, header *WFMHeader) error {
	err := binary.Write(file, binary.LittleEndian, header)
	if err != nil {
		return common.FormatError(common.ErrFailedToWriteHeader, err)
	}
	return nil
}

// writeGlyphPointerTable writes the glyph pointer table to file
func (e *WFMFileEncoder) writeGlyphPointerTable(file *os.File, glyphPointerTable []uint16) error {
	for _, pointer := range glyphPointerTable {
		err := binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphPointer, err)
		}
	}
	return nil
}

// writeGlyphs writes all glyphs to file
func (e *WFMFileEncoder) writeGlyphs(file *os.File, glyphs []Glyph) error {
	for _, glyph := range glyphs {
		if err := e.writeSingleGlyph(file, glyph); err != nil {
			return err
		}
	}
	return nil
}

// writeSingleGlyph writes a single glyph to file
func (e *WFMFileEncoder) writeSingleGlyph(file *os.File, glyph Glyph) error {
	// Write glyph attributes
	if err := binary.Write(file, binary.LittleEndian, glyph.GlyphClut); err != nil {
		return common.FormatError(common.ErrFailedToWriteGlyphClut, err)
	}

	if err := binary.Write(file, binary.LittleEndian, glyph.GlyphHeight); err != nil {
		return common.FormatError(common.ErrFailedToWriteGlyphHeight, err)
	}

	if err := binary.Write(file, binary.LittleEndian, glyph.GlyphWidth); err != nil {
		return common.FormatError(common.ErrFailedToWriteGlyphWidth, err)
	}

	if err := binary.Write(file, binary.LittleEndian, glyph.GlyphHandakuten); err != nil {
		return common.FormatError(common.ErrFailedToWriteGlyphHandakuten, err)
	}

	// Write image data
	if _, err := file.Write(glyph.GlyphImage); err != nil {
		return common.FormatError(common.ErrFailedToWriteGlyphImage, err)
	}

	// Apply glyph padding
	return e.applyGlyphPadding(file, glyph)
}

// applyGlyphPadding applies padding for glyph alignment
func (e *WFMFileEncoder) applyGlyphPadding(file *os.File, glyph Glyph) error {
	// Safe conversion: ensure glyph image size doesn't cause overflow (already validated in buildWFMFile)
	safeGlyphSize, err := common.SafeIntToUint32(8 + len(glyph.GlyphImage))
	if err != nil {
		return fmt.Errorf("glyph size conversion failed: %w", err)
	}
	glyphSize := safeGlyphSize
	alignedGlyphSize := alignToBytes(glyphSize, 2)
	paddingSize := alignedGlyphSize - glyphSize
	if paddingSize > 0 {
		padding := make([]byte, paddingSize)
		if _, err := file.Write(padding); err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphPadding, err)
		}
	}
	return nil
}

// ensureDialogueAlignment ensures proper alignment before dialogue pointer table
func (e *WFMFileEncoder) ensureDialogueAlignment(file *os.File) error {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return common.FormatError(common.ErrFailedToGetFilePosition, err)
	}

	// Safe conversion: file position should not exceed uint32 range in reasonable cases
	if currentPos > (1<<32 - 1) {
		return common.FormatError(common.ErrFailedToGetFilePosition, fmt.Errorf("file too large: %d bytes", currentPos))
	}
	safeCurrentPos, err := common.SafeInt64ToUint32(currentPos)
	if err != nil {
		return common.FormatError(common.ErrFailedToGetFilePosition, fmt.Errorf("file position conversion failed: %w", err))
	}
	alignedPos := alignToBytes(safeCurrentPos, 2)
	paddingForTable := alignedPos - safeCurrentPos
	if paddingForTable > 0 {
		padding := make([]byte, paddingForTable)
		if _, err := file.Write(padding); err != nil {
			return common.FormatError(common.ErrFailedToWritePadding, err)
		}
	}
	return nil
}

// writeDialoguePointerTable writes the dialogue pointer table to file
func (e *WFMFileEncoder) writeDialoguePointerTable(file *os.File, dialoguePointerTable []uint16) error {
	for _, pointer := range dialoguePointerTable {
		err := binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteDialoguePointer, err)
		}
	}
	return nil
}

// writeDialogues writes all dialogues to file
func (e *WFMFileEncoder) writeDialogues(file *os.File, dialogues []Dialogue) error {
	for i, dialogue := range dialogues {
		if _, err := file.Write(dialogue.Data); err != nil {
			return common.FormatError(common.ErrFailedToWriteDialogueData, err)
		}

		// Apply dialogue padding (except for last dialogue)
		if err := e.applyDialoguePadding(file, dialogue, i, len(dialogues)); err != nil {
			return err
		}
	}
	return nil
}

// applyDialoguePadding applies padding for dialogue alignment
func (e *WFMFileEncoder) applyDialoguePadding(file *os.File, dialogue Dialogue, index, total int) error {
	// Safe conversion: dialogue data size already validated in buildWFMFile
	safeDialogueSize, err := common.SafeIntToUint16(len(dialogue.Data))
	if err != nil {
		return fmt.Errorf("dialogue size conversion failed: %w", err)
	}
	dialogueSize := safeDialogueSize
	alignedDialogueSize := alignToBytes16(dialogueSize, 2)
	paddingSize := alignedDialogueSize - dialogueSize
	if paddingSize > 0 && index < total-1 { // Don't apply padding to the last dialogue
		padding := make([]byte, paddingSize)
		if _, err := file.Write(padding); err != nil {
			return common.FormatError(common.ErrFailedToWriteDialoguePadding, err)
		}
	}
	return nil
}

// applyFinalPadding applies final padding to maintain original file size
func (e *WFMFileEncoder) applyFinalPadding(file *os.File) error {
	currentPos, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return common.FormatError(common.ErrFailedToGetFilePosition, err)
	}

	// If we have an original size and current file is smaller, pad with 0xFF
	if e.originalSize > 0 && currentPos < e.originalSize {
		paddingSize := e.originalSize - currentPos
		padding := make([]byte, paddingSize)
		// Fill with 0xFF
		for i := range padding {
			padding[i] = 0xFF
		}

		if _, err := file.Write(padding); err != nil {
			return common.FormatError(common.ErrFailedToWritePadding, err)
		}

		common.LogInfo("%s %d bytes of 0xFF padding to maintain original file size (%d bytes)",
			common.InfoPaddingAdded, paddingSize, e.originalSize)
	} else if e.originalSize > 0 && currentPos > e.originalSize {
		common.LogWarn(common.WarnEncodedFileLarger, currentPos, e.originalSize)
	}

	return nil
}

// loadSingleGlyph loads a single glyph from the fonts directory and converts it to 4bpp linear little endian
func (e *WFMFileEncoder) loadSingleGlyph(char rune, fontHeight int, fontClut uint16) (Glyph, error) {
	// Check for ignored characters first
	if char == '⧗' { // U+29D7 - ignore this character
		return Glyph{}, fmt.Errorf(common.ErrCharacterIgnoredNoGlyph)
	}

	// Determine PNG file path based on character
	glyphPath, err := e.getGlyphPath(char, fontHeight)
	if err != nil {
		return Glyph{}, err
	}

	// Load PNG image
	img, err := e.loadPNGImage(glyphPath)
	if err != nil {
		return Glyph{}, common.FormatErrorString(common.ErrFailedToLoadPNG, "%s: %w", glyphPath, err)
	}

	// Convert to 4bpp linear little endian using PSX tile processor
	processor := psx.NewPSXTileProcessor()

	// Get appropriate palette based on font height
	var palette psx.PSXPalette
	if fontHeight == 24 {
		palette = psx.NewPSXPalette(EventClut)
	} else {
		palette = psx.NewPSXPalette(DialogueClut)
	}

	tile, err := processor.ConvertTo4bppLinearLE(img, palette)
	if err != nil {
		return Glyph{}, common.FormatError(common.ErrFailedToConvertTo4bpp, err)
	}

	bounds := img.Bounds()

	// Safe conversions: image dimensions should be reasonable for glyphs
	height := bounds.Dy()
	width := bounds.Dx()
	if height < 0 || height > 65535 {
		return Glyph{}, fmt.Errorf("invalid glyph height: %d", height)
	}
	if width < 0 || width > 65535 {
		return Glyph{}, fmt.Errorf("invalid glyph width: %d", width)
	}

	safeHeight, err := common.SafeIntToUint16(height)
	if err != nil {
		return Glyph{}, fmt.Errorf("glyph height conversion failed: %w", err)
	}
	safeWidth, err := common.SafeIntToUint16(width)
	if err != nil {
		return Glyph{}, fmt.Errorf("glyph width conversion failed: %w", err)
	}

	glyph := Glyph{
		GlyphClut:       fontClut,
		GlyphHeight:     safeHeight,
		GlyphWidth:      safeWidth,
		GlyphHandakuten: 0,         // TODO: implement if necessary
		GlyphImage:      tile.Data, // Use tile data from PSX processor
	}

	return glyph, nil
}

// getGlyphPath determines the file path for a character's glyph PNG
func (e *WFMFileEncoder) getGlyphPath(char rune, fontHeight int) (string, error) {
	// Ignore the ⧗ character (U+29D7) - skip glyph loading for this character
	if char == '⧗' { // U+29D7
		return "", fmt.Errorf(common.ErrCharacterIgnored)
	}

	unicode := uint32(char)
	filename := fmt.Sprintf("%04X.png", unicode)

	// Handle special characters that map to 2B8B.png
	if char == '▼' || char == '⏷' { // U+25BC or U+23F7 -> 2B8B.png
		filename = "2B8B.png"
	}

	// Find the file in the corresponding height folder
	fontDir := filepath.Join("fonts", fmt.Sprintf("%d", fontHeight))

	// List all subfolders and search for the file
	subdirs := []string{"lowercase", "uppercase", "numbers", "symbols", "psx"}

	for _, subdir := range subdirs {
		glyphPath := filepath.Join(fontDir, subdir, filename)
		if _, err := os.Stat(glyphPath); err == nil {
			return glyphPath, nil
		}
	}

	return "", common.FormatErrorString(common.ErrGlyphFileNotFound, "'%c' (U+%04X)", char, char)
}

// loadPNGImage loads a PNG image from file
func (e *WFMFileEncoder) loadPNGImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	return img, nil
}

// NewWFMEncoder creates a new WFM encoder instance
func NewWFMEncoder() *WFMFileEncoder {
	return &WFMFileEncoder{}
}
