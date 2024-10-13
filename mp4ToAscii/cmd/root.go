package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"strconv"

	tsize "github.com/kopoli/go-terminal-size"
	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		filename := "..\\input.mp4"
		//Get the size of the terminal
		var tSize tsize.Size
		tSize, err := tsize.GetSize()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Current terminal size is", tSize.Width, "by", tSize.Height)

		//Get the size and total frames of the input video
		_, _, frameCount := GetVideoInfo(filename)

		var frames []image.Image
		//Read all frames, decode them into jpeg, then append them to the slice frames
		for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
			frame, err := jpeg.Decode(ReadFrameAsJpeg(filename, frameIndex))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			frames = append(frames, frame)
		}

		//DELETE LATER. Save the first frame for testing purposes
			img, _ := jpeg.Decode(ReadFrameAsJpeg(filename, 0))
			file, _ := os.Create("..\\output.jpeg")
			_ = jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
	},
}
  
func Execute() {
	if err := rootCmd.Execute(); err != nil {
	  fmt.Fprintln(os.Stderr, err)
	  os.Exit(1)
	}
}

//Return an io.Reader containing frame# frameNum
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

//Return width, height, and total frames of video
func GetVideoInfo(inFileName string) (int, int, int) {
	data, err := ffmpeg.Probe(inFileName)
	if err != nil {
		panic(err)
	}

	type VideoInfo struct {
		Streams []struct {
			Width     int
			Height    int
			Frames string `json:"nb_frames"`
		} `json:"streams"`
	}
	vInfo := &VideoInfo{}
	err = json.Unmarshal([]byte(data), vInfo)
	if err != nil {
		panic(err)
	}

	frames,err := strconv.Atoi(vInfo.Streams[0].Frames)
	if err != nil {
		panic(err)
	}
	return vInfo.Streams[0].Width, vInfo.Streams[0].Height, frames
}