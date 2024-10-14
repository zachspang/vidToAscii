package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/term"

	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		filename := "..\\inputComplex.mp4"
		
		//Get the size of the terminal
		termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		//Get the size and total frames of the input video
		originalWidth, originalHeight, frameCount, duration := GetVideoInfo(filename)

		//Set the new width and height based on the terminal size
		newWidth := termWidth / 2
		newHeight := (int((float32(newWidth) / float32(originalWidth)) * float32(originalHeight)))
		if newHeight > termHeight{
			newHeight = termHeight
			newWidth = (int((float32(newHeight) / float32(originalHeight)) * float32(originalWidth)))
		}
		// fmt.Println("TW", termWidth, "TH", termHeight)
		// fmt.Println("NW", newWidth, "NH", newHeight)
		frames := make([]image.Image, frameCount) 

		//Read all frames as a single byte string
		reader := bytes.NewBuffer(nil)
		ReadFramesAsJpeg(filename, frameCount, reader)
		//All jpegs end in an EOI marker, 0xff 0xd9. Split the byte string into seperate byte strings for each frame
		framesAsBytes := bytes.SplitAfter(reader.Bytes(), []byte{0xff, 0xd9})
		
		//Decode each frame's byte string into an image.Image
		for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
			frame, err := jpeg.Decode(bytes.NewReader(framesAsBytes[frameIndex]))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			frames[frameIndex] = frame
			fmt.Print("\033[J\033[HDecoding ", frameIndex, "/", frameCount)
		}

		//For each frame make a blank image of the new size and then draw over it
		for frameIndex, frame := range frames{
			resizedImage:= image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
			//Add flag for different quality options https://pkg.go.dev/golang.org/x/image/draw#pkg-variables
			draw.BiLinear.Scale(resizedImage, resizedImage.Rect, frame, frame.Bounds(), draw.Over, nil)
			frames[frameIndex] = resizedImage
			fmt.Print("\033[J\033[HScaling ", frameIndex, "/", frameCount)
		}

		//Slice with the ASCII representation for a frame. Each frame is stored as a string with escape sequences for color and newlines
		asciiList := make([]string, frameCount)

		var charSet []string
		//eventually add flag to swap between these
		if true{
			charSet = strings.Split(" .:=+*#%@", "")
		}else{
			charSet = strings.Split("░▒▓█", "")
		}

		//For each pixel in each frame get the relative luminance in a range of 0-255, select a char based on that, and set an ANSI color
		for frameIndex, frame := range frames{
			var ansiBuilder strings.Builder
			for y := range newHeight{
				for x := range newWidth{
					R,G,B,_ := frame.At(x,y).RGBA()
					red := uint8(R>>8)
					green := uint8(G>>8)
					blue := uint8(B>>8)
					relativeLuminance := (0.2126 * float64(red)) + (0.715 * float64(green)) + (0.0722 * float64(blue))
					ansiBuilder.WriteString("\x1b[38;2;" + strconv.FormatUint(uint64(red), 10) + ";" + strconv.FormatUint(uint64(green), 10) + ";" + strconv.FormatUint(uint64(blue), 10) + "m") 
					
					charIndex := int(math.Round(relativeLuminance / (255 / float64(len(charSet) - 1)))) 
					ansiBuilder.WriteString(charSet[charIndex] + charSet[charIndex]) 
					
				}
				if y == newHeight - 1{
					ansiBuilder.WriteString("\033[0m")
				}else{
					ansiBuilder.WriteString("\n")
				}
				
			}
			asciiList[frameIndex] = ansiBuilder.String()
			fmt.Print("\033[J\033[HPixels ", frameIndex, "/", frameCount)
		}

		fmt.Printf("\033[2J\033[H")
		frametime := time.Duration((duration / float32(frameCount))* 1000)  * time.Millisecond
		totalDroppedFrames := 0
		dropFrame := false
		startTime := time.Now()
		//This loop will print one frame then sleep until the next frame is meant to be shown
		//If it can't keep up it will drop the next frame
		for frameIndex, frame := range asciiList{
			if dropFrame{
				dropFrame = false
				totalDroppedFrames++
				continue
			}
			expectedTime := time.Duration((frameIndex + 1) * int(frametime))
			print("\033[H",frame)
			if expectedTime - time.Since(startTime) < 0{
				dropFrame = true
			}
			time.Sleep(expectedTime - time.Since(startTime))
		}
		fmt.Printf("\033[J")
		print("\nTotal Dropped Frames ", totalDroppedFrames)
		//Print the frame
		// fmt.Println(asciiList[0])
		// fmt.Println("\033[0m")
		// //DELETE LATER. Save the first frame for testing purposes
		// 	img, _ := jpeg.Decode(ReadFrameAsJpeg(filename, 0))
		// 	file, _ := os.Create("..\\output.jpeg")
		// 	_ = jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
		// 	file, _ = os.Create("..\\output2.jpeg")
		// 	_ = jpeg.Encode(file, frames[0], &jpeg.Options{Quality: 100})
	},
}
  
func Execute() {
	if err := rootCmd.Execute(); err != nil {
	  fmt.Fprintln(os.Stderr, err)
	  os.Exit(1)
	}
}

//Set the passed in byte.Buffer to a byte string containing the jpeg data for every frame
func ReadFramesAsJpeg(inFileName string, frameCount int, reader *bytes.Buffer) {
	err := ffmpeg.Input(inFileName).
		Output("pipe:", ffmpeg.KwArgs{"loglevel": "quiet", "vframes": frameCount, "update": 1, "format": "image2", "vcodec": "mjpeg"}).
		WithOutput(reader, os.Stdout).
		Silent(true).
		Run()
	if err != nil {
		panic(err)
	}
}

//Return width, height, total frames, and duration of video
func GetVideoInfo(inFileName string) (int, int, int, float32) {
	data, err := ffmpeg.Probe(inFileName)
	if err != nil {
		panic(err)
	}

	type VideoInfo struct {
		Streams []struct {
			Width     int
			Height    int
			Frames string `json:"nb_frames"`
			Duration string `json:"duration"`
			Framerate string `json:"r_frame_rate"`
		} `json:"streams"`
	}
	vInfo := &VideoInfo{}
	err = json.Unmarshal([]byte(data), vInfo)
	if err != nil {
		panic(err)
	}

	duration,err := strconv.ParseFloat(vInfo.Streams[0].Duration,32)
	if err != nil {
		panic(err)
	}

	frames,err := strconv.Atoi(vInfo.Streams[0].Frames)
	if err != nil {
		framerate,err := strconv.ParseFloat(strings.Split(vInfo.Streams[0].Framerate, "/")[0],32)
		if err != nil {
			panic(err)
		}
		frames = int(framerate * duration)
	}
	
	return vInfo.Streams[0].Width, vInfo.Streams[0].Height, frames, float32(duration)
}