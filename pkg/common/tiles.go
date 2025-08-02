// Package common provides shared utilities and interfaces for tile processing.
// This file defines generic interfaces that can be implemented by different platforms.
package common

import (
	"image"
)

// TileConverter interface defines methods for converting between different tile formats
type TileConverter interface {
	// ConvertTo4bppLinearLE converts an image to 4bpp linear little endian format
	ConvertTo4bppLinearLE(img image.Image, palette interface{}) (interface{}, error)
	
	// ConvertFromTile converts a tile to a standard image
	ConvertFromTile(tile interface{}) (*image.RGBA, error)
}
