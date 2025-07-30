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
	Use:   "wfm [input_file] [output_directory]",
	Short: "Extract data from WFM font files",
	Long: `Extract data from WFM font files used in Tomba! PSX game.
	
This command will:
- Parse the WFM file structure
- Extract glyph data to separate binary files
- Extract dialogue data to separate binary files  
- Generate a JSON file with metadata and structure information

Example:
  tombatools wfm CFNT999H.WFM ./output/`,
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
		fmt.Printf("- Metadata saved to: %s\n", filepath.Join(outputDir, "info.json"))
		fmt.Printf("- Glyphs extracted to: %s\n", filepath.Join(outputDir, "all_glyphs.png"))
		fmt.Printf("- Dialogues extracted to: %s\n", filepath.Join(outputDir, "dialogues"))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(wfmCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// wfmCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// wfmCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
