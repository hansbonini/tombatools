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
func (e *WFMFileEncoder) Encode(yamlFile string, outputFile string) error {
	// Load dialogues from YAML file
	dialogues, reservedData, err := e.LoadDialogues(yamlFile)
	if err != nil {
		return common.FormatError(common.ErrFailedToLoadDialogues, err)
	}

	// Step 1: Collect all unique characters used in dialogue text attributes
	uniqueChars, unmappedBytes := e.collectUniqueCharacters(dialogues)

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

	// Step 2: Map glyphs by dialogue considering font_height
	glyphMap, err := e.mapGlyphsByDialogue(dialogues)
	if err != nil {
		return common.FormatError(common.ErrFailedToMapGlyphs, err)
	}

	// Step 3: Assign encode values for each mapped glyph
	glyphEncodeMap, encodeValueMap, encodeOrder := e.assignEncodeValues(glyphMap)

	common.LogInfo("\n%s:", common.InfoGlyphMappingByHeight)
	for fontHeight, glyphs := range glyphMap {
		common.LogDebug(common.DebugFontHeightGlyphs, fontHeight, len(glyphs))
	}

	common.LogInfo("\n%s (0x8000-0x%04X) na ordem de adição:", common.InfoEncodeValuesAssigned, 0x8000+uint16(len(encodeValueMap))-1)

	// Display in the order they were added
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		common.LogDebug(common.DebugEncodeValue, encodeValue, glyphInfo.Character, glyphInfo.Character, glyphInfo.FontHeight)
	}

	// Step 4: Re-encode dialogue texts using the mapping
	recodedDialogues, err := e.recodeDialogueTexts(dialogues, glyphEncodeMap)
	if err != nil {
		return common.FormatError(common.ErrFailedToRecodeDialogues, err)
	}

	common.LogInfo("\n%s:", common.InfoRecodedTexts)
	for i, dialogue := range recodedDialogues {
		if i < 5 { // Show only the first 5 with more detail
			common.LogDebug(common.DebugDialogueEncoded, dialogue.ID, dialogue.OriginalText)
			common.LogDebug(common.DebugEncodedText, e.formatEncodedText(dialogue.EncodedText))
			common.LogDebug(common.DebugEncodedLength, len(dialogue.EncodedText)*2) // cada uint16 = 2 bytes
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

	// Step 5: Build the final WFM file
	wfmFile, err := e.buildWFMFile(glyphMap, encodeValueMap, encodeOrder, recodedDialogues, reservedData)
	if err != nil {
		return common.FormatError(common.ErrFailedToBuildWFM, err)
	}

	// Step 6: Write the WFM file
	err = e.writeWFMFile(wfmFile, outputFile)
	if err != nil {
		return common.FormatError(common.ErrFailedToWriteWFM, err)
	}

	common.LogInfo("\n%s: %s", common.InfoWFMFileCreated, outputFile)
	common.LogDebug(common.DebugHeaderInfo,
		string(wfmFile.Header.Magic[:]), wfmFile.Header.TotalDialogues, wfmFile.Header.TotalGlyphs)

	return nil
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
func (e *WFMFileEncoder) collectUniqueCharacters(dialogues []DialogueEntry) ([]rune, []string) {
	charSet := make(map[rune]bool)
	unmappedSet := make(map[string]bool)

	// Regex para identificar bytes sem mapeamento (formato [XXXX] com 4 dígitos hex maiúsculos)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	// Lista de tags especiais conhecidas que devem ser removidas
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

					// Primeiro, coletar bytes sem mapeamento antes de removê-los
					unmappedMatches := unmappedByteRegex.FindAllString(originalText, -1)
					for _, match := range unmappedMatches {
						unmappedSet[match] = true
					}

					cleanText := originalText

					// Remove tags especiais conhecidas
					for _, tag := range specialTags {
						cleanText = strings.ReplaceAll(cleanText, tag, "")
					}

					// Remove bytes sem mapeamento como [8030], [8031], etc. (formato %04X)
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
	var uniqueChars []rune
	for char := range charSet {
		uniqueChars = append(uniqueChars, char)
	}

	// Sort for consistent output
	sort.Slice(uniqueChars, func(i, j int) bool {
		return uniqueChars[i] < uniqueChars[j]
	})

	// Convert unmapped map to slice
	var unmappedBytes []string
	for unmapped := range unmappedSet {
		unmappedBytes = append(unmappedBytes, unmapped)
	}

	// Sort unmapped bytes for consistent output
	sort.Strings(unmappedBytes)

	return uniqueChars, unmappedBytes
}

// mapGlyphsByDialogue maps glyphs by dialogue considering font_height with global caching
func (e *WFMFileEncoder) mapGlyphsByDialogue(dialogues []DialogueEntry) (map[int]map[rune]Glyph, error) {
	// Dicionário global para evitar remapeamento: [fontHeight][char] = glyph
	globalGlyphCache := make(map[int]map[rune]Glyph)

	// Regex para identificar bytes sem mapeamento (formato [XXXX] com 4 dígitos hex maiúsculos)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	// Lista de tags especiais conhecidas que devem ser removidas
	specialTags := []string{
		"[FFF2]", "[HALT]", "[F4]", "[PROMPT]", "[F6]", "[CHANGE COLOR TO]",
		"[INIT TAIL]", "[PAUSE FOR]", "[WAIT FOR INPUT]", "[INIT TEXT BOX]",
	}

	for _, dialogue := range dialogues {
		fontHeight := int(dialogue.FontHeight)
		fontClut := dialogue.FontClut

		// Initialize the map for this font height if it doesn't exist
		if globalGlyphCache[fontHeight] == nil {
			globalGlyphCache[fontHeight] = make(map[rune]Glyph)
		}

		// Process content items to extract text
		for _, contentItem := range dialogue.Content {
			if textValue, exists := contentItem["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					// Clean the dialogue text
					cleanText := textStr

					// Remove tags especiais conhecidas
					for _, tag := range specialTags {
						cleanText = strings.ReplaceAll(cleanText, tag, "")
					}

					// Remove bytes sem mapeamento
					cleanText = unmappedByteRegex.ReplaceAllString(cleanText, "")

					// Remove line breaks
					cleanText = strings.ReplaceAll(cleanText, "\n", "")

					// Process each character
					for _, char := range cleanText {
						// Check if the character has already been mapped for this font height
						if _, exists := globalGlyphCache[fontHeight][char]; !exists {
							// Tentar carregar o glyph
							glyph, err := e.loadSingleGlyph(char, fontHeight, fontClut)
							if err != nil {
								// Check if this is an ignored character
								if char == '⧗' {
									// Silently skip ignored characters
									continue
								}
								common.LogWarn("%s '%c' (U+%04X) at font height %d: %v", common.WarnCouldNotLoadGlyph, char, char, fontHeight, err)
								continue
							}

							// Armazenar no cache global
							globalGlyphCache[fontHeight][char] = glyph
							common.LogDebug(common.DebugGlyphLoaded, common.InfoGlyphLoaded, char, char, fontHeight)
						}
					}
				}
			}
		}
	}

	return globalGlyphCache, nil
}

// assignEncodeValues assigns sequential encode values starting from 0x8000 to each mapped glyph
// Each combination of character + font height gets a unique encode value
func (e *WFMFileEncoder) assignEncodeValues(glyphMap map[int]map[rune]Glyph) (map[int]map[rune]uint16, map[uint16]GlyphEncodeInfo, []uint16) {
	// Mapa para armazenar o valor de encode de cada glyph: [fontHeight][char] = encodeValue
	glyphEncodeMap := make(map[int]map[rune]uint16)

	// Mapa reverso para lookup: [encodeValue] = GlyphEncodeInfo
	encodeValueMap := make(map[uint16]GlyphEncodeInfo)

	// Lista para manter a ordem de adição dos valores de encode
	var encodeOrder []uint16

	// Contador para valores sequenciais começando em 0x8000
	currentEncodeValue := uint16(0x8000)

	// Criar uma lista de todas as combinações (fontHeight, char) para ordenação consistente
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

	// Atribuir valores sequenciais para cada combinação única de char + fontHeight
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

		// Adicionar à lista de ordem
		encodeOrder = append(encodeOrder, currentEncodeValue)

		// Incrementar para o próximo valor
		currentEncodeValue++
	}

	return glyphEncodeMap, encodeValueMap, encodeOrder
}

