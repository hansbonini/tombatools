// Package cmd provides command-line interface for WFM file processing.
// This file contains commands for decoding and encoding WFM font files
// used in the Tomba! PlayStation game.
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hansbonini/tombatools/pkg"
	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/spf13/cobra"
)

// wfmCmd represents the parent command for all WFM file operations.
// It provides access to both decode and encode subcommands for processing
// WFM font files from the Tomba! PlayStation game.
var wfmCmd = &cobra.Command{
	Use:   "wfm",
	Short: "Process WFM font files from Tomba! PSX game",
	Long: `Process WFM font files used in Tomba! PSX game.

Commands:
  decode    Extract glyphs (PNG) and dialogues (YAML) from WFM files
  encode    Create WFM files from YAML dialogues and font PNG files

Examples:
  tombatools wfm decode CFNT999H.WFM ./output/
  tombatools wfm encode dialogues.yaml output.wfm`,
}

// wfmDecodeCmd extracts glyphs and dialogues from WFM font files.
// It parses the WFM file structure and exports individual glyph PNG files
// and a YAML file containing dialogue data with automatic text decoding.
var wfmDecodeCmd = &cobra.Command{
	Use:   "decode [input_file] [output_directory]",
	Short: "Extract glyphs and dialogues from WFM files",
	Long: `Extract glyphs and dialogues from WFM font files.

Output:
  - Individual glyph PNG files in ./glyphs/
  - Dialogue YAML file with decoded text and metadata
  - Automatic glyph-to-character mapping (if fonts/ directory exists)

Example:
  tombatools wfm decode CFNT999H.WFM ./output/`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputDir := args[1]

		// Enable verbose mode if requested
		verbose, _ := cmd.Flags().GetBool("verbose")
		common.SetVerboseMode(verbose)

		// Create WFM processor for handling decode operations
		processor := pkg.NewWFMProcessor()

		// Process the WFM file: decode structure and export data
		fmt.Printf("Processing WFM file: %s\n", inputFile)
		fmt.Printf("Output directory: %s\n", outputDir)

		if err := processor.Process(inputFile, outputDir); err != nil {
			return fmt.Errorf("failed to process WFM file: %w", err)
		}

		// Display success message with output locations
		fmt.Println("WFM file processed successfully!")
		fmt.Printf("- Individual glyph PNG files saved to: %s\n", filepath.Join(outputDir, "glyphs"))
		fmt.Printf("- Dialogues extracted to: %s\n", filepath.Join(outputDir, "dialogues.yaml"))

		return nil
	},
}

// wfmEncodeCmd creates WFM font files from YAML dialogue data and PNG font files.
// It reads dialogue data from a YAML file and corresponding PNG glyph files
// to generate a complete WFM file ready for use in the Tomba! game.
var wfmEncodeCmd = &cobra.Command{
	Use:   "encode dialogue.yaml [output_file]",
	Short: "Create WFM files from YAML dialogues",
	Long: `Create WFM font files from YAML dialogue data and PNG font files.

Requirements:
  - YAML file with dialogue data (from decode command)
  - fonts/ directory with character PNG files (8/, 16/, 24/ subdirectories)

Output:
  - Complete WFM file ready for use in Tomba! PSX game

Example:
  tombatools wfm encode dialogues.yaml CFNT999H_modified.WFM`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputFile := args[1]

		// Enable verbose mode if requested
		verbose, _ := cmd.Flags().GetBool("verbose")
		common.SetVerboseMode(verbose)

		fmt.Printf("Input file: %s\n", inputFile)
		fmt.Printf("Output WFM file: %s\n", outputFile)

		// Create WFM encoder for handling encode operations
		encoder := pkg.NewWFMEncoder()

		// Encode the YAML file to WFM format
		if err := encoder.Encode(inputFile, outputFile); err != nil {
			return fmt.Errorf("failed to encode WFM file: %w", err)
		}

		fmt.Println("WFM file encoded successfully!")
		return nil
	},
}

// init initializes the WFM command and its subcommands with appropriate flags.
func init() {
	// Register the WFM command with the root command
	rootCmd.AddCommand(wfmCmd)

	// Add subcommands to the WFM command
	wfmCmd.AddCommand(wfmDecodeCmd)
	wfmCmd.AddCommand(wfmEncodeCmd)

	// Add verbose flag to decode command for detailed output
	wfmDecodeCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output (show debug messages)")

	// Add verbose flag to encode command for detailed output
	wfmEncodeCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output (show debug messages)")
}
