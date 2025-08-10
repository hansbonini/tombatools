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

// flaRecalcCmd recalculates file link addresses by comparing original and modified CD images.
// It detects differences and updates the FLA table in the modified image.
var flaRecalcCmd = &cobra.Command{
	Use:   "recalc [original.bin] [modified.bin]",
	Short: "Recalculate file addresses by comparing original and modified CD images",
	Long: `Recalculate file link addresses by comparing original and modified CD images.

This command compares two CD images, detects files with different MSF timecodes
and sizes, and recalculates the File Link Address (FLA) table in the modified image.

Arguments:
  original.bin    Original CD image file (reference)
  modified.bin    Modified CD image file (to be updated)

Flags:
  -v, --verbose       Enable verbose output (show debug messages)
  -s, --save-table    Save the recalculated FLA table to a .bin file

Examples:
  tombatools fla recalc original.bin modified.bin
  tombatools fla recalc -v original.bin modified.bin
  tombatools fla recalc --save-table fla_table.bin original.bin modified.bin`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		originalBin := args[0]
		modifiedBin := args[1]

		// Enable verbose mode if requested
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			return fmt.Errorf("error getting verbose flag: %w", err)
		}
		common.SetVerboseMode(verbose)

		// Check if user wants to save FLA table to a separate file
		saveTable, err := cmd.Flags().GetString("save-table")
		if err != nil {
			return fmt.Errorf("error getting save-table flag: %w", err)
		}

		fmt.Printf("Original CD image: %s\n", originalBin)
		fmt.Printf("Modified CD image: %s\n", modifiedBin)

		// Create FLA processor for handling recalculation operations
		processor := pkg.NewFLAProcessor()

		fmt.Printf("\nAnalyzing original CD image...\n")

		// Analyze the original CD image and extract FLA table
		originalTable, err := processor.AnalyzeCDImage(originalBin)
		if err != nil {
			return fmt.Errorf("failed to analyze original CD image: %w", err)
		}

		fmt.Printf("Original FLA Table: Found %d entries at offset 0x%X\n", originalTable.Count, originalTable.Offset)

		fmt.Printf("\nAnalyzing modified CD image...\n")

		// Analyze the modified CD image and extract FLA table
		modifiedTable, err := processor.AnalyzeCDImage(modifiedBin)
		if err != nil {
			return fmt.Errorf("failed to analyze modified CD image: %w", err)
		}

		fmt.Printf("Modified FLA Table: Found %d entries at offset 0x%X\n", modifiedTable.Count, modifiedTable.Offset)

		fmt.Printf("\nComparing actual files between CD images to detect differences...\n")

		// Compare actual files in CD images to detect differences
		fileDifferences, err := processor.CompareCDFiles(originalBin, modifiedBin, originalTable, modifiedTable)
		if err != nil {
			return fmt.Errorf("failed to compare CD files: %w", err)
		}

		if len(fileDifferences) == 0 {
			fmt.Printf("No differences found between CD files.\n")
			return nil
		}

		fmt.Printf("Found %d file differences that require FLA table updates:\n\n", len(fileDifferences))

		fmt.Printf("\nRecalculating FLA table in modified image...\n")

		// Recalculate and update the FLA table in the modified image
		err = processor.RecalculateFLATable(modifiedBin, originalTable, modifiedTable, fileDifferences)
		if err != nil {
			return fmt.Errorf("failed to recalculate FLA table: %w", err)
		}

		// Save FLA table to separate file if requested
		if saveTable != "" {
			fmt.Printf("Saving recalculated FLA table to: %s\n", saveTable)
			err = processor.SaveFLATableToFile(modifiedTable, saveTable)
			if err != nil {
				return fmt.Errorf("failed to save FLA table to file: %w", err)
			}
			fmt.Printf("FLA table saved successfully!\n")
		}

		// Display differences after recalculation to show updated values
		fmt.Printf("ID   | FLA MSF        | Original Size | Modified Size | Size Diff | File\n")
		fmt.Printf("-----|----------------|---------------|---------------|-----------|--------------------------------------------------\n")

		for _, diff := range fileDifferences {
			originalEntry := originalTable.Entries[diff.EntryIndex]
			modifiedEntry := modifiedTable.Entries[diff.EntryIndex]
			
			filename := "NOT LINKED"
			if modifiedEntry.LinkedFile != nil {
				filename = modifiedEntry.LinkedFile.FullPath
			} else if originalEntry.LinkedFile != nil {
				filename = originalEntry.LinkedFile.FullPath
			}

			// Use FLA table sizes for display (after recalculation they will show the updated sizes)
			originalSize := originalEntry.FileSize
			modifiedSize := modifiedEntry.FileSize

			sizeDiff := int64(modifiedSize) - int64(originalSize)
			sizeDiffStr := fmt.Sprintf("%+d", sizeDiff)

			fmt.Printf("%04X | %-14s | %-13d | %-13d | %-9s | %s\n",
				diff.EntryIndex,
				originalEntry.Timecode.String(),
				originalSize,
				modifiedSize,
				sizeDiffStr,
				filename)
		}

		fmt.Printf("FLA table recalculation complete!\n")
		fmt.Printf("\nSummary:\n")
		fmt.Printf("- Detected %d file(s) with size changes\n", len(fileDifferences))
		fmt.Printf("- Updated FLA table written to: %s\n", modifiedBin)
		fmt.Printf("- All subsequent file positions have been recalculated\n")

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
	
	// Add save-table flag to save the recalculated FLA table to a separate .bin file
	flaRecalcCmd.Flags().StringP("save-table", "s", "", "Save the recalculated FLA table to a .bin file")
}
