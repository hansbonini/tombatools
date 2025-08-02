// Package common provides tests for message and logging functionality
package common

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)

func TestSetVerboseMode(t *testing.T) {
	// Test enabling verbose mode
	SetVerboseMode(true)
	if !VerboseMode {
		t.Error("SetVerboseMode(true) should enable verbose mode")
	}

	// Test disabling verbose mode
	SetVerboseMode(false)
	if VerboseMode {
		t.Error("SetVerboseMode(false) should disable verbose mode")
	}
}

func TestLogDebug_VerboseEnabled(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // Restore default output

	// Enable verbose mode
	SetVerboseMode(true)

	// Test debug logging
	testMessage := "Test debug message with value: %d"
	LogDebug(testMessage, 42)

	output := buf.String()
	if !strings.Contains(output, "Test debug message with value: 42") {
		t.Errorf("LogDebug output should contain formatted message, got: %q", output)
	}
}

func TestLogDebug_VerboseDisabled(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // Restore default output

	// Disable verbose mode
	SetVerboseMode(false)

	// Test debug logging (should be silent)
	LogDebug("This should not appear", 42)

	output := buf.String()
	if output != "" {
		t.Errorf("LogDebug should be silent when verbose mode is disabled, got: %q", output)
	}
}

func TestLogInfo(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // Restore default output

	// Test info logging
	testMessage := "Test info message with value: %s"
	LogInfo(testMessage, "test")

	output := buf.String()
	if !strings.Contains(output, "Test info message with value: test") {
		t.Errorf("LogInfo output should contain formatted message, got: %q", output)
	}
}

func TestLogWarn(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // Restore default output

	// Test warning logging
	testMessage := "Test warning message with value: %d"
	LogWarn(testMessage, 123)

	output := buf.String()
	if !strings.Contains(output, "Test warning message with value: 123") {
		t.Errorf("LogWarn output should contain formatted message, got: %q", output)
	}
}

func TestLogError(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr) // Restore default output

	// Test error logging
	testMessage := "Test error message with value: %s"
	LogError(testMessage, "error")

	output := buf.String()
	if !strings.Contains(output, "Test error message with value: error") {
		t.Errorf("LogError output should contain formatted message, got: %q", output)
	}
}

func TestFormatError(t *testing.T) {
	baseMessage := "Base error message"
	originalError := fmt.Errorf("original error")

	formattedError := FormatError(baseMessage, originalError)

	expectedMessage := "Base error message: original error"
	if formattedError.Error() != expectedMessage {
		t.Errorf("FormatError() = %q, want %q", formattedError.Error(), expectedMessage)
	}
}

func TestFormatError_NilError(t *testing.T) {
	// This test should verify the behavior when details is nil
	// Since FormatError expects an error interface, we'll test with a nil error instead
	baseMessage := "Base error message"
	var nilError error = nil

	// This should panic as the current implementation doesn't handle nil
	defer func() {
		if r := recover(); r == nil {
			t.Error("FormatError() should panic with nil error")
		}
	}()

	FormatError(baseMessage, nilError)
}

func TestErrorConstants(t *testing.T) {
	// Test that error constants are not empty
	errorConstants := map[string]string{
		"ErrFailedToLoadDialogues":        ErrFailedToLoadDialogues,
		"ErrFailedToReadYAMLFile":         ErrFailedToReadYAMLFile,
		"ErrFailedToParseYAML":            ErrFailedToParseYAML,
		"ErrFailedToMapGlyphs":            ErrFailedToMapGlyphs,
		"ErrFailedToRecodeDialogues":      ErrFailedToRecodeDialogues,
		"ErrFailedToBuildWFM":             ErrFailedToBuildWFM,
		"ErrFailedToWriteWFM":             ErrFailedToWriteWFM,
		"ErrFailedToLoadPNG":              ErrFailedToLoadPNG,
		"ErrFailedToConvertTo4bpp":        ErrFailedToConvertTo4bpp,
		"ErrFailedToCreateOutputFile":     ErrFailedToCreateOutputFile,
		"ErrFailedToWriteHeader":          ErrFailedToWriteHeader,
		"ErrFailedToWriteGlyphPointer":    ErrFailedToWriteGlyphPointer,
		"ErrFailedToWriteGlyphClut":       ErrFailedToWriteGlyphClut,
		"ErrFailedToWriteGlyphHeight":     ErrFailedToWriteGlyphHeight,
		"ErrFailedToWriteGlyphWidth":      ErrFailedToWriteGlyphWidth,
		"ErrFailedToWriteGlyphHandakuten": ErrFailedToWriteGlyphHandakuten,
		"ErrFailedToWriteGlyphImage":      ErrFailedToWriteGlyphImage,
		"ErrFailedToWriteGlyphPadding":    ErrFailedToWriteGlyphPadding,
		"ErrFailedToWriteDialoguePointer": ErrFailedToWriteDialoguePointer,
		"ErrFailedToWriteDialogueData":    ErrFailedToWriteDialogueData,
		"ErrFailedToWriteDialoguePadding": ErrFailedToWriteDialoguePadding,
		"ErrFailedToWritePadding":         ErrFailedToWritePadding,
		"ErrFailedToGetFilePosition":      ErrFailedToGetFilePosition,
		"ErrGlyphFileNotFound":            ErrGlyphFileNotFound,
		"ErrCharacterIgnored":             ErrCharacterIgnored,
		"ErrCharacterIgnoredNoGlyph":      ErrCharacterIgnoredNoGlyph,
		"ErrReservedDataSize":             ErrReservedDataSize,
	}

	for name, value := range errorConstants {
		if value == "" {
			t.Errorf("Error constant %s should not be empty", name)
		}
		if len(value) < 10 {
			t.Errorf("Error constant %s seems too short: %q", name, value)
		}
	}
}

func TestInfoConstants(t *testing.T) {
	// Test that info constants are not empty
	infoConstants := map[string]string{
		"InfoUniqueCharactersFound": InfoUniqueCharactersFound,
		"InfoTotalUniqueCharacters": InfoTotalUniqueCharacters,
	}

	for name, value := range infoConstants {
		if value == "" {
			t.Errorf("Info constant %s should not be empty", name)
		}
	}
}

// Test logging with multiple arguments
func TestLogFunctions_MultipleArgs(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Test with multiple format arguments
	LogInfo("Test with multiple args: %d, %s, %v", 42, "text", true)

	output := buf.String()
	expected := "Test with multiple args: 42, text, true"
	if !strings.Contains(output, expected) {
		t.Errorf("LogInfo with multiple args should contain %q, got: %q", expected, output)
	}
}

// Test logging with no format arguments
func TestLogFunctions_NoArgs(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Test with no format arguments
	LogInfo("Simple message without formatting")

	output := buf.String()
	expected := "Simple message without formatting"
	if !strings.Contains(output, expected) {
		t.Errorf("LogInfo without args should contain %q, got: %q", expected, output)
	}
}

// Test VerboseMode as global variable
func TestVerboseMode_GlobalVariable(t *testing.T) {
	// Test initial state
	originalMode := VerboseMode
	defer SetVerboseMode(originalMode) // Restore original state

	// Test direct assignment
	VerboseMode = true
	if !VerboseMode {
		t.Error("Direct assignment VerboseMode = true should work")
	}

	VerboseMode = false
	if VerboseMode {
		t.Error("Direct assignment VerboseMode = false should work")
	}
}
