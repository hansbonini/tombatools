/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/hansbonini/tombatools/pkg"
	"github.com/spf13/cobra"
)

// wfmCmd represents the wfm command
var wfmCmd = &cobra.Command{
	Use:   "wfm",
	Short: "Process WFM font files",
	Long: `Process WFM font files used in Tomba! PSX game.
	
Available subcommands:
- decode: Extract data from WFM files (glyphs, dialogues)
- encode: Create WFM files from extracted data (coming soon)

Examples:
  tombatools wfm decode CFNT999H.WFM ./output/
  tombatools wfm encode ./input/ output.wfm`,
}

// wfmDecodeCmd represents the wfm decode command
var wfmDecodeCmd = &cobra.Command{
	Use:   "decode [input_file] [output_directory]",
	Short: "Extract data from WFM font files",
	Long: `Extract data from WFM font files used in Tomba! PSX game.
	
This command will:
- Parse the WFM file structure
- Extract glyph data to PNG files
- Extract dialogue data to YAML file with font height information
- Generate glyph mapping for text decoding

Example:
  tombatools wfm decode CFNT999H.WFM ./output/`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputDir := args[1]

		// Create WFM processor
		processor := pkg.NewWFMProcessor()

		// Process the file
		fmt.Printf("Processing WFM file: %s\n", inputFile)
		fmt.Printf("Output directory: %s\n", outputDir)

		if err := processor.Process(inputFile, outputDir); err != nil {
			return fmt.Errorf("failed to process WFM file: %w", err)
		}

		fmt.Println("WFM file processed successfully!")
		fmt.Printf("- Individual glyph PNG files saved to: %s\n", filepath.Join(outputDir, "glyphs"))
		fmt.Printf("- Dialogues extracted to: %s\n", filepath.Join(outputDir, "dialogues.yaml"))

		return nil
	},
}

// wfmEncodeCmd represents the wfm encode command
var wfmEncodeCmd = &cobra.Command{
	Use:   "encode dialogue.yaml [output_file]",
	Short: "Create WFM font files from extracted data",
	Long: `Create WFM font files from extracted data (PNG glyphs and YAML dialogues).
	
This command will:
- Read dialogue data from YAML file
- Read PNG glyph files from the proper font directory based on font height
- Generate a WFM file with the correct structure

Example:
  tombatools wfm encode dialogue.yaml output.wfm`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		inputFile := args[0]
		outputFile := args[1]

		fmt.Printf("Input file: %s\n", inputFile)
		fmt.Printf("Output WFM file: %s\n", outputFile)

		// Create WFM encoder
		encoder := pkg.NewWFMEncoder()

		// Encode the YAML file to WFM
		if err := encoder.Encode(inputFile, outputFile); err != nil {
			return fmt.Errorf("failed to encode WFM file: %w", err)
		}

		fmt.Println("WFM file encoded successfully!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(wfmCmd)

	// Add subcommands to wfm
	wfmCmd.AddCommand(wfmDecodeCmd)
	wfmCmd.AddCommand(wfmEncodeCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// wfmCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// wfmCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
