package common

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ValidateWFMHeader checks if the given bytes represent a valid WFM3 header
func ValidateWFMHeader(magic [4]byte) error {
	if string(magic[:]) != "WFM3" {
		return fmt.Errorf("invalid WFM header: expected 'WFM3', got '%s'", string(magic[:]))
	}
	return nil
}

// ReadUint16LE reads a uint16 in little-endian format
func ReadUint16LE(reader io.Reader) (uint16, error) {
	var value uint16
	err := binary.Read(reader, binary.LittleEndian, &value)
	return value, err
}

// ReadUint32LE reads a uint32 in little-endian format
func ReadUint32LE(reader io.Reader) (uint32, error) {
	var value uint32
	err := binary.Read(reader, binary.LittleEndian, &value)
	return value, err
}

// ReadBytes reads a specified number of bytes
func ReadBytes(reader io.Reader, count int) ([]byte, error) {
	buffer := make([]byte, count)
	n, err := io.ReadFull(reader, buffer)
	if err != nil {
		return nil, err
	}
	if n != count {
		return nil, fmt.Errorf("expected to read %d bytes, got %d", count, n)
	}
	return buffer, nil
}

// SkipBytes skips a specified number of bytes in the reader
func SkipBytes(reader io.Reader, count int) error {
	_, err := io.CopyN(io.Discard, reader, int64(count))
	return err
}
