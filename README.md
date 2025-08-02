# TombaTools

A collection of utilities for extracting and modifying game files from **Tomba!** (Ore no Tomba) for PlayStation.

## Features

- **WFM Font File Processing**: Extract and modify font glyphs and dialogue text
- **4bpp PSX Graphics**: Native support for PlayStation 4bpp linear little endian format
- **YAML Export/Import**: Human-readable dialogue editing
- **PNG Glyph Export**: Individual character extraction as PNG images

## Installation

### Prerequisites
- Go 1.19 or later

### Build from Source
```bash
git clone https://github.com/hansbonini/tombatools.git
cd tombatools
go build -o tombatools.exe
```

## Usage

### WFM Font Files

#### Extract (Decode)
Extract glyphs and dialogues from a WFM file:
```bash
tombatools wfm decode CFNT999H.WFM ./output/
```

This creates:
- `glyphs/` - Individual PNG files for each character
- `dialogues.yaml` - Editable dialogue text in YAML format

#### Create (Encode)
Create a new WFM file from edited dialogues:
```bash
tombatools wfm encode dialogues.yaml CFNT999H_modified.WFM
```

#### Verbose Output
Use `-v` flag for detailed processing information:
```bash
tombatools wfm decode -v CFNT999H.WFM ./output/
tombatools wfm encode -v dialogues.yaml output.WFM
```

### Example Workflow

1. **Extract original WFM file:**
   ```bash
   tombatools wfm decode CFNT999H.WFM ./extracted/
   ```

2. **Edit dialogues:**
   - Open `extracted/dialogues.yaml` in a text editor
   - Modify the `text` fields under dialogue entries
   - Save the file

3. **Create modified WFM:**
   ```bash
   tombatools wfm encode extracted/dialogues.yaml CFNT999H_translated.WFM
   ```

## File Formats

### WFM Files
WFM (WFM3) files contain:
- **Font glyphs**: 4bpp PSX format character graphics
- **Dialogue data**: Text with control codes for display
- **Palettes**: Color lookup tables (CLUT) for rendering

### Supported Dialogue Control Codes
- `[INIT TEXT BOX]` - Initialize dialogue box with dimensions
- `[NEWLINE]` - Line break
- `[WAIT FOR INPUT]` - Pause for user input
- `[HALT]` - End dialogue
- `[CHANGE COLOR TO]` - Change text color
- And more...

## Technical Details

### PSX Graphics Support
- Native 4bpp linear little endian processing
- Automatic palette selection (Dialogue/Event CLUT)
- PSX 15-bit color format conversion

### Font Heights
- **8px**: Menu and UI text (DialogueClut)
- **16px**: Standard dialogue text (DialogueClut) 
- **24px**: Event and emphasis text (EventClut)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Tomba! game by Whoopee Camp
- PlayStation technical documentation community
- Go image processing libraries

---

**Note**: This tool is for educational and preservation purposes. Respect copyright laws and game developer rights.
