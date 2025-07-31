package pkg

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// WFMFileEncoder implements the WFMEncoder interface
type WFMFileEncoder struct{}

// GlyphEncodeInfo holds information about a glyph and its assigned encode value
type GlyphEncodeInfo struct {
	Character  rune
	FontHeight int
	Glyph      Glyph
}

// RecodedDialogue represents a dialogue with recoded text
type RecodedDialogue struct {
	ID          int
	Type        string
	FontHeight  uint16
	OriginalText string
	EncodedText []uint16
}

// Encode creates a WFM file from a YAML dialogue file and associated glyph directory
func (e *WFMFileEncoder) Encode(yamlFile string, outputFile string) error {
	// Load dialogues from YAML
	dialogues, err := e.LoadDialogues(yamlFile)
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
	wfmFile, err := e.buildWFMFile(glyphMap, encodeValueMap, encodeOrder, recodedDialogues)
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
func (e *WFMFileEncoder) LoadDialogues(yamlFile string) ([]DialogueEntry, error) {
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var yamlData struct {
		TotalDialogues int             `yaml:"total_dialogues"`
		Dialogues      []DialogueEntry `yaml:"dialogues"`
	}

	if err := yaml.Unmarshal(data, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return yamlData.Dialogues, nil
}

// collectUniqueCharacters collects all unique characters from dialogue texts and returns unmapped bytes
func (e *WFMFileEncoder) collectUniqueCharacters(dialogues []DialogueEntry) ([]rune, []string) {
	charSet := make(map[rune]bool)
	unmappedSet := make(map[string]bool)
	
	// Regex para identificar bytes sem mapeamento (formato [XXXX] com 4 dígitos hex maiúsculos)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)
	
	// Lista de tags especiais conhecidas que devem ser removidas
	specialTags := []string{
		"[HALT]", "[F4]", "[PROMPT]", "[F6]", "[CHANGE COLOR TO]", 
		"[INIT TAIL]", "[PAUSE FOR]", "[WAIT FOR INPUT]", "[INIT TEXT BOX]",
	}
	
	for _, dialogue := range dialogues {
		originalText := dialogue.Text
		
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
		"[HALT]", "[F4]", "[PROMPT]", "[F6]", "[CHANGE COLOR TO]", 
		"[INIT TAIL]", "[PAUSE FOR]", "[WAIT FOR INPUT]", "[INIT TEXT BOX]",
	}
	
	for _, dialogue := range dialogues {
		fontHeight := int(dialogue.FontHeight)
		fontClut := dialogue.FontClut
		
		// Inicializar o mapa para esta altura de fonte se não existir
		if globalGlyphCache[fontHeight] == nil {
			globalGlyphCache[fontHeight] = make(map[rune]Glyph)
		}
		
		// Limpar o texto do diálogo
		cleanText := dialogue.Text
		
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
	
	return globalGlyphCache, nil
}

// assignEncodeValues assigns sequential encode values starting from 0x8000 to each mapped glyph
func (e *WFMFileEncoder) assignEncodeValues(glyphMap map[int]map[rune]Glyph) (map[int]map[rune]uint16, map[uint16]GlyphEncodeInfo, []uint16) {
	// Mapa para armazenar o valor de encode de cada glyph: [fontHeight][char] = encodeValue
	glyphEncodeMap := make(map[int]map[rune]uint16)
	
	// Mapa reverso para lookup: [encodeValue] = GlyphEncodeInfo
	encodeValueMap := make(map[uint16]GlyphEncodeInfo)
	
	// Lista para manter a ordem de adição dos valores de encode
	var encodeOrder []uint16
	
	// Contador para valores sequenciais começando em 0x8000
	currentEncodeValue := uint16(0x8000)
	
	// Processar cada altura de fonte
	for fontHeight, glyphs := range glyphMap {
		// Inicializar o mapa para esta altura de fonte
		if glyphEncodeMap[fontHeight] == nil {
			glyphEncodeMap[fontHeight] = make(map[rune]uint16)
		}
		
		// Criar uma lista ordenada de caracteres para atribuição consistente
		var sortedChars []rune
		for char := range glyphs {
			sortedChars = append(sortedChars, char)
		}
		
		// Ordenar os caracteres para atribuição consistente
		sort.Slice(sortedChars, func(i, j int) bool {
			return sortedChars[i] < sortedChars[j]
		})
		
		// Atribuir valores sequenciais
		for _, char := range sortedChars {
			glyph := glyphs[char]
			
			// Atribuir o valor de encode
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
	}
	
	return glyphEncodeMap, encodeValueMap, encodeOrder
}

// recodeDialogueTexts recodes dialogue texts using the glyph encode mapping
func (e *WFMFileEncoder) recodeDialogueTexts(dialogues []DialogueEntry, glyphEncodeMap map[int]map[rune]uint16) ([]RecodedDialogue, error) {
	var recodedDialogues []RecodedDialogue
	
	// Regex para identificar bytes sem mapeamento (formato [XXXX] com 4 dígitos hex maiúsculos)
	unmappedByteRegex := regexp.MustCompile(`\[[0-9A-F]{4}\]`)
	
	// Lista de tags especiais conhecidas que devem ser convertidas para códigos especiais
	specialTagMap := map[string]uint16{
		"[HALT]":          HALT,
		"[F4]":            F4,
		"[PROMPT]":        PROMPT,
		"[F6]":            F6,
		"[CHANGE COLOR TO]": 0xFFF2, // TODO: verificar código correto
		"[INIT TAIL]":     0xFFF1,   // TODO: verificar código correto
		"[PAUSE FOR]":     0xFFF0,   // TODO: verificar código correto
		"[WAIT FOR INPUT]": 0xFFEF,  // TODO: verificar código correto
		"[INIT TEXT BOX]": 0xFFEE,   // TODO: verificar código correto
	}
	
	for _, dialogue := range dialogues {
		fontHeight := int(dialogue.FontHeight)
		
		// Verificar se temos mapeamento para esta altura de fonte
		if glyphEncodeMap[fontHeight] == nil {
			return nil, fmt.Errorf("no glyph mapping found for font height %d in dialogue %d", fontHeight, dialogue.ID)
		}
		
		var encodedText []uint16
		originalText := dialogue.Text
		
		// Processar o texto caractere por caractere e tag por tag
		i := 0
		for i < len(originalText) {
			// Verificar se é uma tag especial
			if originalText[i] == '[' {
				tagProcessed := false
				
				// Verificar tags especiais conhecidas
				for tag, code := range specialTagMap {
					if i+len(tag) <= len(originalText) && originalText[i:i+len(tag)] == tag {
						encodedText = append(encodedText, code)
						i += len(tag)
						tagProcessed = true
						break
					}
				}
				
				if tagProcessed {
					continue
				}
				
				// Verificar se é um byte sem mapeamento [XXXX]
				if i+6 <= len(originalText) {
					possibleUnmapped := originalText[i:i+6]
					if unmappedByteRegex.MatchString(possibleUnmapped) {
						// Pular bytes sem mapeamento (não incluir no encode)
						fmt.Printf("Warning: Skipping unmapped byte %s in dialogue %d\n", possibleUnmapped, dialogue.ID)
						i += 6
						continue
					}
				}
			}
			
			// Processar caractere normal
			if i < len(originalText) {
				char := rune(originalText[i])
				
				// Pular quebras de linha
				if char == '\n' {
					i++
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
		
		recodedDialogue := RecodedDialogue{
			ID:           dialogue.ID,
			Type:         dialogue.Type,
			FontHeight:   uint16(dialogue.FontHeight),
			OriginalText: dialogue.Text,
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

// buildWFMFile constructs a complete WFM file from the processed data
func (e *WFMFileEncoder) buildWFMFile(glyphMap map[int]map[rune]Glyph, encodeValueMap map[uint16]GlyphEncodeInfo, encodeOrder []uint16, recodedDialogues []RecodedDialogue) (*WFMFile, error) {
	// Criar lista ordenada de glifos usando encodeOrder
	var glyphs []Glyph
	for _, encodeValue := range encodeOrder {
		glyphInfo := encodeValueMap[encodeValue]
		glyphs = append(glyphs, glyphInfo.Glyph)
	}
	
	// Converter diálogos recodificados para formato WFM
	var dialogues []Dialogue
	for _, recodedDialogue := range recodedDialogues {
		// Converter os valores uint16 para bytes (little endian)
		var dialogueData []byte
		for _, value := range recodedDialogue.EncodedText {
			// Adicionar valor em little endian (2 bytes)
			dialogueData = append(dialogueData, byte(value&0xFF))         // byte baixo
			dialogueData = append(dialogueData, byte((value>>8)&0xFF))    // byte alto
		}
		
		// Adicionar terminador
		dialogueData = append(dialogueData, byte(TERMINATOR_1&0xFF))      // 0xFE
		dialogueData = append(dialogueData, byte((TERMINATOR_1>>8)&0xFF)) // 0xFF
		
		dialogue := Dialogue{
			Data: dialogueData,
		}
		dialogues = append(dialogues, dialogue)
	}
	
	// Calcular ponteiros dos glifos (relativos ao início da tabela de ponteiros dos glifos)
	var glyphPointerTable []uint16
	currentGlyphOffset := uint16(len(glyphs) * 2) // Tamanho da tabela de ponteiros
	
	for _, glyph := range glyphs {
		glyphPointerTable = append(glyphPointerTable, currentGlyphOffset)
		// Cada glyph tem: 2+2+2+2 = 8 bytes de atributos + tamanho da imagem
		glyphSize := 8 + len(glyph.GlyphImage)
		currentGlyphOffset += uint16(glyphSize)
	}
	
	// Calcular ponteiros dos diálogos (relativos ao início da tabela de ponteiros dos diálogos)
	var dialoguePointerTable []uint16
	currentDialogueOffset := uint16(len(dialogues) * 2) // Tamanho da tabela de ponteiros
	
	for _, dialogue := range dialogues {
		dialoguePointerTable = append(dialoguePointerTable, currentDialogueOffset)
		currentDialogueOffset += uint16(len(dialogue.Data))
	}
	
	// Calcular posição da tabela de ponteiros dos diálogos
	headerSize := uint32(4 + 4 + 4 + 2 + 2 + 128) // Magic + Padding + DialoguePointerTable + TotalDialogues + TotalGlyphs + Reserved
	glyphTableSize := uint32(len(glyphPointerTable) * 2)
	totalGlyphsSize := uint32(0)
	for _, glyph := range glyphs {
		totalGlyphsSize += uint32(8 + len(glyph.GlyphImage))
	}
	
	dialoguePointerTableOffset := headerSize + glyphTableSize + totalGlyphsSize
	
	// Criar header
	header := WFMHeader{
		Magic:                [4]byte{'W', 'F', 'M', '3'},
		Padding:              0,
		DialoguePointerTable: dialoguePointerTableOffset,
		TotalDialogues:       uint16(len(dialogues)),
		TotalGlyphs:          uint16(len(glyphs)),
		Reserved:             [128]byte{}, // Zerado
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
	}
	
	// Escrever tabela de ponteiros dos diálogos
	for _, pointer := range wfm.DialoguePointerTable {
		err = binary.Write(file, binary.LittleEndian, pointer)
		if err != nil {
			return fmt.Errorf("failed to write dialogue pointer: %w", err)
		}
	}
	
	// Escrever diálogos
	for _, dialogue := range wfm.Dialogues {
		_, err = file.Write(dialogue.Data)
		if err != nil {
			return fmt.Errorf("failed to write dialogue data: %w", err)
		}
	}
	
	return nil
}

// loadGlyphsForCharacters loads and converts glyphs for the given characters from the fonts directory
func (e *WFMFileEncoder) loadGlyphsForCharacters(characters []rune) ([]Glyph, error) {
	var glyphs []Glyph
	fontHeight := 16 // Default font height, pode ser configurável no futuro
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
