// Package cmd provides command-line interface for FLA file processing.
// This file contains commands for recalculating file link addresses (FLA)
// used in the Tomba! PlayStation game.
package cmd

import (
	"fmt"

	"github.com/hansbonini/tombatools/pkg"
	"github.com/hansbonini/tombatools/pkg/common"
	"github.com/spf13/cobra"
)

// flaCmd represents the parent command for all FLA file operations.
// It provides access to recalc subcommand for processing file link addresses
// from the Tomba! PlayStation game.
var flaCmd = &cobra.Command{
	Use:   "fla",
	Short: "Process File Link Addresses (FLA) from Tomba! PSX game",
	Long: `Process File Link Addresses (FLA) used in Tomba! PSX game.

Commands:
  recalc    Recalculate file addresses after modifications

Examples:
  tombatools fla recalc original.bin`,
}

// flaRecalcCmd recalculates file link addresses from a CD image.
// It reads the CD image and analyzes the FLA table structure.
var flaRecalcCmd = &cobra.Command{
	Use:   "recalc [image.bin]",
	Short: "Recalculate file addresses from CD image",
	Long: `Recalculate file link addresses from a PlayStation CD image.

This command reads a CD image file and analyzes the File Link Address (FLA)
table structure used in Tomba! PSX game.

Arguments:
  image.bin    CD image file to analyze

Example:
  tombatools fla recalc original.bin`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imageBin := args[0]

		// Enable verbose mode if requested
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("error getting verbose flag: %w", err)
		}
		common.SetVerboseMode(verbose)

		fmt.Printf("CD image: %s\n", imageBin)

		// Create FLA processor for handling recalculation operations
		processor := pkg.NewFLAProcessor()

		fmt.Printf("\nStarting FLA analysis process...\n")

		// Analyze the CD image and extract FLA table
		table, err := processor.AnalyzeCDImage(imageBin)
		if err != nil {
			return fmt.Errorf("failed to analyze CD image: %w", err)
		}

		fmt.Printf("\nFLA Table Analysis Complete!\n")
		fmt.Printf("Found %d entries at offset 0x%X\n\n", table.Count, table.Offset)

		// Display the table in organized columns (always show in verbose mode)
		if verbose || true { // Show table by default for now
			fmt.Printf("ID   | Timecode  | Size\n")
			fmt.Printf("-----|-----------|----------\n")

			for i, entry := range table.Entries {
				fmt.Printf("%04X | %s | %d\n", i, entry.Timecode.String(), entry.FileSize)
			}
		}

		return nil
	},
}

// init initializes the FLA command and its subcommands with appropriate flags.
func init() {
	// Register the FLA command with the root command
	rootCmd.AddCommand(flaCmd)

	// Add subcommands to the FLA command
	flaCmd.AddCommand(flaRecalcCmd)

	// Add verbose flag to recalc command for detailed output
	flaRecalcCmd.Flags().BoolP("verbose", "v", false, "Enable verbose output (show debug messages)")
}
