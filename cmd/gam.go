// Package cmd provides command-line interface for GAM file processing.
// This file contains commands for decoding and encoding GAM files
// used in the Tomba! PlayStation game.
package cmd

import (
	"fmt"

	"github.com/hansbonini/tombatools/pkg"
	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/spf13/cobra"
)

// gamCmd represents the parent command for all GAM file operations.
// It provides access to both unpack and pack subcommands for processing
// GAM files from the Tomba! PlayStation game.
var gamCmd = &cobra.Command{
	Use:   "gam",
	Short: "Process GAM files from Tomba! PSX game",
	Long: `Process GAM files used in Tomba! PSX game.

Commands:
  unpack    Extract data from GAM files
  pack      Create GAM files from extracted data

Examples:
  tombatools gam unpack input.GAM output.UNGAM
  tombatools gam pack input.UNGAM output.GAM`,
}

// gamUnpackCmd extracts data from GAM files.
// It parses the GAM file structure and exports the contained data
// in a human-readable format for modification.
var gamUnpackCmd = &cobra.Command{
	Use:   "unpack [input_file] [output_file]",
	Short: "Extract data from GAM files",
	Long: `Extract data from GAM files used in Tomba! PSX game.

Output:
  - Extracted data file (.UNGAM)
  - Decompressed game data

Example:
  tombatools gam unpack GAME.GAM data.UNGAM`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputFile := args[1]

		// Enable verbose mode if requested
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("error getting verbose flag: %w", err)
		}
		common.SetVerboseMode(verbose)

		// Create GAM processor for handling unpack operations
		processor := pkg.NewGAMProcessor()

		fmt.Printf("Processing GAM file: %s\n", inputFile)
		fmt.Printf("Output file: %s\n", outputFile)

		// Unpack the GAM file
		if err := processor.UnpackGAM(inputFile, outputFile); err != nil {
			return fmt.Errorf("failed to unpack GAM file: %w", err)
		}

		fmt.Println("GAM file unpacked successfully!")
		return nil
	},
}

// gamPackCmd creates GAM files from extracted data.
// It reads processed data and reconstructs a GAM file
// ready for use in the Tomba! game.
var gamPackCmd = &cobra.Command{
	Use:   "pack [input_file] [output_file]",
	Short: "Create GAM files from extracted data",
	Long: `Create GAM files from extracted data.

Requirements:
  - Uncompressed data file (from unpack command)

Output:
  - Complete GAM file ready for use in Tomba! PSX game

Example:
  tombatools gam pack data.UNGAM GAME_modified.GAM`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputFile := args[1]

		// Enable verbose mode if requested
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("error getting verbose flag: %w", err)
		}
		common.SetVerboseMode(verbose)

		// Create GAM processor for handling pack operations
		processor := pkg.NewGAMProcessor()

		fmt.Printf("Input file: %s\n", inputFile)
		fmt.Printf("Output GAM file: %s\n", outputFile)

		// Pack the file into GAM format
		if err := processor.PackGAM(inputFile, outputFile); err != nil {
			return fmt.Errorf("failed to pack GAM file: %w", err)
		}

		fmt.Println("GAM file packed successfully!")
		return nil
	},
}

// init initializes the GAM command and its subcommands with appropriate flags.
func init() {
	// Register the GAM command with the root command
	rootCmd.AddCommand(gamCmd)

	// Add subcommands to the GAM command
	gamCmd.AddCommand(gamUnpackCmd)
	gamCmd.AddCommand(gamPackCmd)

	// Add verbose flag to unpack command for detailed output
	gamUnpackCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output (show debug messages)")

	// Add verbose flag to pack command for detailed output
	gamPackCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output (show debug messages)")
}