// recodeDialogueTexts recodes dialogue content using the glyph encode mapping and handles content structure
func (e *WFMFileEncoder) recodeDialogueTexts(dialogues []DialogueEntry, glyphEncodeMap map[int]map[rune]uint16) ([]RecodedDialogue, error) {
	var recodedDialogues []RecodedDialogue

	// Regex para identificar bytes sem mapeamento (formato [XXXX] com 4 dígitos hex maiúsculos)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)

	// Lista de tags especiais conhecidas que devem ser convertidas para códigos especiais
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

	for _, dialogue := range dialogues {
		fontHeight := int(dialogue.FontHeight)

		// Verificar se temos mapeamento para esta altura de fonte
		// Nota: Permitir mapeamento vazio quando o diálogo só contém códigos especiais
		if glyphEncodeMap[fontHeight] == nil {
			// Inicializar mapeamento vazio se não existir
			glyphEncodeMap[fontHeight] = make(map[rune]uint16)
		}

		var encodedText []uint16
		var fullOriginalText strings.Builder

		// Process content items sequentially
		for _, contentItem := range dialogue.Content {
			// Handle box content
			if boxValue, exists := contentItem["box"]; exists {
				if boxMap, ok := boxValue.(map[string]interface{}); ok {
					encodedText = append(encodedText, INIT_TEXT_BOX)
					if width, hasWidth := boxMap["width"]; hasWidth {
						if w, ok := width.(int); ok {
							encodedText = append(encodedText, uint16(w))
						}
					}
					if height, hasHeight := boxMap["height"]; hasHeight {
						if h, ok := height.(int); ok {
							encodedText = append(encodedText, uint16(h))
						}
					}
				}
				continue
			}

			// Handle tail content
			if tailValue, exists := contentItem["tail"]; exists {
				if tailMap, ok := tailValue.(map[string]interface{}); ok {
					encodedText = append(encodedText, INIT_TAIL)
					if width, hasWidth := tailMap["width"]; hasWidth {
						if w, ok := width.(int); ok {
							encodedText = append(encodedText, uint16(w))
						}
					}
					if height, hasHeight := tailMap["height"]; hasHeight {
						if h, ok := height.(int); ok {
							encodedText = append(encodedText, uint16(h))
						}
					}
				}
				continue
			}

			// Handle f6 content
			if f6Value, exists := contentItem["f6"]; exists {
				if f6Map, ok := f6Value.(map[string]interface{}); ok {
					encodedText = append(encodedText, F6)
					if width, hasWidth := f6Map["width"]; hasWidth {
						if w, ok := width.(int); ok {
							encodedText = append(encodedText, uint16(w))
						}
					}
					if height, hasHeight := f6Map["height"]; hasHeight {
						if h, ok := height.(int); ok {
							encodedText = append(encodedText, uint16(h))
						}
					}
				}
				continue
			}

			// Handle color content
			if colorValue, exists := contentItem["color"]; exists {
				if colorMap, ok := colorValue.(map[string]interface{}); ok {
					encodedText = append(encodedText, CHANGE_COLOR_TO)
					if value, hasValue := colorMap["value"]; hasValue {
						if v, ok := value.(int); ok {
							encodedText = append(encodedText, uint16(v))
						}
					}
				}
				continue
			}

			// Handle pause content
			if pauseValue, exists := contentItem["pause"]; exists {
				if pauseMap, ok := pauseValue.(map[string]interface{}); ok {
					encodedText = append(encodedText, PAUSE_FOR)
					if duration, hasDuration := pauseMap["duration"]; hasDuration {
						if d, ok := duration.(int); ok {
							encodedText = append(encodedText, uint16(d))
						}
					}
				}
				continue
			}

			// Handle fff2 content
			if fff2Value, exists := contentItem["fff2"]; exists {
				if fff2Map, ok := fff2Value.(map[string]interface{}); ok {
					encodedText = append(encodedText, FFF2)
					if value, hasValue := fff2Map["value"]; hasValue {
						if v, ok := value.(int); ok {
							encodedText = append(encodedText, uint16(v))
						}
					}
				}
				continue
			}

			// Handle text content
			if textValue, exists := contentItem["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					fullOriginalText.WriteString(textStr)

					// Processar o texto caractere por caractere e tag por tag
					originalText := textStr
					runes := []rune(originalText)
					i := 0
					for i < len(runes) {
						// Verificar se é uma tag especial - convertendo de volta para string para verificar
						currentText := string(runes[i:])
						if len(currentText) > 0 && currentText[0] == '[' {
							tagProcessed := false

							// Verificar tags especiais conhecidas
							for tag, code := range specialTagMap {
								tagRunes := []rune(tag)
								if i+len(tagRunes) <= len(runes) {
									match := true
									for j, tagRune := range tagRunes {
										if runes[i+j] != tagRune {
											match = false
											break
										}
									}
									if match {
										encodedText = append(encodedText, code)
										i += len(tagRunes)
										tagProcessed = true
										break
									}
								}
							}

							if tagProcessed {
								continue
							}

							// Verificar se é um byte sem mapeamento [XXXX]
							remainingText := string(runes[i:])
							if len(remainingText) >= 6 {
								possibleUnmapped := remainingText[:6]
								if unmappedByteRegex.MatchString(possibleUnmapped) {
									// Pular bytes sem mapeamento (não incluir no encode)
									common.LogWarn("%s %s in dialogue %d", common.WarnSkippingUnmappedByte, possibleUnmapped, dialogue.ID)
									i += 6
									continue
								}
							}
						}

						// Handle special unicode characters
						if i < len(runes) {
							char := runes[i]

							// Handle ▼ (C04D unicode)
							if char == '▼' {
								encodedText = append(encodedText, C04D)
								i++
								continue
							}

							// Handle ⏷ (C04E unicode)
							if char == '⏷' {
								encodedText = append(encodedText, C04E)
								i++
								continue
							}

							// Handle ⧗ (WAIT_FOR_INPUT unicode)
							if char == '⧗' {
								encodedText = append(encodedText, WAIT_FOR_INPUT)
								i++
								continue
							}

							// Handle newlines - check for double newlines first
							if char == '\n' {
								// Check if this is a double newline (\n\n)
								if i+1 < len(runes) && runes[i+1] == '\n' {
									encodedText = append(encodedText, DOUBLE_NEWLINE)
									i += 2 // Skip both newline characters
								} else {
									encodedText = append(encodedText, NEWLINE)
									i++
								}
								continue
							}

							// Verificar se temos mapeamento para este caractere
							if encodeValue, exists := glyphEncodeMap[fontHeight][char]; exists {
								encodedText = append(encodedText, encodeValue)
							} else {
								common.LogWarn("%s '%c' (U+%04X) in dialogue %d", common.WarnNoEncodeMapping, char, char, dialogue.ID)
							}

							i++
						}
					}
				}
			}
		}

		// Add termination marker from dialogue terminator property
		// Convert terminator value (1 or 2) back to hex values
		var terminatorHex uint16
		switch dialogue.Terminator {
		case 1:
			terminatorHex = 0xFFFE // TERMINATOR_1
		case 2:
			terminatorHex = 0xFFFF // TERMINATOR_2
		default:
			terminatorHex = 0xFFFF // Default to TERMINATOR_2
		}
		encodedText = append(encodedText, terminatorHex)

		recodedDialogue := RecodedDialogue{
			ID:           dialogue.ID,
			Type:         dialogue.Type,
			FontHeight:   uint16(dialogue.FontHeight),
			OriginalText: fullOriginalText.String(),
			EncodedText:  encodedText,
		}

		recodedDialogues = append(recodedDialogues, recodedDialogue)
	}

	return recodedDialogues, nil
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

		// Limitar o número de valores exibidos para não poluir a saída
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
func alignToBytes(value uint32, alignment uint32) uint32 {
	if alignment == 0 {
		return value
	}
	return ((value + alignment - 1) / alignment) * alignment
}

