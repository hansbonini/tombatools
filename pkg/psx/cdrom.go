// Package psx provides PlayStation-specific structures and functionality.
// This file contains CD-ROM related structures for PlayStation disc images.
package psx

// Sector size constants for PlayStation CD-ROM
const (
	CD_SECTOR_SIZE  = 2352 // Full CD sector size
	CD_DATA_SIZE    = 2048 // Data portion of Mode 1 sector
	CD_XA_DATA_SIZE = 2336 // Data portion of Mode 2 Form 2 sector
	CD_SYNC_SIZE    = 12   // Sync pattern size
	CD_HEADER_SIZE  = 4    // Header size (3 address bytes + 1 mode byte)
)

// SectorM2F1 represents a Mode 2 Form 1 sector (used in regular files)
type SectorM2F1 struct {
	Sync     [12]byte   // Sync pattern
	Address  [3]byte    // Sector address (MSF format)
	Mode     byte       // Mode (usually 2)
	Data     [2048]byte // User data
	EDC      [4]byte    // Error Detection Code
	Reserved [8]byte    // Reserved area
	ECC      [276]byte  // Error Correction Code
}

// SectorM2F2 represents a Mode 2 Form 2 sector (used in XA/STR files)
type SectorM2F2 struct {
	Sync      [12]byte   // Sync pattern
	Address   [3]byte    // Sector address (MSF format)
	Mode      byte       // Mode (usually 2)
	SubHeader [8]byte    // XA subheader
	Data      [2324]byte // User data
	EDC       [4]byte    // Error Detection Code
}

// ISO9660 directory entry structure
type ISODirEntry struct {
	EntryLength          byte    // Length of directory record
	ExtendedAttrLength   byte    // Extended attribute record length
	ExtentLocationLSB    uint32  // Location of extent (LBA) - little endian
	ExtentLocationMSB    uint32  // Location of extent (LBA) - big endian
	DataLengthLSB        uint32  // Data length - little endian
	DataLengthMSB        uint32  // Data length - big endian
	RecordingDateTime    [7]byte // Recording date and time
	FileFlags            byte    // File flags
	FileUnitSize         byte    // File unit size
	InterleaveGapSize    byte    // Interleave gap size
	VolumeSequenceNumLSB uint16  // Volume sequence number - little endian
	VolumeSequenceNumMSB uint16  // Volume sequence number - big endian
	FileIdentifierLength byte    // Length of file identifier
	// File identifier and padding follow
}

// ISO9660 descriptor structure
type ISODescriptor struct {
	Type                   byte      // Volume descriptor type
	ID                     [5]byte   // Standard identifier "CD001"
	Version                byte      // Volume descriptor version
	SystemID               [32]byte  // System identifier
	VolumeID               [32]byte  // Volume identifier
	Reserved1              [8]byte   // Reserved
	VolumeSpaceSizeLSB     uint32    // Volume space size - little endian
	VolumeSpaceSizeMSB     uint32    // Volume space size - big endian
	Reserved2              [32]byte  // Reserved
	VolumeSetSizeLSB       uint16    // Volume set size - little endian
	VolumeSetSizeMSB       uint16    // Volume set size - big endian
	VolumeSequenceNumLSB   uint16    // Volume sequence number - little endian
	VolumeSequenceNumMSB   uint16    // Volume sequence number - big endian
	LogicalBlockSizeLSB    uint16    // Logical block size - little endian
	LogicalBlockSizeMSB    uint16    // Logical block size - big endian
	PathTableSizeLSB       uint32    // Path table size - little endian
	PathTableSizeMSB       uint32    // Path table size - big endian
	PathTable1Offs         uint32    // LBA to Type-L path table
	PathTable2Offs         uint32    // LBA to optional Type-L path table
	PathTable1MSBOffs      uint32    // LBA to Type-M path table
	PathTable2MSBOffs      uint32    // LBA to optional Type-M path table
	RootDirRecord          [34]byte  // Directory entry for root directory
	VolumeSetIdentifier    [128]byte // Volume set identifier
	PublisherIdentifier    [128]byte // Publisher identifier
	DataPreparerIdentifier [128]byte // Data preparer identifier
	ApplicationIdentifier  [128]byte // Application identifier
	CopyrightFileID        [37]byte  // Copyright file identifier
	AbstractFileID         [37]byte  // Abstract file identifier
	BibliographicFileID    [37]byte  // Bibliographic file identifier
	VolumeCreateDate       [17]byte  // Volume creation date
	VolumeModifyDate       [17]byte  // Volume modification date
	VolumeExpiryDate       [17]byte  // Volume expiry date
	VolumeEffectiveDate    [17]byte  // Volume effective date
	FileStructureVersion   byte      // File structure version
	Reserved3              byte      // Reserved
	ApplicationUse         [512]byte // Application use
	Reserved4              [653]byte // Reserved
}

// PathTableEntry represents an entry in the path table
type PathTableEntry struct {
	NameLength         byte
	ExtendedAttrLength byte
	DirLocation        uint32
	ParentDir          uint16
	Name               string
}
