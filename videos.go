package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	buffer := &bytes.Buffer{}
	cmd.Stdout = buffer

	cmd.Run()

	type ffprobeOutput struct {
		Streams []struct {
			WidthPixels  int `json:"width"`
			HeightPixels int `json:"height"`
		} `json:"streams"`
	}

	dimensions := ffprobeOutput{}
	err := json.Unmarshal(buffer.Bytes(), &dimensions)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling bytes: %w", err)
	}

	width := dimensions.Streams[0].WidthPixels
	height := dimensions.Streams[0].HeightPixels

	ratio := width / height
	if ratio == 16/9 {
		return "16:9", nil
	}

	ratio = height / width
	if ratio == 16/9 {
		return "9:16", nil
	}

	return "other", nil
}

func getS3KeyPrefix(ratio string) string {
	switch ratio {
	case "16:9":
		return "landscape"
	case "9:16":
		return "portrait"
	default:
		return "other"
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := fmt.Sprintf("%s.processing", filePath)

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("error reformatting video for fast start: %w", err)
	}

	return outputFilePath, nil
}
