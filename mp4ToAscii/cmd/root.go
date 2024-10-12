package cmd

import (
	"fmt"
	"os"

	tsize "github.com/kopoli/go-terminal-size"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		var tSize tsize.Size

		tSize, err := tsize.GetSize()
		if err == nil {
			fmt.Println("Current size is", tSize.Width, "by", tSize.Height)
		}
	},
  }
  
  func Execute() {
	if err := rootCmd.Execute(); err != nil {
	  fmt.Fprintln(os.Stderr, err)
	  os.Exit(1)
	}
  }