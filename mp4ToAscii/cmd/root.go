package cmd

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"io"
	"os"

	tsize "github.com/kopoli/go-terminal-size"
	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		var tSize tsize.Size

		tSize, err := tsize.GetSize()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Current terminal size is", tSize.Width, "by", tSize.Height)

		img, err := jpeg.Decode(ReadFrameAsJpeg("..\\input.mp4", 1))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		file, err := os.Create("..\\output.jpeg")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	},
  }
  
  func Execute() {
	if err := rootCmd.Execute(); err != nil {
	  fmt.Fprintln(os.Stderr, err)
	  os.Exit(1)
	}
  }

  func ReadFrameAsJpeg(inFileName string, frameNum int) io.Reader {
	buf := bytes.NewBuffer(nil)
	err := ffmpeg.Input(inFileName).
		Filter("select", ffmpeg.Args{fmt.Sprintf("gte(n,%d)", frameNum)}).
		Output("pipe:", ffmpeg.KwArgs{"loglevel":"warning", "vframes": 1, "format": "image2", "vcodec": "mjpeg"}).
		WithOutput(buf, os.Stdout).
		Run()
	if err != nil {
		panic(err)
	}
	return buf
}