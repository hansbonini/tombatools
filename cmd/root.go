// Package cmd provides command-line interface functionality for TombaTools.
// TombaTools is a collection of utilities for extracting and modifying
// game files from Tomba! (Ore no Tomba) for PlayStation.
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands.
// It provides the main entry point for the TombaTools application.
var rootCmd = &cobra.Command{
	Use:   "tombatools",
	Short: "Tools for modding Tomba! PSX game files",
	Long: `TombaTools - A collection of utilities for extracting and modifying 
game files from Tomba! (Ore no Tomba) for PlayStation.

Currently supports:
  - WFM font files (extract/create glyphs and dialogues)
  - GAM files (unpack/pack game data)
  - CD image files (extract files from ISO9660 file system)
  - FLA files (recalculate file link addresses)

Examples:
  tombatools wfm decode CFNT999H.WFM ./output/
  tombatools wfm encode dialogues.yaml CFNT999H_modified.WFM
  tombatools gam unpack GAME.GAM data.UNGAM
  tombatools gam pack data.UNGAM GAME_modified.GAM
  tombatools cd dump original.bin ./output/
  tombatools cd dump -v original.bin ./output/
  tombatools fla recalc original.bin

Use 'tombatools [command] --help' for more information about a command.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main() and serves as the entry point for command execution.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// init initializes the root command with flags and configuration settings.
func init() {
	// Note: Persistent flags defined here would be global for the entire application.
	// Local flags only run when this specific command is called directly.

	// Example toggle flag (can be removed if not needed)
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
