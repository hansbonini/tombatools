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

	"gopkg.in/yaml.v3"
)

// WFMFileEncoder implements the WFMEncoder interface
type WFMFileEncoder struct {
	originalSize int64 // Store original file size for padding
}

// GlyphEncodeInfo holds information about a glyph and its assigned encode value
type GlyphEncodeInfo struct {
	Character  rune
	FontHeight int
	Glyph      Glyph
}

// RecodedDialogue represents a dialogue with recoded text
type RecodedDialogue struct {
	ID           int
	Type         string
	FontHeight   uint16
	OriginalText string
	EncodedText  []uint16
}

// Encode creates a WFM file from a YAML dialogue file and associated glyph directory
func (e *WFMFileEncoder) Encode(yamlFile string, outputFile string) error {
	// Load dialogues from YAML
	dialogues, reservedData, err := e.LoadDialogues(yamlFile)
	if err != nil {
		return fmt.Errorf("failed to load dialogues: %w", err)
	}

	// Primeiro passo: fazer levantamento geral dos caracteres usados nos atributos text dos diálogos
	uniqueChars, unmappedBytes := e.collectUniqueCharacters(dialogues)

	fmt.Printf("Levantamento de caracteres únicos encontrados:\n")
	fmt.Printf("Total de caracteres únicos: %d\n", len(uniqueChars))

	// Mostrar os caracteres ordenados
	for i, char := range uniqueChars {
		fmt.Printf("Char %d: '%c' (U+%04X)\n", i, char, char)
	}

	// Mostrar bytes sem mapeamento encontrados
	if len(unmappedBytes) > 0 {
		fmt.Printf("\nBytes sem mapeamento encontrados:\n")
		fmt.Printf("Total de bytes sem mapeamento: %d\n", len(unmappedBytes))
		for i, unmappedByte := range unmappedBytes {
			fmt.Printf("Unmapped %d: %s\n", i, unmappedByte)
		}
		fmt.Printf("\nNota: Estes bytes precisam ser manualmente adicionados à fonte no futuro.\n")
	}

	// Segundo passo: mapear glifos por diálogo considerando font_height
	glyphMap, err := e.mapGlyphsByDialogue(dialogues)
	if err != nil {
		return fmt.Errorf("failed to map glyphs: %w", err)
	}

	// Terceiro passo: atribuir valores de encode para cada glyph mapeado
	glyphEncodeMap, encodeValueMap, encodeOrder := e.assignEncodeValues(glyphMap)

	fmt.Printf("\nMapeamento de glifos por altura de fonte:\n")
	for fontHeight, glyphs := range glyphMap {
		fmt.Printf("Font Height %d: %d glifos mapeados\n", fontHeight, len(glyphs))
	}

	fmt.Printf("\nValores de encode atribuídos (0x8000-0x%04X) na ordem de adição:\n", 0x8000+uint16(len(encodeValueMap))-1)

	// Exibir na ordem em que foram adicionados
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		fmt.Printf("0x%04X -> '%c' (U+%04X) at font height %d\n", encodeValue, glyphInfo.Character, glyphInfo.Character, glyphInfo.FontHeight)
	}

	// Quarto passo: recodificar os textos dos diálogos usando o mapeamento
	recodedDialogues, err := e.recodeDialogueTexts(dialogues, glyphEncodeMap)
	if err != nil {
		return fmt.Errorf("failed to recode dialogue texts: %w", err)
	}

	fmt.Printf("\nTextos recodificados:\n")
	for i, dialogue := range recodedDialogues {
		if i < 5 { // Mostrar apenas os primeiros 5 com mais detalhe
			fmt.Printf("Dialogue %d ('%s'):\n", dialogue.ID, dialogue.OriginalText)
			fmt.Printf("  Encoded: %s\n", e.formatEncodedText(dialogue.EncodedText))
			fmt.Printf("  Length: %d bytes\n\n", len(dialogue.EncodedText)*2) // cada uint16 = 2 bytes
		}
	}
	if len(recodedDialogues) > 5 {
		fmt.Printf("... e mais %d diálogos recodificados\n", len(recodedDialogues)-5)
	}

	fmt.Printf("\nEstatísticas de recodificação:\n")
	fmt.Printf("Total de diálogos processados: %d\n", len(recodedDialogues))

	totalEncodedBytes := 0
	for _, dialogue := range recodedDialogues {
		totalEncodedBytes += len(dialogue.EncodedText) * 2 // cada uint16 = 2 bytes
	}
	fmt.Printf("Total de bytes codificados: %d\n", totalEncodedBytes)

	// Quinto passo: montar o arquivo WFM final
	wfmFile, err := e.buildWFMFile(glyphMap, encodeValueMap, encodeOrder, recodedDialogues, reservedData)
	if err != nil {
		return fmt.Errorf("failed to build WFM file: %w", err)
	}

	// Sexto passo: escrever o arquivo WFM
	err = e.writeWFMFile(wfmFile, outputFile)
	if err != nil {
		return fmt.Errorf("failed to write WFM file: %w", err)
	}

	fmt.Printf("\nArquivo WFM criado com sucesso: %s\n", outputFile)
	fmt.Printf("Header: Magic=%s, Diálogos=%d, Glifos=%d\n",
		string(wfmFile.Header.Magic[:]), wfmFile.Header.TotalDialogues, wfmFile.Header.TotalGlyphs)

	return nil
}

