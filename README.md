# TombaTools

[![CI/CD](https://github.com/hansbonini/tombatools/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/hansbonini/tombatools/actions/workflows/ci-cd.yml)
[![Security](https://github.com/hansbonini/tombatools/actions/workflows/security.yml/badge.svg)](https://github.com/hansbonini/tombatools/actions/workflows/security.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hansbonini/tombatools)](https://goreportcard.com/report/github.com/hansbonini/tombatools)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A collection of utilities for extracting and modifying game files from **Tomba!** (Ore no Tomba) for PlayStation.

Currently supports WFM font files and GAM game data files with full extraction, modification, and recreation capabilities.

## Features

- **WFM Font File Processing**: Extract and modify font glyphs and dialogue text
- **GAM File Processing**: Extract and compress game data using custom LZ algorithm
- **4bpp PSX Graphics**: Native support for PlayStation 4bpp linear little endian format
- **YAML Export/Import**: Human-readable dialogue editing
- **PNG Glyph Export**: Individual character extraction as PNG images

## Installation

### Download Pre-built Binaries

Download the latest release for your platform from the [Releases page](https://github.com/hansbonini/tombatools/releases):

- **Windows**: `tombatools_windows_amd64.zip`
- **Linux**: `tombatools_linux_amd64.tar.gz` or `tombatools_linux_arm64.tar.gz`
- **macOS**: `tombatools_darwin_amd64.tar.gz` (Intel) or `tombatools_darwin_arm64.tar.gz` (Apple Silicon)

### Build from Source

#### Prerequisites
- Go 1.21 or later

#### Clone and Build
```bash
git clone https://github.com/hansbonini/tombatools.git
cd tombatools
make build
# or
go build -o tombatools
```

#### Development Tools
```bash
# Install development dependencies
make install

# Install linting and security tools
make tools
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

### GAM Files

GAM files contain compressed game data using a custom LZ compression algorithm.

#### Extract (Unpack)
Extract and decompress data from a GAM file:
```bash
tombatools gam unpack GAME.GAM data.UNGAM
```

This creates a decompressed `.UNGAM` file containing the raw game data.

#### Create (Pack)
Compress data back into a GAM file:
```bash
tombatools gam pack data.UNGAM GAME_modified.GAM
```

#### Verbose Output
Use `-v` flag for detailed compression/decompression information:
```bash
tombatools gam unpack -v GAME.GAM data.UNGAM
tombatools gam pack -v data.UNGAM output.GAM
```

## Development

### Available Make Targets

```bash
make help      # Show available commands
make build     # Build for current platform
make test      # Run tests with coverage
make lint      # Run code linters
make release   # Build for all platforms
make security  # Run security scans
make clean     # Clean build artifacts
```

### Running Tests

```bash
# Run all tests
make test

# Run specific package tests
go test ./pkg/...

# Run with verbose output
go test -v ./...
```

### Code Quality

This project uses:
- **golangci-lint** for code linting
- **gosec** for security scanning  
- **GitHub Actions** for CI/CD
- **Dependabot** for dependency updates

### Example Workflow

#### WFM Files (Fonts and Dialogues)

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

#### GAM Files (Game Data)

1. **Extract original GAM file:**
   ```bash
   tombatools gam unpack GAME.GAM extracted_data.UNGAM
   ```

2. **Modify data:**
   - Edit the extracted `.UNGAM` file with a hex editor or custom tools
   - Make your desired changes to the game data

3. **Create modified GAM:**
   ```bash
   tombatools gam pack extracted_data.UNGAM GAME_modified.GAM
   ```

## File Formats

### WFM Files
WFM (WFM3) files contain:
- **Font glyphs**: 4bpp PSX format character graphics
- **Dialogue data**: Text with control codes for display
- **Palettes**: Color lookup tables (CLUT) for rendering

### GAM Files
GAM files contain:
- **8-byte header**: Magic "GAM" + padding + uncompressed size (little-endian)
- **Compressed data**: Game data compressed using custom LZ algorithm
- **LZ compression**: Bitmask-based algorithm with literal bytes and back-references

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
