// Package common provides shared utilities and data structures for PSX tile processing.
// This file defines tiles, palettes, and pixel processing functionality.
package common

import (
	"fmt"
	"image"
	"image/color"
)

// PSX tile and pixel processing constants
const (
	// BitsPerPixel4bpp defines 4 bits per pixel for PSX textures
	BitsPerPixel4bpp = 4
	
	// PixelsPerByte4bpp defines how many pixels fit in a byte for 4bpp format
	PixelsPerByte4bpp = 2
	
	// MaxPaletteSize4bpp defines maximum palette entries for 4bpp
	MaxPaletteSize4bpp = 16
	
	// PSXColorMask defines the 15-bit color mask for PSX colors
	PSXColorMask = 0x7FFF
)

// PSXColor represents a 15-bit PSX color value
type PSXColor uint16

// ToRGBA converts a PSX 15-bit color to standard RGBA format
func (c PSXColor) ToRGBA() color.RGBA {
	psxColor := uint16(c)
	
	// Extract RGB components from 15-bit PSX format (0BBBBBGGGGGRRRRR)
	r := uint8((psxColor & 0x1F) << 3)        // Red: bits 0-4
	g := uint8(((psxColor >> 5) & 0x1F) << 3) // Green: bits 5-9
	b := uint8(((psxColor >> 10) & 0x1F) << 3) // Blue: bits 10-14
	
	// Full opacity for visible colors, transparent for color 0
	var a uint8 = 255
	if psxColor == 0 {
		a = 0 // Transparent
	}
	
	return color.RGBA{R: r, G: g, B: b, A: a}
}

// FromRGBA creates a PSXColor from RGBA values
func PSXColorFromRGBA(r, g, b, a uint8) PSXColor {
	if a == 0 {
		return PSXColor(0) // Transparent
	}
	
	// Convert 8-bit RGB to 5-bit PSX format
	r5 := (r >> 3) & 0x1F
	g5 := (g >> 3) & 0x1F
	b5 := (b >> 3) & 0x1F
	
	return PSXColor(uint16(r5) | (uint16(g5) << 5) | (uint16(b5) << 10))
}

// PSXPalette represents a color palette for PSX graphics
type PSXPalette [MaxPaletteSize4bpp]PSXColor

// NewPSXPalette creates a new PSX palette from uint16 values
func NewPSXPalette(colors [MaxPaletteSize4bpp]uint16) PSXPalette {
	var palette PSXPalette
	for i, color := range colors {
		palette[i] = PSXColor(color)
	}
	return palette
}

// GetColor returns the RGBA color for a given palette index
func (p PSXPalette) GetColor(index uint8) color.RGBA {
	if index >= MaxPaletteSize4bpp {
		return color.RGBA{} // Transparent for invalid indices
	}
	return p[index].ToRGBA()
}

// FindClosestColor finds the closest palette index for a given RGBA color
func (p PSXPalette) FindClosestColor(c color.RGBA) uint8 {
	targetPSX := PSXColorFromRGBA(c.R, c.G, c.B, c.A)
	
	// Handle transparency
	if c.A == 0 {
		return 0 // Assume index 0 is transparent
	}
	
	bestIndex := uint8(0)
	bestDistance := uint32(0xFFFFFFFF)
	
	for i, paletteColor := range p {
		distance := colorDistance(targetPSX, paletteColor)
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = uint8(i)
		}
	}
	
	return bestIndex
}

// colorDistance calculates the distance between two PSX colors
func colorDistance(c1, c2 PSXColor) uint32 {
	rgba1 := c1.ToRGBA()
	rgba2 := c2.ToRGBA()
	
	dr := int32(rgba1.R) - int32(rgba2.R)
	dg := int32(rgba1.G) - int32(rgba2.G)
	db := int32(rgba1.B) - int32(rgba2.B)
	
	return uint32(dr*dr + dg*dg + db*db)
}

// PSXTile represents a tile in PSX 4bpp linear little endian format
type PSXTile struct {
	Width    int        // Tile width in pixels
	Height   int        // Tile height in pixels
	Data     []byte     // Raw 4bpp pixel data
	Palette  PSXPalette // Color palette for this tile
}

// NewPSXTile creates a new PSX tile with specified dimensions
func NewPSXTile(width, height int, palette PSXPalette) *PSXTile {
	// Calculate required bytes (2 pixels per byte for 4bpp)
	bytesPerRow := (width + 1) / 2
	totalBytes := bytesPerRow * height
	
	return &PSXTile{
		Width:   width,
		Height:  height,
		Data:    make([]byte, totalBytes),
		Palette: palette,
	}
}

