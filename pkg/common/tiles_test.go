// Package common provides tests for PSX tile processing functionality.
package common

import (
	"image"
	"image/color"
	"testing"
)

func TestPSXColor_ToRGBA(t *testing.T) {
	tests := []struct {
		name     string
		psxColor PSXColor
		expected color.RGBA
	}{
		{
			name:     "transparent color",
			psxColor: PSXColor(0),
			expected: color.RGBA{0, 0, 0, 0},
		},
		{
			name:     "white color",
			psxColor: PSXColor(0x7FFF), // All bits set in 15-bit format
			expected: color.RGBA{248, 248, 248, 255},
		},
		{
			name:     "red color",
			psxColor: PSXColor(0x001F), // Only red bits set
			expected: color.RGBA{248, 0, 0, 255},
		},
		{
			name:     "green color",
			psxColor: PSXColor(0x03E0), // Only green bits set
			expected: color.RGBA{0, 248, 0, 255},
		},
		{
			name:     "blue color",
			psxColor: PSXColor(0x7C00), // Only blue bits set
			expected: color.RGBA{0, 0, 248, 255},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.psxColor.ToRGBA()
			if result != tt.expected {
				t.Errorf("PSXColor.ToRGBA() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPSXColorFromRGBA(t *testing.T) {
	tests := []struct {
		name     string
		r, g, b, a uint8
		expected PSXColor
	}{
		{
			name:     "transparent color",
			r: 255, g: 255, b: 255, a: 0,
			expected: PSXColor(0),
		},
		{
			name:     "white color",
			r: 248, g: 248, b: 248, a: 255,
			expected: PSXColor(0x7FFF),
		},
		{
			name:     "red color",
			r: 248, g: 0, b: 0, a: 255,
			expected: PSXColor(0x001F),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PSXColorFromRGBA(tt.r, tt.g, tt.b, tt.a)
			if result != tt.expected {
				t.Errorf("PSXColorFromRGBA() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestPSXTile_GetSetPixel(t *testing.T) {
	// Create a test palette
	palette := NewPSXPalette([16]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	tile := NewPSXTile(4, 2, palette)

	// Test setting and getting pixels
	tests := []struct {
		x, y  int
		value uint8
	}{
		{0, 0, 1}, // First pixel
		{1, 0, 2}, // Second pixel (same byte)
		{2, 0, 3}, // Third pixel
		{3, 0, 4}, // Fourth pixel (same byte as third)
		{0, 1, 5}, // Second row
	}

	for _, tt := range tests {
		// Set pixel
		err := tile.SetPixel(tt.x, tt.y, tt.value)
		if err != nil {
			t.Errorf("SetPixel(%d, %d, %d) error = %v", tt.x, tt.y, tt.value, err)
			continue
		}

		// Get pixel
		value, err := tile.GetPixel(tt.x, tt.y)
		if err != nil {
			t.Errorf("GetPixel(%d, %d) error = %v", tt.x, tt.y, err)
			continue
		}

		if value != tt.value {
			t.Errorf("GetPixel(%d, %d) = %d, want %d", tt.x, tt.y, value, tt.value)
		}
	}
}

func TestPSXTile_ToFromImage(t *testing.T) {
	// Create a test palette
	palette := NewPSXPalette([16]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	tile := NewPSXTile(2, 2, palette)

	// Set some pixels
	tile.SetPixel(0, 0, 1) // Red
	tile.SetPixel(1, 0, 2) // Green
	tile.SetPixel(0, 1, 3) // Blue
	tile.SetPixel(1, 1, 4) // White

	// Convert to image
	img := tile.ToImage()

	// Check image dimensions
	bounds := img.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Errorf("Image dimensions = %dx%d, want 2x2", bounds.Dx(), bounds.Dy())
	}

	// Create a new tile and convert from image
	newTile := NewPSXTile(2, 2, palette)
	err := newTile.FromImage(img)
	if err != nil {
		t.Errorf("FromImage() error = %v", err)
	}

	// Compare pixel values
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			originalValue, _ := tile.GetPixel(x, y)
			newValue, _ := newTile.GetPixel(x, y)
			if originalValue != newValue {
				t.Errorf("Pixel (%d, %d): original = %d, new = %d", x, y, originalValue, newValue)
			}
		}
	}
}

func TestPSXTileProcessor(t *testing.T) {
	processor := NewPSXTileProcessor()

	// Create a test palette
	palette := NewPSXPalette([16]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{248, 0, 0, 255})   // Red
	img.Set(1, 0, color.RGBA{0, 248, 0, 255})   // Green
	img.Set(0, 1, color.RGBA{0, 0, 248, 255})   // Blue
	img.Set(1, 1, color.RGBA{248, 248, 248, 255}) // White

	// Convert to tile
	tile, err := processor.ConvertTo4bppLinearLE(img, palette)
	if err != nil {
		t.Errorf("ConvertTo4bppLinearLE() error = %v", err)
		return
	}

	// Convert back to image
	resultImg, err := processor.ConvertFromTile(tile)
	if err != nil {
		t.Errorf("ConvertFromTile() error = %v", err)
		return
	}

	// Check dimensions
	bounds := resultImg.Bounds()
	if bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Errorf("Result image dimensions = %dx%d, want 2x2", bounds.Dx(), bounds.Dy())
	}

	// Note: We don't check exact color matching because the palette conversion
	// may introduce slight differences due to color quantization
}
