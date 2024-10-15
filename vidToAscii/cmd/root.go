package cmd

import (
	"bytes"
	"encoding/binary"
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

var save bool
var load bool
var input string
func init() {
	rootCmd.PersistentFlags().BoolVarP(&save, "save", "s", false, "save the converted data as a txt that can be loaded with --load")
	rootCmd.PersistentFlags().BoolVarP(&load, "load", "l", false, "load saved data created by --save")
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "file path of input video")
	rootCmd.MarkFlagRequired("input")
}

var rootCmd = &cobra.Command{
	Use: "vidToAscii",
	Short: "Convert a video into ascii and play it in the terminal",
	Run: func(cmd *cobra.Command, args []string) {
		_, err := os.Stat(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		filename := input
		
		var expectedFrametime time.Duration
		var frameCount int
		var originalHeight int
		var originalWidth int

		var asciiList []string
		var saveOut *os.File
		if save{
			saveOut, err := os.Create("save.txt")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			//Get the size and total frames of the input video
			originalWidth, originalHeight, frameCount, expectedFrametime = GetVideoInfo(filename)

			//The first 8 bytes are reserved to store the expectedFrametime
			err = binary.Write(saveOut, binary.LittleEndian, expectedFrametime.Nanoseconds())
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			Convert(originalWidth,originalHeight,frameCount,filename,saveOut)
			saveOut.Close()
			return
		}else if !load{
			//Get the size and total frames of the input video
			originalWidth, originalHeight, frameCount, expectedFrametime = GetVideoInfo(filename)
			asciiList = Convert(originalWidth,originalHeight,frameCount,filename,saveOut)
		}
		
		if load{
			saveIn,err := os.ReadFile(filename)
			if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
			}
			expectedFrametime = time.Duration(binary.LittleEndian.Uint64(saveIn[0:8]))
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			asciiList = strings.SplitAfter(string(saveIn[8:]), "\033[0m")
		}
		
		fmt.Printf("\033[2J\033[H")
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
			expectedTime := time.Duration((frameIndex + 1) * int(expectedFrametime))
			print(frame)
			if expectedTime - time.Since(startTime) < 0{
				dropFrame = true
			}
			time.Sleep(expectedTime - time.Since(startTime))
		}
		fmt.Printf("\033[J")
		print("\nTotal Dropped Frames ", totalDroppedFrames)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//Return width, height, total frames, and expectedFrametime
func GetVideoInfo(inFileName string) (int, int, int, time.Duration) {
	data, err := ffmpeg.Probe(inFileName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	duration,err := strconv.ParseFloat(vInfo.Streams[0].Duration,32)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	frames,err := strconv.Atoi(vInfo.Streams[0].Frames)
	if err != nil {
		framerateString := strings.Split(vInfo.Streams[0].Framerate, "/")
		framerateNumerator,err := strconv.ParseFloat(framerateString[0],32)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		framerateDenomonator, err := strconv.ParseFloat(framerateString[1],32)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		framerate := framerateNumerator / framerateDenomonator
		frames = int(framerate * duration)
	}
	
	expectedFrametime := time.Duration((duration / float64(frames))* 1000000000)  * time.Nanosecond

	return vInfo.Streams[0].Width, vInfo.Streams[0].Height, frames, expectedFrametime
}

func Convert(originalWidth int, originalHeight int, frameCount int, filename string, saveOut *os.File) []string{
	//Get the size of the terminal
	termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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
		fmt.Print("\033[J\033[HDecoding ", frameIndex + 1, "/", frameCount)
	}

	//For each frame make a blank image of the new size and then draw over it
	for frameIndex, frame := range frames{
		resizedImage:= image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
		//Add flag for different quality options https://pkg.go.dev/golang.org/x/image/draw#pkg-variables
		draw.BiLinear.Scale(resizedImage, resizedImage.Rect, frame, frame.Bounds(), draw.Over, nil)
		frames[frameIndex] = resizedImage
		fmt.Print("\033[J\033[HScaling ", frameIndex + 1, "/", frameCount)
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
		ansiBuilder.WriteString("\033[H")
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
		
		if save{
			fmt.Fprint(saveOut, ansiBuilder.String())
		} else{
			asciiList[frameIndex] = ansiBuilder.String()
		}
		fmt.Print("\033[J\033[HPixels ", frameIndex + 1, "/", frameCount)
	}
	return asciiList
}