// alignToBytes16 ensures a value is aligned to the specified byte boundary for uint16
func alignToBytes16(value uint16, alignment uint16) uint16 {
	if alignment == 0 {
		return value
	}
	return ((value + alignment - 1) / alignment) * alignment
}

// buildWFMFile constructs a complete WFM file from the processed data
func (e *WFMFileEncoder) buildWFMFile(glyphMap map[int]map[rune]Glyph, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16, recodedDialogues []RecodedDialogue, reservedData []byte) (*WFMFile, error) {
	// Criar lista ordenada de glifos usando encodeOrder
	var glyphs []Glyph
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		glyphs = append(glyphs, glyphInfo.Glyph)
	}

	// Converter diálogos recodificados para formato WFM
	// Primeiro, ordenar os diálogos por ID para garantir a sequência correta
	sort.Slice(recodedDialogues, func(i, j int) bool {
		return recodedDialogues[i].ID < recodedDialogues[j].ID
	})

	var dialogues []Dialogue
	for _, recodedDialogue := range recodedDialogues {
		// Converter os valores uint16 para bytes (little endian)
		var dialogueData []byte
		for _, value := range recodedDialogue.EncodedText {
			// Adicionar valor em little endian (2 bytes)
			dialogueData = append(dialogueData, byte(value&0xFF))      // byte baixo
			dialogueData = append(dialogueData, byte((value>>8)&0xFF)) // byte alto
		}

		dialogue := Dialogue{
			Data: dialogueData,
		}
		dialogues = append(dialogues, dialogue)
	}

	// Calcular ponteiros dos glifos (relativos ao início do arquivo WFM - posição 0x0)
	var glyphPointerTable []uint16
	headerSize := uint32(4 + 4 + 4 + 2 + 2 + 128)             // Magic + Padding + DialoguePointerTable + TotalDialogues + TotalGlyphs + Reserved
	glyphTableSize := uint32(len(glyphs) * 2)                 // Tamanho da tabela de ponteiros dos glifos
	currentGlyphOffset := uint32(headerSize + glyphTableSize) // Início dos dados dos glifos

	for _, glyph := range glyphs {
		glyphPointerTable = append(glyphPointerTable, uint16(currentGlyphOffset))
		// Cada glyph tem: 2+2+2+2 = 8 bytes de atributos + tamanho da imagem
		glyphSize := 8 + len(glyph.GlyphImage)
		currentGlyphOffset += uint32(glyphSize)
	}

	// Calcular ponteiros dos diálogos (relativos ao início da tabela de ponteiros dos diálogos)
	var dialoguePointerTable []uint16
	currentDialogueOffset := uint16(len(dialogues) * 2) // Tamanho da tabela de ponteiros

	// Garantir que os dados dos diálogos sejam byte-aligned (alinhamento de 2 bytes)
	currentDialogueOffset = alignToBytes16(currentDialogueOffset, 2)

	for _, dialogue := range dialogues {
		dialoguePointerTable = append(dialoguePointerTable, currentDialogueOffset)
		dialogueSize := uint16(len(dialogue.Data))
		// Garantir que cada diálogo seja byte-aligned
		alignedDialogueSize := alignToBytes16(dialogueSize, 2)
		currentDialogueOffset += alignedDialogueSize
	}

	// Calcular posição da tabela de ponteiros dos diálogos
	totalGlyphsSize := uint32(0)
	for _, glyph := range glyphs {
		glyphSize := uint32(8 + len(glyph.GlyphImage))
		// Garantir que cada glifo seja byte-aligned
		alignedGlyphSize := alignToBytes(glyphSize, 2)
		totalGlyphsSize += alignedGlyphSize
	}

	dialoguePointerTableOffset := headerSize + glyphTableSize + totalGlyphsSize
	// Garantir que a tabela de ponteiros dos diálogos seja byte-aligned
	dialoguePointerTableOffset = alignToBytes(dialoguePointerTableOffset, 2)

	// Criar header
	var reservedBytes [128]byte
	if len(reservedData) > 0 {
		// Ensure reservedData is exactly 128 bytes
		if len(reservedData) != 128 {
			return nil, common.FormatErrorString(common.ErrReservedDataSize, "got %d", len(reservedData))
		}
		// Use reserved data from special dialogues
		copy(reservedBytes[:], reservedData)
	}
	// If reservedData is empty or nil, reservedBytes remains zero-filled

	common.LogInfo("%s: %d bytes", common.InfoReservedSectionUsed, len(reservedBytes))

	header := WFMHeader{
		Magic:                [4]byte{'W', 'F', 'M', '3'},
		Padding:              0,
		DialoguePointerTable: dialoguePointerTableOffset,
		TotalDialogues:       uint16(len(dialogues)),
		TotalGlyphs:          uint16(len(glyphs)),
		Reserved:             reservedBytes,
	}

	// Criar arquivo WFM completo
	wfmFile := &WFMFile{
		Header:               header,
		GlyphPointerTable:    glyphPointerTable,
		Glyphs:               glyphs,
		DialoguePointerTable: dialoguePointerTable,
		Dialogues:            dialogues,
	}

	return wfmFile, nil
}

