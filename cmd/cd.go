// Package cmd provides command-line interface for CD image processing.
// This file contains commands for dumping and extracting files from CD images
// used in PlayStation games.
package cmd

import (
	"fmt"

	"github.com/hansbonini/tombatools/pkg"
	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/spf13/cobra"
)

// cdCmd represents the parent command for all CD image operations.
// It provides access to dump subcommand for processing CD images
// from PlayStation games.
var cdCmd = &cobra.Command{
	Use:   "cd",
	Short: "Process CD image files from PlayStation games",
	Long: `Process CD image files used in PlayStation games.

Commands:
  dump      Extract files from CD image files (.bin format)

Examples:
  tombatools cd dump original.bin ./output/`,
}

// cdDumpCmd extracts files from CD image files.
// It parses the ISO9660 file system structure and exports individual files
// with detailed logging when verbose mode is enabled.
var cdDumpCmd = &cobra.Command{
	Use:   "dump [input_file] [output_directory]",
	Short: "Extract files from CD image files",
	Long: `Extract files from CD image files (.bin format).

This command reads PlayStation CD images in .bin format and extracts all files
from the ISO9660 file system. When verbose mode is enabled (-v), it displays
detailed information about each file including:
  - ID (4-digit hex)
  - MSF (Minutes:Seconds:Frames)
  - LBA (Logical Block Address)
  - Size in bytes
  - Path within the CD structure

Output:
  - Extracted files maintain the original directory structure
  - Detailed log of file information (when -v flag is used)

Example:
  tombatools cd dump original.bin ./output/
  tombatools cd dump -v original.bin ./output/`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputDir := args[1]

		// Enable verbose mode if requested
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("error getting verbose flag: %w", err)
		}
		common.SetVerboseMode(verbose)

		// Create CD processor for handling dump operations
		processor := pkg.NewCDProcessor()

		// Process the CD image file: parse structure and extract files
		fmt.Printf("Processing CD image file: %s\n", inputFile)
		fmt.Printf("Output directory: %s\n", outputDir)

		if err := processor.Dump(inputFile, outputDir); err != nil {
			return fmt.Errorf("failed to process CD image file: %w", err)
		}

		fmt.Println("CD image file processed successfully!")
		fmt.Printf("Files extracted to: %s\n", outputDir)

		return nil
	},
}

// init initializes the CD command with its subcommands and flags.
func init() {
	// Add the CD command to the root command
	rootCmd.AddCommand(cdCmd)

	// Add the dump subcommand to the CD command
	cdCmd.AddCommand(cdDumpCmd)

	// Add verbose flag to the dump command
	cdDumpCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output with detailed file information")
}
