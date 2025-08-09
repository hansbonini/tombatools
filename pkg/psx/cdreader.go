// Package psx provides PlayStation-specific CD-ROM reading functionality.
// Implementation based on mkpsxiso's dumpsxiso for accurate PlayStation CD parsing.
package psx

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/hansbonini/tombatools/pkg/common"
)

// CDReader provides functionality to read CD image files with mkpsxiso-style parsing
type CDReader struct {
	file          *os.File
	totalSectors  int64
	currentSector int64
	currentOffset int
	sectorBuffer  []byte
}

// NewCDReader creates a new CD reader instance
func NewCDReader(filename string) (*CDReader, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	// Get total sectors
	fileInfo, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	totalSectors := fileInfo.Size() / CD_SECTOR_SIZE

	return &CDReader{
		file:          file,
		totalSectors:  totalSectors,
		currentSector: -1,
		sectorBuffer:  make([]byte, CD_SECTOR_SIZE),
	}, nil
}

func (r *CDReader) Close() error {
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}

// SeekToSector seeks to a specific sector - based on mkpsxiso SeekToSector
func (r *CDReader) SeekToSector(lba int64) error {
	if lba >= r.totalSectors || lba < 0 {
		return fmt.Errorf("LBA %d out of bounds (total: %d)", lba, r.totalSectors)
	}

	offset := lba * CD_SECTOR_SIZE
	_, err := r.file.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}

	// Read the sector into buffer
	_, err = io.ReadFull(r.file, r.sectorBuffer)
	if err != nil {
		return err
	}

	r.currentSector = lba
	r.currentOffset = 0
	return nil
}

// ReadBytes reads data from current position - based on mkpsxiso ReadBytes
func (r *CDReader) ReadBytes(buffer []byte) (int, error) {
	bytesRead := 0

	for bytesRead < len(buffer) {
		// Check if we need to read next sector
		if r.currentOffset >= CD_DATA_SIZE {
			err := r.SeekToSector(r.currentSector + 1)
			if err != nil {
				return bytesRead, err
			}
		}

		// Calculate available bytes in current sector (skip CD header/footer)
		// PlayStation CD-ROM Mode 2 Form 1: sync(12) + header(4) + subheader(8) + data(2048) + edc(4) + ecc(276)
		dataStart := 24 // Skip sync(12) + header(4) + subheader(8)
		available := CD_DATA_SIZE - r.currentOffset

		if available <= 0 {
			// Move to next sector
			err := r.SeekToSector(r.currentSector + 1)
			if err != nil {
				return bytesRead, err
			}
			continue
		}

		// Copy data
		toCopy := len(buffer) - bytesRead
		if toCopy > available {
			toCopy = available
		}

		copy(buffer[bytesRead:], r.sectorBuffer[dataStart+r.currentOffset:dataStart+r.currentOffset+toCopy])
		bytesRead += toCopy
		r.currentOffset += toCopy
	}

	return bytesRead, nil
}

// ValidateISO9660 - Check if file has valid ISO9660 header
func (r *CDReader) ValidateISO9660() error {
	err := r.SeekToSector(16) // Primary Volume Descriptor at sector 16
	if err != nil {
		return err
	}

	header := make([]byte, 7)
	_, err = r.ReadBytes(header)
	if err != nil {
		return err
	}

	// Check for ISO9660 signature: 0x01 + "CD001" + 0x01
	expected := []byte{0x01, 0x43, 0x44, 0x30, 0x30, 0x31, 0x01}
	for i, b := range expected {
		if header[i] != b {
			return fmt.Errorf("invalid ISO9660 signature at byte %d: got 0x%02X, expected 0x%02X", i, header[i], b)
		}
	}

	return nil
}

