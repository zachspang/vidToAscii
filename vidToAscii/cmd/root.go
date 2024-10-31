package cmd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var save bool
var load bool
var input string
var background bool
func init() {
	rootCmd.PersistentFlags().BoolVarP(&save, "save", "s", false, "save the converted data as a txt that can be loaded with --load")
	rootCmd.PersistentFlags().BoolVarP(&load, "load", "l", false, "load saved data created by --save. Must use -i to specify filepath of save")
	rootCmd.Flags().StringVarP(&input, "input", "i", "", "file path of input video")
	rootCmd.MarkFlagRequired("input")
	rootCmd.PersistentFlags().BoolVarP(&background, "background", "b", false, "use background colors instead of ascii characters. This makes the video look like pixel art")
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
			Convert(originalWidth,originalHeight,frameCount,filename,expectedFrametime,saveOut)
			saveOut.Close()
			return
		}else if !load{
			//Get the size and total frames of the input video
			originalWidth, originalHeight, frameCount, expectedFrametime = GetVideoInfo(filename)
			Convert(originalWidth,originalHeight,frameCount,filename,expectedFrametime,saveOut)
		}
		
		if load{
			loadSave(filename)
		}
	},
}
  
func Execute() {
	if err := rootCmd.Execute(); err != nil {
	  fmt.Fprintln(os.Stderr, err)
	  os.Exit(1)
	}
}

//Set the passed in byte.Buffer to a byte string containing the jpeg data for every frame scaled to the size of the terminal
func ReadFramesAsJpeg(inFileName string, frameCount int, newWidth int, newHeight int, reader *bytes.Buffer) {
	println("Waiting on FFMPEG to split video into frames")
	err := ffmpeg.Input(inFileName).
		Output("pipe:", ffmpeg.KwArgs{"loglevel": "quiet", "vframes": frameCount, "update": 1, "q":"1", "qmin": 1, "qmax": 1, "vf":"scale=w=" + strconv.Itoa(newWidth) + ":h=" + strconv.Itoa(newHeight) +":flags=lanczos" ,"format": "image2", "vcodec": "mjpeg"}).
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

func Convert(originalWidth int, originalHeight int, frameCount int, filename string, expectedFrametime time.Duration, saveOut *os.File) {

	//Define the set of characters to represent different luminosity pixels
	var charSet []string
	if background{
		charSet = []string{" "}
	}else{
		charSet = strings.Split(" .:=+*#%@", "")
	}

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

	//Read all frames as bytes
	reader := bytes.NewBuffer(nil)
	ReadFramesAsJpeg(filename, frameCount, newWidth, newHeight, reader)
	//All jpegs end in an EOI marker, 0xff 0xd9. Split the byte string into seperate byte strings for each frame
	framesAsBytes := bytes.SplitAfter(reader.Bytes(), []byte{0xff, 0xd9})
	
	totalDroppedFrames := 0
	dropFrame := false
	startTime := time.Now()
	fmt.Printf("\033[2J\033[H")

	//Decode each frame and print it
	for frameIndex := 0; frameIndex < frameCount; frameIndex++ {
		frame, err := jpeg.Decode(bytes.NewReader(framesAsBytes[frameIndex]))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		
		//Loop through all the pixels of the frame and turn it into a string
		var ansiBuilder strings.Builder
		ansiBuilder.WriteString("\033[H")
		for y := range newHeight{
			for x := range newWidth{
				R,G,B,_ := frame.At(x,y).RGBA()
				red := uint8(R>>8)
				green := uint8(G>>8)
				blue := uint8(B>>8)
				relativeLuminance := (0.2126 * float64(red)) + (0.715 * float64(green)) + (0.0722 * float64(blue))
				if background{
					ansiBuilder.WriteString("\x1b[48;2;" + strconv.FormatUint(uint64(red), 10) + ";" + strconv.FormatUint(uint64(green), 10) + ";" + strconv.FormatUint(uint64(blue), 10) + "m") 
				} else{
					ansiBuilder.WriteString("\x1b[38;2;" + strconv.FormatUint(uint64(red), 10) + ";" + strconv.FormatUint(uint64(green), 10) + ";" + strconv.FormatUint(uint64(blue), 10) + "m") 
				}
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
			//Print one frame then sleep until the next frame is meant to be shown
			//If it can't keep up it will drop the next frame
			if dropFrame{
				dropFrame = false
				totalDroppedFrames++
				
			}else {
				expectedTime := time.Duration((frameIndex + 1) * int(expectedFrametime))
				print(ansiBuilder.String())
				if expectedTime - time.Since(startTime) < 0{
					dropFrame = true
				}
				time.Sleep(expectedTime - time.Since(startTime))
			}
		}
	}
	if !save{
		fmt.Printf("\033[J")
		print("\nTotal Dropped Frames ", totalDroppedFrames)	
	}
}

func loadSave(filename string){
	saveIn,err := os.ReadFile(filename)
	if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
	}
	expectedFrametime := time.Duration(binary.LittleEndian.Uint64(saveIn[0:8]))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	asciiList := strings.SplitAfter(string(saveIn[8:]), "\033[0m")

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
}
