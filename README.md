# vidToAscii

A CLI application to convert a video or gif into ASCII and play it in a terminal.

## Example
As seen in the bottom right, the video will scale to whatever size the terminal is at the time the command is run.

https://github.com/user-attachments/assets/0b8eabfe-c944-4d60-9382-f7756b4e4153

## Options
vidToAscii [flags]

|Flag|| Description |
| --- | ------------ | --------------------------------------------------------------------------------------------------- |
| -h |--help |  help for vidToAscii |
| -i *filepath*|--input *filepath* | file path of input video |
|  -b | --background |    use background colors instead of ascii characters. This makes the video look like pixel art |
|  -s | --save |  save the converted data as a txt that can be loaded with --load |
| -l | --load | load saved data created by --save. Must use -i to specify filepath of save |


## Setup
1. Install ffmpeg, ffprobe, and go

2. Download or clone repository

3. Navigate to the vidToAscii folder within the repository

4. Intstall the application.
   This will create an executable. On Windows it will also add the executable to the PATH variable.
```
go install
```
5. Run application
```
vidToAscii -i input.mp4
```
6. List all options
```
vidToAscii --help
```