// writeWFMFile escreve o arquivo WFM no disco
func (e *WFMFileEncoder) writeWFMFile(wfm *WFMFile, outputFile string) error {
	file, err := os.Create(outputFile)
	if err != nil {
		return common.FormatError(common.ErrFailedToCreateOutputFile, err)
	}
	defer file.Close()

	// Escrever header
	err = binary.Write(file, binary.LittleEndian, wfm.Header)
	if err != nil {
		return common.FormatError(common.ErrFailedToWriteHeader, err)
	}

	// Escrever tabela de ponteiros dos glifos
	for _, pointer := range wfm.GlyphPointerTable {
		err = binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphPointer, err)
		}
	}

	// Escrever glifos
	for _, glyph := range wfm.Glyphs {
		// Escrever atributos do glyph
		err = binary.Write(file, binary.LittleEndian, glyph.GlyphClut)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphClut, err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphHeight)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphHeight, err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphWidth)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphWidth, err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphHandakuten)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphHandakuten, err)
		}

		// Escrever dados da imagem
		_, err = file.Write(glyph.GlyphImage)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteGlyphImage, err)
		}

		// Aplicar padding para garantir alinhamento de 2 bytes para cada glifo
		glyphSize := uint32(8 + len(glyph.GlyphImage))
		alignedGlyphSize := alignToBytes(glyphSize, 2)
		paddingSize := alignedGlyphSize - glyphSize
		if paddingSize > 0 {
			padding := make([]byte, paddingSize)
			_, err = file.Write(padding)
			if err != nil {
				return common.FormatError(common.ErrFailedToWriteGlyphPadding, err)
			}
		}
	}

	// Garantir que a posição atual esteja alinhada antes da tabela de ponteiros dos diálogos
	var currentPos int64
	currentPos, err = file.Seek(0, io.SeekCurrent)
	if err != nil {
		return common.FormatError(common.ErrFailedToGetFilePosition, err)
	}

	alignedPos := alignToBytes(uint32(currentPos), 2)
	paddingForTable := alignedPos - uint32(currentPos)
	if paddingForTable > 0 {
		padding := make([]byte, paddingForTable)
		_, err = file.Write(padding)
		if err != nil {
			return common.FormatError(common.ErrFailedToWritePadding, err)
		}
	}

	// Escrever tabela de ponteiros dos diálogos
	for _, pointer := range wfm.DialoguePointerTable {
		err = binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteDialoguePointer, err)
		}
	}

	// Escrever diálogos
	for i, dialogue := range wfm.Dialogues {
		_, err = file.Write(dialogue.Data)
		if err != nil {
			return common.FormatError(common.ErrFailedToWriteDialogueData, err)
		}

		// Aplicar padding para garantir alinhamento de 2 bytes para cada diálogo
		dialogueSize := uint16(len(dialogue.Data))
		alignedDialogueSize := alignToBytes16(dialogueSize, 2)
		paddingSize := alignedDialogueSize - dialogueSize
		if paddingSize > 0 && i < len(wfm.Dialogues)-1 { // Não aplicar padding no último diálogo
			padding := make([]byte, paddingSize)
			_, err = file.Write(padding)
			if err != nil {
				return common.FormatError(common.ErrFailedToWriteDialoguePadding, err)
			}
		}
	}

	// Get current file size and apply padding if necessary
	currentPos, err = file.Seek(0, io.SeekCurrent)
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

		_, err = file.Write(padding)
		if err != nil {
			return common.FormatError(common.ErrFailedToWritePadding, err)
		}

		common.LogInfo("%s %d bytes of 0xFF padding to maintain original file size (%d bytes)",
			common.InfoPaddingAdded, paddingSize, e.originalSize)
	} else if e.originalSize > 0 && currentPos > e.originalSize {
		common.LogWarn(common.WarnEncodedFileLarger, currentPos, e.originalSize)
	}

	return nil
}

