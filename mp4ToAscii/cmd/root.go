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

	"golang.org/x/image/draw"

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

		//Get the size and total frames of the input video
		originalWidth, originalHeight, frameCount := GetVideoInfo(filename)

		//Set the new width and height based on the terminal size
		newHeight := 0
		newWidth := 0
		
		if tSize.Width / 2 > tSize.Height{
			newHeight = tSize.Height
			newWidth = (int((float32(newHeight) / float32(originalHeight)) * float32(originalWidth)))
		} else{
			newWidth = tSize.Width / 2
			newHeight = (int((float32(newWidth) / float32(originalWidth)) * float32(originalHeight)))
		}
		
		var frames []image.Image
		//Read all frames, decode them from jpeg, then append them to the slice frames
		for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
			frame, err := jpeg.Decode(ReadFrameAsJpeg(filename, frameIndex))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			frames = append(frames, frame)
		}

		//For each frame make a blank image of the new size and then draw over it
		for frameIndex, frame := range frames{
			resizedImage:= image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
			//Add flag for different quality options https://pkg.go.dev/golang.org/x/image/draw#pkg-variables
			draw.CatmullRom.Scale(resizedImage, resizedImage.Rect, frame, frame.Bounds(), draw.Over, nil)
			frames[frameIndex] = resizedImage
		}

		//Slice with the ASCII representation for a pixel. Each frame is stored as a string with escape sequences for color and newlines
		var asciiList[]string
		//For each pixel in each frame get the relative luminance in a range of 0-255, select a char based on that, and set an ANSI color
		for frameIndex, frame := range frames{
			asciiList = append(asciiList, "")
			for y := range newHeight{
				for x := range newWidth{
					R,G,B,_ := frame.At(x,y).RGBA()
					red := uint8(R>>8)
					green := uint8(G>>8)
					blue := uint8(B>>8)
					//relativeLuminance := (0.2126 * float32(R)) + (0.715 * float32(G)) + (0.0722 * float32(B))
					ansi := "\x1b[38;2;" + strconv.FormatUint(uint64(red), 10) + ";" + strconv.FormatUint(uint64(green), 10) + ";" + strconv.FormatUint(uint64(blue), 10) + "m" 
					ansi += "██"
					asciiList[frameIndex] += ansi
				}
				asciiList[frameIndex] += "\033[0m\n"
			}
		}
		//Print the frame
		fmt.Println(asciiList[0])
		fmt.Println("\033[0m")
		//DELETE LATER. Save the first frame for testing purposes
			img, _ := jpeg.Decode(ReadFrameAsJpeg(filename, 0))
			file, _ := os.Create("..\\output.jpeg")
			_ = jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
			file, _ = os.Create("..\\output2.jpeg")
			_ = jpeg.Encode(file, frames[0], &jpeg.Options{Quality: 100})
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
		Output("pipe:", ffmpeg.KwArgs{"loglevel":"quiet", "vframes": 1, "format": "image2", "vcodec": "mjpeg"}).
		WithOutput(buf).
		Silent(true).
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