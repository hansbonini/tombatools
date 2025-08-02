package common

import (
	"fmt"
	"math"
)

// SafeIntToUint16 safely converts int to uint16 with bounds checking
func SafeIntToUint16(value int) (uint16, error) {
	if value < 0 || value > math.MaxUint16 {
		return 0, fmt.Errorf("value %d out of range for uint16 (0-%d)", value, math.MaxUint16)
	}
	return uint16(value), nil
}

// SafeIntToUint32 safely converts int to uint32 with bounds checking
func SafeIntToUint32(value int) (uint32, error) {
	if value < 0 {
		return 0, fmt.Errorf("value %d is negative, cannot convert to uint32", value)
	}
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("value %d out of range for uint32 (0-%d)", value, math.MaxUint32)
	}
	return uint32(value), nil
}

// SafeUint32ToUint16 safely converts uint32 to uint16 with bounds checking
func SafeUint32ToUint16(value uint32) (uint16, error) {
	if value > math.MaxUint16 {
		return 0, fmt.Errorf("value %d out of range for uint16 (0-%d)", value, math.MaxUint16)
	}
	return uint16(value), nil
}

// SafeInt64ToUint32 safely converts int64 to uint32 with bounds checking
func SafeInt64ToUint32(value int64) (uint32, error) {
	if value < 0 {
		return 0, fmt.Errorf("value %d is negative, cannot convert to uint32", value)
	}
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("value %d out of range for uint32 (0-%d)", value, math.MaxUint32)
	}
	return uint32(value), nil
}

// SafeIntToUint8 safely converts int to uint8 with bounds checking
func SafeIntToUint8(value int) (uint8, error) {
	if value < 0 || value > math.MaxUint8 {
		return 0, fmt.Errorf("value %d out of range for uint8 (0-%d)", value, math.MaxUint8)
	}
	return uint8(value), nil
}

// SafeUint32ToUint8 safely converts uint32 to uint8 with bounds checking (for color components)
func SafeUint32ToUint8(value uint32) uint8 {
	// For color components, we typically want to clamp rather than error
	if value > math.MaxUint8 {
		return math.MaxUint8
	}
	return uint8(value)
}

// SafeInt32ToUint32 safely converts int32 to uint32 with bounds checking
func SafeInt32ToUint32(value int32) (uint32, error) {
	if value < 0 {
		return 0, fmt.Errorf("value %d is negative, cannot convert to uint32", value)
	}
	return uint32(value), nil
}
