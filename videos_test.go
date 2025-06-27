package main

import (
	"fmt"
	"testing"
)

func TestGetAspectRatio(t *testing.T) {
	cases := []struct {
		filepath      string
		expectedValue string
	}{
		{
			filepath:      "samples/boots-video-horizontal.mp4",
			expectedValue: "16:9",
		},
		{
			filepath:      "samples/boots-video-vertical.mp4",
			expectedValue: "9:16",
		},
	}

	for _, c := range cases {
		result, err := getVideoAspectRatio(c.filepath)
		if err != nil {
			t.Errorf("Error with file '%s', expected '%s', but got an error: %v", c.filepath, c.expectedValue, err)
		}

		if result != c.expectedValue {
			t.Errorf("Error with file '%s', expected '%s', but got '%s'", c.filepath, c.expectedValue, result)
		}

		fmt.Println(result)
	}
}
