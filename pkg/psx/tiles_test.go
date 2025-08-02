// Package psx provides tests for PSX tile processing functionality.
package psx

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
		name       string
		r, g, b, a uint8
		expected   PSXColor
	}{
		{
			name: "transparent color",
			r:    255, g: 255, b: 255, a: 0,
			expected: PSXColor(0),
		},
		{
			name: "white color",
			r:    248, g: 248, b: 248, a: 255,
			expected: PSXColor(0x7FFF),
		},
		{
			name: "red color",
			r:    248, g: 0, b: 0, a: 255,
			expected: PSXColor(0x001F),
		},
		{
			name: "green color",
			r:    0, g: 248, b: 0, a: 255,
			expected: PSXColor(0x03E0),
		},
		{
			name: "blue color",
			r:    0, g: 0, b: 248, a: 255,
			expected: PSXColor(0x7C00),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PSXColorFromRGBA(tt.r, tt.g, tt.b, tt.a)
			if result != tt.expected {
				t.Errorf("PSXColorFromRGBA(%d, %d, %d, %d) = %d, want %d",
					tt.r, tt.g, tt.b, tt.a, result, tt.expected)
			}
		})
	}
}

func TestPSXTile_GetSetPixel(t *testing.T) {
	palette := NewPSXPalette([MaxPaletteSize4bpp]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF, // Basic colors
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	tile := NewPSXTile(4, 4, palette)

	// Test setting and getting pixels
	testCases := []struct {
		x, y  int
		index uint8
	}{
		{0, 0, 0},
		{1, 0, 1},
		{2, 0, 2},
		{3, 0, 3},
		{0, 1, 4},
		{1, 1, 5},
	}

	for _, tc := range testCases {
		// Set pixel
		err := tile.SetPixel(tc.x, tc.y, tc.index)
		if err != nil {
			t.Errorf("SetPixel(%d, %d, %d) failed: %v", tc.x, tc.y, tc.index, err)
			continue
		}

		// Get pixel
		got, err := tile.GetPixel(tc.x, tc.y)
		if err != nil {
			t.Errorf("GetPixel(%d, %d) failed: %v", tc.x, tc.y, err)
			continue
		}

		if got != tc.index {
			t.Errorf("GetPixel(%d, %d) = %d, want %d", tc.x, tc.y, got, tc.index)
		}
	}

	// Test bounds checking
	err := tile.SetPixel(4, 0, 0)
	if err == nil {
		t.Error("SetPixel should fail for out-of-bounds coordinates")
	}

	_, err = tile.GetPixel(0, 4)
	if err == nil {
		t.Error("GetPixel should fail for out-of-bounds coordinates")
	}
}

func TestPSXTile_ToFromImage(t *testing.T) {
	palette := NewPSXPalette([MaxPaletteSize4bpp]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF, // Basic colors
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{0, 0, 0, 0})     // Transparent (index 0)
	img.Set(1, 0, color.RGBA{248, 0, 0, 255}) // Red (index 1)
	img.Set(0, 1, color.RGBA{0, 248, 0, 255}) // Green (index 2)
	img.Set(1, 1, color.RGBA{0, 0, 248, 255}) // Blue (index 3)

	// Create tile and load from image
	tile := NewPSXTile(2, 2, palette)
	err := tile.FromImage(img)
	if err != nil {
		t.Fatalf("FromImage failed: %v", err)
	}

	// Convert back to image
	resultImg := tile.ToImage()

	// Verify the colors match (approximately, due to PSX color precision)
	testCases := []struct {
		x, y     int
		expected color.RGBA
	}{
		{0, 0, color.RGBA{0, 0, 0, 0}},
		{1, 0, color.RGBA{248, 0, 0, 255}},
		{0, 1, color.RGBA{0, 248, 0, 255}},
		{1, 1, color.RGBA{0, 0, 248, 255}},
	}

	for _, tc := range testCases {
		got := color.RGBAModel.Convert(resultImg.At(tc.x, tc.y)).(color.RGBA)
		if got != tc.expected {
			t.Errorf("Image color at (%d, %d) = %v, want %v", tc.x, tc.y, got, tc.expected)
		}
	}
}

func TestPSXTileProcessor(t *testing.T) {
	processor := NewPSXTileProcessor()

	palette := NewPSXPalette([MaxPaletteSize4bpp]uint16{
		0x0000, 0x001F, 0x03E0, 0x7C00, 0x7FFF, // Basic colors
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	})

	// Create a test image
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{0, 0, 0, 0})     // Transparent
	img.Set(1, 0, color.RGBA{248, 0, 0, 255}) // Red
	img.Set(0, 1, color.RGBA{0, 248, 0, 255}) // Green
	img.Set(1, 1, color.RGBA{0, 0, 248, 255}) // Blue

	// Convert to PSX tile
	tile, err := processor.ConvertTo4bppLinearLE(img, palette)
	if err != nil {
		t.Fatalf("ConvertTo4bppLinearLE failed: %v", err)
	}

	if tile.Width != 2 || tile.Height != 2 {
		t.Errorf("Tile dimensions = %dx%d, want 2x2", tile.Width, tile.Height)
	}

	// Convert back to image
	resultImg, err := processor.ConvertFromTile(tile)
	if err != nil {
		t.Fatalf("ConvertFromTile failed: %v", err)
	}

	if bounds := resultImg.Bounds(); bounds.Dx() != 2 || bounds.Dy() != 2 {
		t.Errorf("Result image dimensions = %dx%d, want 2x2", bounds.Dx(), bounds.Dy())
	}

	// Test nil tile
	_, err = processor.ConvertFromTile(nil)
	if err == nil {
		t.Error("ConvertFromTile should fail with nil tile")
	}
}
