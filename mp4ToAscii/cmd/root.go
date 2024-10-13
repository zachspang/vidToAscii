package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"os"
	"strconv"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/term"

	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

var rootCmd = &cobra.Command{
	Run: func(cmd *cobra.Command, args []string) {
		filename := "..\\inputLong.mp4"
		
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

		//Slice with the ASCII representation for a frame. Each frame is stored as a string with escape sequences for color and newlines
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
					relativeLuminance := (0.2126 * float64(red)) + (0.715 * float64(green)) + (0.0722 * float64(blue))
					ansi := "\x1b[38;2;" + strconv.FormatUint(uint64(red), 10) + ";" + strconv.FormatUint(uint64(green), 10) + ";" + strconv.FormatUint(uint64(blue), 10) + "m" 
					
					var charSet []string
					//eventually add flag to swap between these
					if true{
						charSet = []string{".", ",", ":","-", "=", "+", "*", "#", "%", "@"}
					}else{
						charSet = []string{"░", "▒", "▓", "█"}
					}
					charIndex := int(math.Round(relativeLuminance / (255 / float64(len(charSet) - 1)))) 
					ansi += charSet[charIndex] + charSet[charIndex ]
					asciiList[frameIndex] += ansi
				}
				if y == newHeight - 1{
					asciiList[frameIndex] += "\033[0m"
				}else{
					asciiList[frameIndex] += "\033[0m\n"
				}
			}
		}

		fmt.Printf("\033[2J\033[H")
		frametime := duration / float32(frameCount)
		for _, frame := range asciiList{
			fmt.Print(frame)
			time.Sleep(time.Duration(frametime * 1000) * time.Millisecond)
			fmt.Printf("\033[J\033[H")
		}
		fmt.Printf("\033[J")
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
	duration,err := strconv.ParseFloat(vInfo.Streams[0].Duration,32)
	if err != nil {
		panic(err)
	}
	return vInfo.Streams[0].Width, vInfo.Streams[0].Height, frames, float32(duration)
}