// loadGlyphsForCharacters loads and converts glyphs for the given characters from the fonts directory
func (e *WFMFileEncoder) loadGlyphsForCharacters(characters []rune) ([]Glyph, error) {
	var glyphs []Glyph
	fontHeight := 16         // Default font height, pode ser configurável no futuro
	defaultClut := uint16(0) // CLUT padrão

	for _, char := range characters {
		glyph, err := e.loadSingleGlyph(char, fontHeight, defaultClut)
		if err != nil {
			// Check if this is an ignored character
			if char == '⧗' {
				// Silently skip ignored characters
				continue
			}
			// Se não encontrar o glyph, log o erro mas continue
			common.LogWarn("%s '%c' (U+%04X): %v", common.WarnCouldNotLoadGlyph, char, char, err)
			continue
		}
		glyphs = append(glyphs, glyph)
	}

	return glyphs, nil
}

// loadSingleGlyph loads a single glyph from the fonts directory and converts it to 4bpp linear little endian
func (e *WFMFileEncoder) loadSingleGlyph(char rune, fontHeight int, fontClut uint16) (Glyph, error) {
	// Check for ignored characters first
	if char == '⧗' { // U+29D7 - ignore this character
		return Glyph{}, fmt.Errorf(common.ErrCharacterIgnoredNoGlyph)
	}

	// Determinar o caminho do arquivo PNG baseado no caractere
	glyphPath, err := e.getGlyphPath(char, fontHeight)
	if err != nil {
		return Glyph{}, err
	}

	// Carregar a imagem PNG
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
	glyph := Glyph{
		GlyphClut:       fontClut,
		GlyphHeight:     uint16(bounds.Dy()),
		GlyphWidth:      uint16(bounds.Dx()),
		GlyphHandakuten: 0, // TODO: implementar se necessário
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

	// Buscar o arquivo na pasta da altura correspondente
	fontDir := filepath.Join("fonts", fmt.Sprintf("%d", fontHeight))

	// Listar todas as subpastas e procurar o arquivo
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