// ReadISODescriptor reads the ISO9660 descriptor from sector 16
func (r *CDReader) ReadISODescriptor() (*ISODescriptor, error) {
	// ISO descriptor is at sector 16
	if err := r.SeekToSector(16); err != nil {
		return nil, err
	}

	data := make([]byte, CD_DATA_SIZE)
	_, err := r.ReadBytes(data)
	if err != nil {
		return nil, err
	}

	// Validate ISO signature
	if string(data[1:6]) != "CD001" {
		return nil, fmt.Errorf("invalid ISO9660 signature")
	}

	descriptor := &ISODescriptor{}

	// Parse descriptor fields (little-endian format)
	descriptor.Type = data[0]
	copy(descriptor.ID[:], data[1:6])
	descriptor.Version = data[6]
	copy(descriptor.SystemID[:], data[8:40])
	copy(descriptor.VolumeID[:], data[40:72])
	descriptor.VolumeSpaceSizeLSB = binary.LittleEndian.Uint32(data[80:84])
	descriptor.VolumeSpaceSizeMSB = binary.BigEndian.Uint32(data[84:88])
	descriptor.LogicalBlockSizeLSB = binary.LittleEndian.Uint16(data[128:130])
	descriptor.LogicalBlockSizeMSB = binary.BigEndian.Uint16(data[130:132])
	descriptor.PathTableSizeLSB = binary.LittleEndian.Uint32(data[132:136])
	descriptor.PathTableSizeMSB = binary.BigEndian.Uint32(data[136:140])
	descriptor.PathTable1Offs = binary.LittleEndian.Uint32(data[140:144])
	descriptor.PathTable2Offs = binary.LittleEndian.Uint32(data[144:148])
	descriptor.PathTable1MSBOffs = binary.BigEndian.Uint32(data[148:152])
	descriptor.PathTable2MSBOffs = binary.BigEndian.Uint32(data[152:156])
	copy(descriptor.RootDirRecord[:], data[156:190])

	return descriptor, nil
}