// LoadDialogues loads dialogue entries from YAML file
func (e *WFMFileEncoder) LoadDialogues(yamlFile string) ([]DialogueEntry, []byte, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var yamlData struct {
		TotalDialogues int             `yaml:"total_dialogues"`
		OriginalSize   int64           `yaml:"original_size"`
		Dialogues      []DialogueEntry `yaml:"dialogues"`
	}

	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML: %w", err)
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
		fmt.Printf("No special dialogues found - Reserved section will be zero-filled (128 bytes)\n")
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
			fmt.Printf("Warning: Too many special dialogues (%d), only first %d will be stored\n", len(specialDialogueIDs), maxEntries)
			break
		}

		if byteIndex+1 < len(reservedData) {
			// Store ID as uint16 little endian
			reservedData[byteIndex] = byte(id & 0xFF)          // Low byte
			reservedData[byteIndex+1] = byte((id >> 8) & 0xFF) // High byte
			byteIndex += 2
		}
	}

	fmt.Printf("Special dialogues found: %v\n", specialDialogueIDs)
	fmt.Printf("Reserved section built with %d special dialogue IDs (128 bytes total)\n", len(specialDialogueIDs))

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

					// Remove quebras de linha que podem vir das tags
					cleanText = strings.ReplaceAll(cleanText, "\n", "")

					// Agora conta apenas os caracteres reais que precisam de mapeamento
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

		// Inicializar o mapa para esta altura de fonte se não existir
		if globalGlyphCache[fontHeight] == nil {
			globalGlyphCache[fontHeight] = make(map[rune]Glyph)
		}

		// Process content items to extract text
		for _, contentItem := range dialogue.Content {
			if textValue, exists := contentItem["text"]; exists {
				if textStr, ok := textValue.(string); ok {
					// Limpar o texto do diálogo
					cleanText := textStr

					// Remove tags especiais conhecidas
					for _, tag := range specialTags {
						cleanText = strings.ReplaceAll(cleanText, tag, "")
					}

					// Remove bytes sem mapeamento
					cleanText = unmappedByteRegex.ReplaceAllString(cleanText, "")

					// Remove quebras de linha
					cleanText = strings.ReplaceAll(cleanText, "\n", "")

					// Processar cada caractere
					for _, char := range cleanText {
						// Verificar se o caractere já foi mapeado para esta altura de fonte
						if _, exists := globalGlyphCache[fontHeight][char]; !exists {
							// Tentar carregar o glyph
							glyph, err := e.loadSingleGlyph(char, fontHeight, fontClut)
							if err != nil {
								fmt.Printf("Warning: Could not load glyph for character '%c' (U+%04X) at font height %d: %v\n", char, char, fontHeight, err)
								continue
							}

							// Armazenar no cache global
							globalGlyphCache[fontHeight][char] = glyph
							fmt.Printf("Loaded glyph for '%c' (U+%04X) at font height %d\n", char, char, fontHeight)
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

	// Ordenar por altura de fonte primeiro, depois por caractere
	// Isso garante que glifos da mesma altura fiquem agrupados, mas cada char+altura é único
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

		// Inicializar o mapa para esta altura de fonte se não existir
		if glyphEncodeMap[fontHeight] == nil {
			glyphEncodeMap[fontHeight] = make(map[rune]uint16)
		}

		// Atribuir o valor de encode (cada char+altura é tratado como glifo único)
		glyphEncodeMap[fontHeight][char] = currentEncodeValue

		// Armazenar informações no mapa reverso
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
									fmt.Printf("Warning: Skipping unmapped byte %s in dialogue %d\n", possibleUnmapped, dialogue.ID)
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
								fmt.Printf("Warning: No encode mapping found for character '%c' (U+%04X) in dialogue %d\n", char, char, dialogue.ID)
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
			return nil, fmt.Errorf("reservedData must be exactly 128 bytes, got %d", len(reservedData))
		}
		// Use reserved data from special dialogues
		copy(reservedBytes[:], reservedData)
	}
	// If reservedData is empty or nil, reservedBytes remains zero-filled

	fmt.Printf("Reserved section: %d bytes used in header\n", len(reservedBytes))

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
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Escrever header
	err = binary.Write(file, binary.LittleEndian, wfm.Header)
	if err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Escrever tabela de ponteiros dos glifos
	for _, pointer := range wfm.GlyphPointerTable {
		err = binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return fmt.Errorf("failed to write glyph pointer: %w", err)
		}
	}

	// Escrever glifos
	for _, glyph := range wfm.Glyphs {
		// Escrever atributos do glyph
		err = binary.Write(file, binary.LittleEndian, glyph.GlyphClut)
		if err != nil {
			return fmt.Errorf("failed to write glyph clut: %w", err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphHeight)
		if err != nil {
			return fmt.Errorf("failed to write glyph height: %w", err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphWidth)
		if err != nil {
			return fmt.Errorf("failed to write glyph width: %w", err)
		}

		err = binary.Write(file, binary.LittleEndian, glyph.GlyphHandakuten)
		if err != nil {
			return fmt.Errorf("failed to write glyph handakuten: %w", err)
		}

		// Escrever dados da imagem
		_, err = file.Write(glyph.GlyphImage)
		if err != nil {
			return fmt.Errorf("failed to write glyph image: %w", err)
		}

		// Aplicar padding para garantir alinhamento de 2 bytes para cada glifo
		glyphSize := uint32(8 + len(glyph.GlyphImage))
		alignedGlyphSize := alignToBytes(glyphSize, 2)
		paddingSize := alignedGlyphSize - glyphSize
		if paddingSize > 0 {
			padding := make([]byte, paddingSize)
			_, err = file.Write(padding)
			if err != nil {
				return fmt.Errorf("failed to write glyph padding: %w", err)
			}
		}
	}

	// Garantir que a posição atual esteja alinhada antes da tabela de ponteiros dos diálogos
	var currentPos int64
	currentPos, err = file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
	}

	alignedPos := alignToBytes(uint32(currentPos), 2)
	paddingForTable := alignedPos - uint32(currentPos)
	if paddingForTable > 0 {
		padding := make([]byte, paddingForTable)
		_, err = file.Write(padding)
		if err != nil {
			return fmt.Errorf("failed to write table alignment padding: %w", err)
		}
	}

	// Escrever tabela de ponteiros dos diálogos
	for _, pointer := range wfm.DialoguePointerTable {
		err = binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return fmt.Errorf("failed to write dialogue pointer: %w", err)
		}
	}

	// Escrever diálogos
	for i, dialogue := range wfm.Dialogues {
		_, err = file.Write(dialogue.Data)
		if err != nil {
			return fmt.Errorf("failed to write dialogue data: %w", err)
		}

		// Aplicar padding para garantir alinhamento de 2 bytes para cada diálogo
		dialogueSize := uint16(len(dialogue.Data))
		alignedDialogueSize := alignToBytes16(dialogueSize, 2)
		paddingSize := alignedDialogueSize - dialogueSize
		if paddingSize > 0 && i < len(wfm.Dialogues)-1 { // Não aplicar padding no último diálogo
			padding := make([]byte, paddingSize)
			_, err = file.Write(padding)
			if err != nil {
				return fmt.Errorf("failed to write dialogue padding: %w", err)
			}
		}
	}

	// Get current file size and apply padding if necessary
	currentPos, err = file.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get current file position: %w", err)
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
			return fmt.Errorf("failed to write padding: %w", err)
		}

		fmt.Printf("Added %d bytes of 0xFF padding to maintain original file size (%d bytes)\n",
			paddingSize, e.originalSize)
	} else if e.originalSize > 0 && currentPos > e.originalSize {
		fmt.Printf("Warning: Encoded file (%d bytes) is larger than original (%d bytes)\n",
			currentPos, e.originalSize)
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
			// Se não encontrar o glyph, log o erro mas continue
			fmt.Printf("Warning: Could not load glyph for character '%c' (U+%04X): %v\n", char, char, err)
			continue
		}
		glyphs = append(glyphs, glyph)
	}

	return glyphs, nil
}