// GetPixel returns the palette index for a pixel at coordinates (x, y)
func (t *PSXTile) GetPixel(x, y int) (uint8, error) {
	if x >= t.Width || y >= t.Height || x < 0 || y < 0 {
		return 0, fmt.Errorf("pixel coordinates (%d, %d) out of bounds", x, y)
	}
	
	pixelIndex := y*t.Width + x
	byteIndex := pixelIndex / PixelsPerByte4bpp
	
	if byteIndex >= len(t.Data) {
		return 0, fmt.Errorf("byte index %d out of bounds", byteIndex)
	}
	
	if pixelIndex%2 == 0 {
		// Even pixel: lower 4 bits (little endian)
		return t.Data[byteIndex] & 0x0F, nil
	} else {
		// Odd pixel: upper 4 bits (little endian)
		return (t.Data[byteIndex] & 0xF0) >> 4, nil
	}
}

// SetPixel sets the palette index for a pixel at coordinates (x, y)
func (t *PSXTile) SetPixel(x, y int, paletteIndex uint8) error {
	if x >= t.Width || y >= t.Height || x < 0 || y < 0 {
		return fmt.Errorf("pixel coordinates (%d, %d) out of bounds", x, y)
	}
	
	if paletteIndex >= MaxPaletteSize4bpp {
		return fmt.Errorf("palette index %d out of range (max %d)", paletteIndex, MaxPaletteSize4bpp-1)
	}
	
	pixelIndex := y*t.Width + x
	byteIndex := pixelIndex / PixelsPerByte4bpp
	
	if byteIndex >= len(t.Data) {
		return fmt.Errorf("byte index %d out of bounds", byteIndex)
	}
	
	if pixelIndex%2 == 0 {
		// Even pixel: lower 4 bits (little endian)
		t.Data[byteIndex] = (t.Data[byteIndex] & 0xF0) | (paletteIndex & 0x0F)
	} else {
		// Odd pixel: upper 4 bits (little endian)
		t.Data[byteIndex] = (t.Data[byteIndex] & 0x0F) | ((paletteIndex & 0x0F) << 4)
	}
	
	return nil
}

// ToImage converts the PSX tile to a standard Go image
func (t *PSXTile) ToImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, t.Width, t.Height))
	
	for y := 0; y < t.Height; y++ {
		for x := 0; x < t.Width; x++ {
			paletteIndex, err := t.GetPixel(x, y)
			if err != nil {
				continue // Skip invalid pixels
			}
			
			color := t.Palette.GetColor(paletteIndex)
			img.Set(x, y, color)
		}
	}
	
	return img
}

// FromImage creates a PSX tile from a standard Go image using the specified palette
func (t *PSXTile) FromImage(img image.Image) error {
	bounds := img.Bounds()
	if bounds.Dx() != t.Width || bounds.Dy() != t.Height {
		return fmt.Errorf("image dimensions (%dx%d) don't match tile dimensions (%dx%d)",
			bounds.Dx(), bounds.Dy(), t.Width, t.Height)
	}
	
	for y := 0; y < t.Height; y++ {
		for x := 0; x < t.Width; x++ {
			imgColor := color.RGBAModel.Convert(img.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.RGBA)
			paletteIndex := t.Palette.FindClosestColor(imgColor)
			
			if err := t.SetPixel(x, y, paletteIndex); err != nil {
				return fmt.Errorf("failed to set pixel at (%d, %d): %w", x, y, err)
			}
		}
	}
	
	return nil
}

// TileConverter interface defines methods for converting between different tile formats
type TileConverter interface {
	// ConvertTo4bppLinearLE converts an image to 4bpp linear little endian format
	ConvertTo4bppLinearLE(img image.Image, palette PSXPalette) (*PSXTile, error)
	
	// ConvertFromTile converts a PSX tile to a standard image
	ConvertFromTile(tile *PSXTile) (*image.RGBA, error)
}

// PSXTileProcessor implements the TileConverter interface
type PSXTileProcessor struct{}

// NewPSXTileProcessor creates a new PSX tile processor
func NewPSXTileProcessor() *PSXTileProcessor {
	return &PSXTileProcessor{}
}

// ConvertTo4bppLinearLE converts an image to 4bpp linear little endian format
func (p *PSXTileProcessor) ConvertTo4bppLinearLE(img image.Image, palette PSXPalette) (*PSXTile, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	
	tile := NewPSXTile(width, height, palette)
	
	if err := tile.FromImage(img); err != nil {
		return nil, fmt.Errorf("failed to convert image to tile: %w", err)
	}
	
	return tile, nil
}

// ConvertFromTile converts a PSX tile to a standard image
func (p *PSXTileProcessor) ConvertFromTile(tile *PSXTile) (*image.RGBA, error) {
	if tile == nil {
		return nil, fmt.Errorf("tile is nil")
	}
	
	return tile.ToImage(), nil
}
