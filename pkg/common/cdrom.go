// Package common provides common utilities for CD-ROM operations.
// This file contains functions for MSF conversion and CD-ROM related utilities.
package common

import "fmt"

// LBAToMSF converts LBA (Logical Block Address) to MSF (Minutes:Seconds:Frames) format
// LBA to MSF conversion: LBA + 150 (pregap)
func LBAToMSF(lba uint32) string {
	totalFrames := lba + 150

	minutes := totalFrames / (60 * 75)
	seconds := (totalFrames % (60 * 75)) / 75
	frames := totalFrames % 75

	return fmt.Sprintf("%02d:%02d:%02d", minutes, seconds, frames)
}

// GetSizeInSectors calculates the number of sectors needed for a given size in bytes
func GetSizeInSectors(sizeBytes uint32) uint32 {
	const sectorSize = 2048
	return (sizeBytes + sectorSize - 1) / sectorSize
}

// CleanFileName removes version numbers from ISO9660 file names
func CleanFileName(fileName string) string {
	// Remove version numbers (e.g., "FILE.EXT;1" -> "FILE.EXT")
	if len(fileName) > 0 && fileName[len(fileName)-1] >= '0' && fileName[len(fileName)-1] <= '9' {
		if len(fileName) > 2 && fileName[len(fileName)-2] == ';' {
			return fileName[:len(fileName)-2]
		}
	}
	return fileName
}

// IsSpecialDirEntry checks if a directory entry is "." or ".."
func IsSpecialDirEntry(fileName string) bool {
	return fileName == "\x00" || fileName == "\x01"
}

// IsValidFileName checks if a filename contains only valid characters
func IsValidFileName(fileName string) bool {
	if len(fileName) == 0 || len(fileName) > 255 {
		return false
	}

	// Check for obvious binary data corruption
	if HasTooManyNullBytes(fileName) || HasControlCharacterSpam(fileName) {
		return false
	}

	validChars := 0
	for _, b := range []byte(fileName) {
		// Allow only specific safe characters for filenames
		if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') ||
			(b >= '0' && b <= '9') || b == '.' || b == '_' || b == '-' {
			validChars++
		} else if b == ' ' && validChars > 0 { // Allow spaces if not at start
			// Space is okay
		} else if b < 0x20 && (b == 0x00 || b == 0x01) {
			// Allow special directory entries
		} else if b >= 0x80 && b <= 0xFF {
			// High-bit characters might be valid in some encodings, but risky
			return false
		} else {
			// Other characters (punctuation, etc.)
			if b == '<' || b == '>' || b == ':' || b == '"' || b == '|' || b == '?' || b == '*' || b == '\\' || b == '/' {
				return false
			}
		}
	}

	// Must have at least one valid character
	return validChars > 0
}

// HasTooManyNullBytes detects if string has suspicious amount of null bytes
func HasTooManyNullBytes(s string) bool {
	if len(s) < 10 {
		return false
	}

	nullCount := 0
	for _, b := range []byte(s) {
		if b == 0x00 {
			nullCount++
		}
	}

	// If more than 20% are null bytes, likely corrupted
	return float64(nullCount)/float64(len(s)) > 0.2
}

// HasControlCharacterSpam detects patterns of repeated control characters
func HasControlCharacterSpam(s string) bool {
	if len(s) < 5 {
		return false
	}

	controlCount := 0
	for _, b := range []byte(s) {
		if b < 0x20 && b != 0x00 && b != 0x01 {
			controlCount++
		}
	}

	// If more than 30% are control characters, likely corrupted
	return float64(controlCount)/float64(len(s)) > 0.3
}

// ExtractLBAFromDirRecord extracts LBA from ISO9660 directory record
func ExtractLBAFromDirRecord(dirRecord []byte) uint32 {
	if len(dirRecord) < 6 {
		return 0
	}
	// LBA is at offset 2 (little-endian)
	return uint32(dirRecord[2]) |
		uint32(dirRecord[3])<<8 |
		uint32(dirRecord[4])<<16 |
		uint32(dirRecord[5])<<24
}

// ExtractSizeFromDirRecord extracts size from ISO9660 directory record
func ExtractSizeFromDirRecord(dirRecord []byte) uint32 {
	if len(dirRecord) < 14 {
		return 0
	}
	// Size is at offset 10 (little-endian)
	return uint32(dirRecord[10]) |
		uint32(dirRecord[11])<<8 |
		uint32(dirRecord[12])<<16 |
		uint32(dirRecord[13])<<24
}
