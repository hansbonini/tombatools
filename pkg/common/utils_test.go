// Package common provides tests for utility functions
package common

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestIsValidWFMFile_Valid(t *testing.T) {
	magic := []byte("WFM3")
	reader := bytes.NewReader(magic)
	err := IsValidWFMFile(reader)
	if err != nil {
		t.Errorf("IsValidWFMFile() failed with valid header: %v", err)
	}
}

func TestIsValidWFMFile_Invalid(t *testing.T) {
	testCases := []struct {
		name  string
		magic [4]byte
	}{
		{"empty", [4]byte{0, 0, 0, 0}},
		{"wrong format", [4]byte{'A', 'B', 'C', 'D'}},
		{"partial match", [4]byte{'W', 'F', 'M', '2'}},
		{"case sensitive", [4]byte{'w', 'f', 'm', '3'}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.magic[:])
			err := IsValidWFMFile(reader)
			if err == nil {
				t.Errorf("IsValidWFMFile() should fail with invalid header %v", tc.magic)
			}

			expectedMsg := "invalid WFM header"
			if !bytes.Contains([]byte(err.Error()), []byte(expectedMsg)) {
				t.Errorf("Error message %q should contain %q", err.Error(), expectedMsg)
			}
		})
	}
}

func TestReadUint16LE(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected uint16
		hasError bool
	}{
		{"normal value", []byte{0x34, 0x12}, 0x1234, false},
		{"zero value", []byte{0x00, 0x00}, 0x0000, false},
		{"max value", []byte{0xFF, 0xFF}, 0xFFFF, false},
		{"incomplete data", []byte{0x34}, 0, true},
		{"empty data", []byte{}, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			result, err := ReadUint16LE(reader)

			if tc.hasError {
				if err == nil {
					t.Errorf("ReadUint16LE() should fail with data %v", tc.data)
				}
			} else {
				if err != nil {
					t.Errorf("ReadUint16LE() failed: %v", err)
				}
				if result != tc.expected {
					t.Errorf("ReadUint16LE() = 0x%04X, want 0x%04X", result, tc.expected)
				}
			}
		})
	}
}

func TestReadUint32LE(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		expected uint32
		hasError bool
	}{
		{"normal value", []byte{0x78, 0x56, 0x34, 0x12}, 0x12345678, false},
		{"zero value", []byte{0x00, 0x00, 0x00, 0x00}, 0x00000000, false},
		{"max value", []byte{0xFF, 0xFF, 0xFF, 0xFF}, 0xFFFFFFFF, false},
		{"incomplete data", []byte{0x78, 0x56, 0x34}, 0, true},
		{"empty data", []byte{}, 0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			result, err := ReadUint32LE(reader)

			if tc.hasError {
				if err == nil {
					t.Errorf("ReadUint32LE() should fail with data %v", tc.data)
				}
			} else {
				if err != nil {
					t.Errorf("ReadUint32LE() failed: %v", err)
				}
				if result != tc.expected {
					t.Errorf("ReadUint32LE() = 0x%08X, want 0x%08X", result, tc.expected)
				}
			}
		})
	}
}

func TestReadBytes(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		count    int
		expected []byte
		hasError bool
	}{
		{"normal read", []byte{0x01, 0x02, 0x03, 0x04}, 3, []byte{0x01, 0x02, 0x03}, false},
		{"exact read", []byte{0x01, 0x02}, 2, []byte{0x01, 0x02}, false},
		{"zero read", []byte{0x01, 0x02}, 0, []byte{}, false},
		{"insufficient data", []byte{0x01, 0x02}, 3, nil, true},
		{"empty source", []byte{}, 1, nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			result, err := ReadBytes(reader, tc.count)

			if tc.hasError {
				if err == nil {
					t.Errorf("ReadBytes() should fail when requesting %d bytes from %v", tc.count, tc.data)
				}
			} else {
				if err != nil {
					t.Errorf("ReadBytes() failed: %v", err)
				}
				if len(result) != len(tc.expected) {
					t.Errorf("ReadBytes() returned %d bytes, want %d", len(result), len(tc.expected))
				} else {
					for i, expected := range tc.expected {
						if result[i] != expected {
							t.Errorf("ReadBytes()[%d] = 0x%02X, want 0x%02X", i, result[i], expected)
						}
					}
				}
			}
		})
	}
}

func TestSkipBytes(t *testing.T) {
	testCases := []struct {
		name     string
		data     []byte
		skip     int
		hasError bool
	}{
		{"normal skip", []byte{0x01, 0x02, 0x03, 0x04, 0x05}, 3, false},
		{"skip all", []byte{0x01, 0x02}, 2, false},
		{"skip zero", []byte{0x01, 0x02}, 0, false},
		{"skip more than available", []byte{0x01, 0x02}, 5, true},
		{"skip from empty", []byte{}, 1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bytes.NewReader(tc.data)
			err := SkipBytes(reader, tc.skip)

			if tc.hasError {
				if err == nil {
					t.Errorf("SkipBytes() should fail when skipping %d bytes from %v", tc.skip, tc.data)
				}
			} else {
				if err != nil {
					t.Errorf("SkipBytes() failed: %v", err)
				}

				// Verify that the correct number of bytes were skipped
				remaining, readErr := io.ReadAll(reader)
				if readErr != nil {
					t.Errorf("Failed to read remaining bytes: %v", readErr)
				}

				expectedRemaining := len(tc.data) - tc.skip
				if len(remaining) != expectedRemaining {
					t.Errorf("After skipping %d bytes, %d bytes remain, want %d", tc.skip, len(remaining), expectedRemaining)
				}
			}
		})
	}
}

// Test reading from binary data created with the same endianness
func TestReadFunctions_BinaryCompatibility(t *testing.T) {
	var buffer bytes.Buffer

	// Write test data using binary.Write
	test16 := uint16(0x1234)
	test32 := uint32(0x12345678)

	binary.Write(&buffer, binary.LittleEndian, test16)
	binary.Write(&buffer, binary.LittleEndian, test32)

	reader := bytes.NewReader(buffer.Bytes())

	// Read back using our functions
	read16, err := ReadUint16LE(reader)
	if err != nil {
		t.Fatalf("ReadUint16LE() failed: %v", err)
	}

	if read16 != test16 {
		t.Errorf("ReadUint16LE() = 0x%04X, want 0x%04X", read16, test16)
	}

	read32, err := ReadUint32LE(reader)
	if err != nil {
		t.Fatalf("ReadUint32LE() failed: %v", err)
	}

	if read32 != test32 {
		t.Errorf("ReadUint32LE() = 0x%08X, want 0x%08X", read32, test32)
	}
}