// loadSingleGlyph loads a single glyph from the fonts directory and converts it to 4bpp linear little endian
func (e *WFMFileEncoder) loadSingleGlyph(char rune, fontHeight int, fontClut uint16) (Glyph, error) {
	// Determinar o caminho do arquivo PNG baseado no caractere
	glyphPath, err := e.getGlyphPath(char, fontHeight)
	if err != nil {
		return Glyph{}, err
	}

	// Carregar a imagem PNG
	img, err := e.loadPNGImage(glyphPath)
	if err != nil {
		return Glyph{}, fmt.Errorf("failed to load PNG %s: %w", glyphPath, err)
	}

	// Converter para 4bpp linear little endian
	imageData, err := e.convertTo4bppLinearLE(img, fontHeight)
	if err != nil {
		return Glyph{}, fmt.Errorf("failed to convert to 4bpp: %w", err)
	}

	bounds := img.Bounds()
	glyph := Glyph{
		GlyphClut:       fontClut,
		GlyphHeight:     uint16(bounds.Dy()),
		GlyphWidth:      uint16(bounds.Dx()),
		GlyphHandakuten: 0, // TODO: implementar se necessário
		GlyphImage:      imageData,
	}

	return glyph, nil
}

// getGlyphPath determines the file path for a character's glyph PNG
func (e *WFMFileEncoder) getGlyphPath(char rune, fontHeight int) (string, error) {
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

	return "", fmt.Errorf("glyph file not found for character '%c' (U+%04X)", char, char)
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

// convertTo4bppLinearLE converts an image to 4bpp linear little endian format using proper CLUT palette mapping
func (e *WFMFileEncoder) convertTo4bppLinearLE(img image.Image, fontHeight int) ([]byte, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Cada pixel usa 4 bits, então 2 pixels por byte
	// Se a largura for ímpar, precisamos adicionar padding
	bytesPerRow := (width + 1) / 2
	totalBytes := bytesPerRow * height

	data := make([]byte, totalBytes)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x += 2 {
			byteIndex := y*bytesPerRow + x/2

			// Primeiro pixel (4 bits baixos)
			pixel1 := e.getPixelIntensity(img, bounds.Min.X+x, bounds.Min.Y+y, fontHeight)

			// Segundo pixel (4 bits altos), se existir
			var pixel2 uint8
			if x+1 < width {
				pixel2 = e.getPixelIntensity(img, bounds.Min.X+x+1, bounds.Min.Y+y, fontHeight)
			}

			// Combinar os dois pixels em um byte (little endian: pixel1 nos bits baixos)
			data[byteIndex] = pixel1 | (pixel2 << 4)
		}
	}

	return data, nil
}