// ReadPathTable reads the path table from the specified location
func (r *CDReader) ReadPathTable(lba uint32, size uint32) ([]PathTableEntry, error) {
	if err := r.SeekToSector(int64(lba)); err != nil {
		return nil, err
	}

	// Calculate number of sectors needed
	sectorsNeeded := (size + CD_DATA_SIZE - 1) / CD_DATA_SIZE

	var pathData []byte
	for i := uint32(0); i < sectorsNeeded; i++ {
		data := make([]byte, CD_DATA_SIZE)
		_, err := r.ReadBytes(data)
		if err != nil {
			return nil, err
		}
		pathData = append(pathData, data...)
	}

	// Limit to actual path table size
	if uint32(len(pathData)) > size {
		pathData = pathData[:size]
	}

	var entries []PathTableEntry
	offset := 0

	for offset < len(pathData) {
		if offset+8 > len(pathData) {
			break
		}

		entry := PathTableEntry{}
		entry.NameLength = pathData[offset]

		// End of path table
		if entry.NameLength == 0 {
			break
		}

		// Validate name length
		if entry.NameLength > 255 {
			common.LogDebug("Invalid path table entry name length: %d", entry.NameLength)
			break
		}

		entry.ExtendedAttrLength = pathData[offset+1]
		entry.DirLocation = binary.LittleEndian.Uint32(pathData[offset+2 : offset+6])
		entry.ParentDir = binary.LittleEndian.Uint16(pathData[offset+6 : offset+8])

		// Validate directory location
		if entry.DirLocation == 0 || entry.DirLocation > 1000000 { // Reasonable sector limit
			common.LogDebug("Invalid directory location: %d", entry.DirLocation)
			offset += 8 + int(entry.NameLength)
			if offset%2 != 0 {
				offset++
			}
			continue
		}

		// Read directory name
		nameStart := offset + 8
		nameEnd := nameStart + int(entry.NameLength)
		if nameEnd > len(pathData) {
			break
		}
		entry.Name = string(pathData[nameStart:nameEnd])

		// Validate directory name
		if !r.isValidFilename(entry.Name) {
			common.LogDebug("Invalid directory name: %q", entry.Name)
			offset = nameEnd
			if offset%2 != 0 {
				offset++
			}
			continue
		}

		// Align to even boundary
		offset = nameEnd
		if offset%2 != 0 {
			offset++
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// ParseDirectoryEntries parses directory entries based on mkpsxiso ReadDirEntries implementation
func (r *CDReader) ParseDirectoryEntries(lba int64, sizeInBytes uint32) ([]CDFileEntry, error) {
	var entries []CDFileEntry
	sizeInSectors := (sizeInBytes + CD_DATA_SIZE - 1) / CD_DATA_SIZE
	numEntries := 0 // Track entries to skip . and ..

	for sector := uint32(0); sector < sizeInSectors; sector++ {
		err := r.SeekToSector(lba + int64(sector))
		if err != nil {
			return nil, fmt.Errorf("failed to seek to sector %d: %v", lba+int64(sector), err)
		}

		r.currentOffset = 0 // Reset offset for new sector

		for {
			entry, entrySize, err := r.readDirectoryEntry()
			if err != nil {
				// End of sector or invalid entry
				break
			}

			// Skip first two entries (. and ..) - following mkpsxiso pattern
			if numEntries >= 2 {
				// Validate entry using mkpsxiso-style validation
				if r.isValidEntry(entry) {
					entries = append(entries, entry)
				} else {
					// Log but continue - following mkpsxiso behavior for corrupted entries
					if common.VerboseMode {
						fmt.Printf("DEBUG: Skipping invalid entry: %s (LBA: %d, Size: %d)\n",
							entry.Name, entry.LBA, entry.Size)
					}
				}
			}
			numEntries++

			// Move to next entry
			r.currentOffset += entrySize

			// Check if we've reached the end of sector
			if r.currentOffset >= CD_DATA_SIZE {
				break
			}
		}
	}

	return entries, nil
}

// Read single directory entry based on mkpsxiso ReadEntry
func (r *CDReader) readDirectoryEntry() (CDFileEntry, int, error) {
	// Check if we have enough bytes for entry header
	if r.currentOffset >= CD_DATA_SIZE {
		return CDFileEntry{}, 0, fmt.Errorf("end of sector")
	}

	// Read entry length
	dataStart := 24 // Skip sync + header + subheader
	if dataStart+r.currentOffset >= len(r.sectorBuffer) {
		return CDFileEntry{}, 0, fmt.Errorf("buffer overflow")
	}

	entryLength := int(r.sectorBuffer[dataStart+r.currentOffset])

	if entryLength == 0 {
		return CDFileEntry{}, 0, fmt.Errorf("end of directory entries")
	}

	if entryLength < 33 {
		return CDFileEntry{}, 0, fmt.Errorf("entry too short")
	}

	if dataStart+r.currentOffset+entryLength > len(r.sectorBuffer) {
		return CDFileEntry{}, 0, fmt.Errorf("entry exceeds sector bounds")
	}

	// Extract entry data from sector buffer
	entryData := r.sectorBuffer[dataStart+r.currentOffset : dataStart+r.currentOffset+entryLength]

	// Parse entry following ISO9660 standard
	entry, err := r.parseEntryData(entryData)
	if err != nil {
		return CDFileEntry{}, entryLength, err
	}

	return entry, entryLength, nil
}

func (r *CDReader) parseEntryData(data []byte) (CDFileEntry, error) {
	if len(data) < 33 {
		return CDFileEntry{}, fmt.Errorf("insufficient data")
	}

	// Parse directory entry structure - based on ISO9660 DIR_ENTRY
	length := data[0]
	_ = data[1] // extended attribute length
	lbaLE := binary.LittleEndian.Uint32(data[2:6])
	_ = binary.BigEndian.Uint32(data[6:10]) // LBA big-endian (not used)
	sizeLE := binary.LittleEndian.Uint32(data[10:14])
	_ = binary.BigEndian.Uint32(data[14:18]) // size big-endian (not used)
	// date at data[18:25]
	flags := data[25]
	// file unit size and interleave gap at data[26:28]
	// volume sequence number at data[28:32]
	filenameLength := data[32]

	if 33+int(filenameLength) > int(length) {
		return CDFileEntry{}, fmt.Errorf("filename exceeds entry bounds")
	}

	filename := string(data[33 : 33+filenameLength])

	// Clean filename similar to mkpsxiso CleanIdentifier
	filename = r.cleanIdentifier(filename)

	// Create file entry
	entry := CDFileEntry{
		Name:       filename,
		LBA:        uint32(lbaLE),
		Size:       uint32(sizeLE),
		IsDir:      (flags & 0x02) != 0,
		ExtentSize: common.GetSizeInSectors(uint32(sizeLE)),
	}

	// Set MSF
	entry.MSF = common.LBAToMSF(entry.LBA)

	return entry, nil
}

// Clean identifier following mkpsxiso style
func (r *CDReader) cleanIdentifier(name string) string {
	// Remove version suffix (;1) common in ISO9660
	if idx := strings.Index(name, ";"); idx != -1 {
		name = name[:idx]
	}

	// Handle special directory entries
	if name == "\x00" {
		return "."
	}
	if name == "\x01" {
		return ".."
	}

	return name
}

// Validate entry using mkpsxiso-style validation
func (r *CDReader) isValidEntry(entry CDFileEntry) bool {
	// Skip . and .. entries
	if entry.Name == "." || entry.Name == ".." {
		return false
	}

	// Validate LBA is within reasonable bounds
	if entry.LBA == 0 || int64(entry.LBA) >= r.totalSectors {
		return false
	}

	// Validate size is reasonable (max 700MB for CD)
	if entry.Size > 700*1024*1024 {
		return false
	}

	// Enhanced filename validation
	if !r.isValidFilename(entry.Name) {
		return false
	}

	return true
}

// Enhanced filename validation based on mkpsxiso behavior
func (r *CDReader) isValidFilename(name string) bool {
	if len(name) == 0 {
		return false
	}

	// Check for null bytes
	if strings.Contains(name, "\x00") {
		return false
	}

	// Check if name contains too many non-printable characters
	nonPrintableCount := 0
	for _, r := range name {
		if !unicode.IsPrint(r) && r != '\t' {
			nonPrintableCount++
		}
	}

	// Allow some non-printable characters but not excessive amounts
	if nonPrintableCount > len(name)/2 {
		return false
	}

	// Check for valid UTF-8
	if !utf8.ValidString(name) {
		return false
	}

	return true
}

// ExtractFile extracts a single file from the CD image with improved error handling
func (r *CDReader) ExtractFile(lba uint32, fileSize uint32, outputPath string) error {
	// Create output directory
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", outputPath, err)
	}
	defer outFile.Close()

	// Validate LBA bounds
	if int64(lba) >= r.totalSectors {
		return fmt.Errorf("LBA %d out of bounds (total sectors: %d)", lba, r.totalSectors)
	}

	// Seek to file location
	if err := r.SeekToSector(int64(lba)); err != nil {
		return fmt.Errorf("failed to seek to LBA %d: %w", lba, err)
	}

	// Copy file data with proper sector handling
	bytesLeft := fileSize
	totalWritten := uint32(0)

	for bytesLeft > 0 {
		// Calculate how much to read from current sector
		bytesToRead := CD_DATA_SIZE - uint32(r.currentOffset)
		if bytesToRead > bytesLeft {
			bytesToRead = bytesLeft
		}

		// Protect against reading beyond file bounds
		if bytesToRead == 0 {
			break
		}

		// Read data from current sector
		buffer := make([]byte, bytesToRead)
		bytesRead, err := r.ReadBytes(buffer)
		if err != nil {
			return fmt.Errorf("failed to read data at offset %d: %w", totalWritten, err)
		}

		// Only write the bytes we actually read
		if bytesRead > 0 {
			_, err = outFile.Write(buffer[:bytesRead])
			if err != nil {
				return fmt.Errorf("failed to write data at offset %d: %w", totalWritten, err)
			}
		}

		bytesLeft -= uint32(bytesRead)
		totalWritten += uint32(bytesRead)

		// Safety check to prevent infinite loops
		if bytesRead == 0 {
			break
		}
	}

	return nil
}

// Legacy compatibility methods for existing code

// BuildDirectoryPath builds the full path for a directory using the path table
func (r *CDReader) BuildDirectoryPath(entry PathTableEntry, pathTable []PathTableEntry) string {
	if entry.ParentDir == 1 { // Root directory
		return entry.Name
	}

	// Recursively build parent path
	if int(entry.ParentDir-1) < len(pathTable) {
		parentPath := r.BuildDirectoryPath(pathTable[entry.ParentDir-1], pathTable)
		return parentPath + "/" + entry.Name
	}

	return entry.Name
}

// ReadSector reads a complete sector from the CD image (legacy compatibility)
func (r *CDReader) ReadSector() (*SectorM2F1, error) {
	sector := &SectorM2F1{}

	if r.currentSector < 0 {
		return nil, fmt.Errorf("no sector loaded")
	}

	// Copy data to sector structure
	copy(sector.Sync[:], r.sectorBuffer[0:12])
	copy(sector.Address[:], r.sectorBuffer[12:15])
	sector.Mode = r.sectorBuffer[15]
	copy(sector.Data[:], r.sectorBuffer[24:24+CD_DATA_SIZE])

	return sector, nil
}

// ReadDataFromSector reads only the data portion from current sector (legacy compatibility)
func (r *CDReader) ReadDataFromSector() ([]byte, error) {
	if r.currentSector < 0 {
		return nil, fmt.Errorf("no sector loaded")
	}

	// For Mode 2 sectors, data starts at offset 24
	data := make([]byte, CD_DATA_SIZE)
	copy(data, r.sectorBuffer[24:24+CD_DATA_SIZE])
	return data, nil
}

// SeekToSector32 legacy method for compatibility with uint32
func (r *CDReader) SeekToSector32(sector uint32) error {
	return r.SeekToSector(int64(sector))
}

// CDFileEntry represents a file extracted from CD image
type CDFileEntry struct {
	ID         uint16 // 4-digit hex ID
	Name       string // File name
	Path       string // Full path within CD
	LBA        uint32 // Logical Block Address
	MSF        string // Minutes:Seconds:Frames format
	Size       uint32 // File size in bytes
	IsDir      bool   // Whether this is a directory
	ExtentSize uint32 // Size in sectors
}