// getPixelIntensity gets the 4-bit palette index value of a pixel by matching RGB colors to CLUT palette
func (e *WFMFileEncoder) getPixelIntensity(img image.Image, x, y int, fontHeight int) uint8 {
	r, g, b, a := img.At(x, y).RGBA()

	// Se o pixel for transparente, retornar 0
	if a == 0 {
		return 0
	}

	// Converter de 16-bit para 8-bit RGB
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)

	// Selecionar a paleta correta baseada na altura da fonte
	var currentPalette [16]uint16
	if fontHeight == 24 {
		// Use EventClut para altura 24
		currentPalette = EventClut
	} else {
		// Use DialogueClut para outras alturas
		currentPalette = DialogueClut
	}

	// Função para converter cor PSX 16-bit para RGB 8-bit
	psxToRGB := func(psxColor uint16) (uint8, uint8, uint8) {
		if psxColor == 0 {
			return 0, 0, 0 // Cor 0 é transparente/preta
		}
		r := uint8((psxColor & 0x1F) << 3)         // Red: bits 0-4
		g := uint8(((psxColor >> 5) & 0x1F) << 3)  // Green: bits 5-9
		b := uint8(((psxColor >> 10) & 0x1F) << 3) // Blue: bits 10-14
		return r, g, b
	}

	// Procurar a cor mais próxima na paleta
	bestMatch := uint8(0)
	minDistance := uint32(0xFFFFFFFF)

	for i, psxColor := range currentPalette {
		pr, pg, pb := psxToRGB(psxColor)

		// Calcular distância euclidiana no espaço RGB
		dr := int32(r8) - int32(pr)
		dg := int32(g8) - int32(pg)
		db := int32(b8) - int32(pb)
		distance := uint32(dr*dr + dg*dg + db*db)

		if distance < minDistance {
			minDistance = distance
			bestMatch = uint8(i)
		}

		// Se encontrou uma correspondência exata, parar
		if distance == 0 {
			break
		}
	}

	return bestMatch
}

// NewWFMEncoder creates a new WFM encoder instance
func NewWFMEncoder() *WFMFileEncoder {
	return &WFMFileEncoder{}
}
