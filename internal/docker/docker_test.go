package docker

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/konstructio/colony/internal/logger"
)

func requireNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("error = %v", err)
	}
}

func Test_waitUntilFileExists(t *testing.T) {
	// Create a temporary directory
	dir, err := os.MkdirTemp("", "test")
	requireNoError(t, err)
	defer os.RemoveAll(dir)

	// Define the filename
	filename := fmt.Sprintf("%s/testfile", dir)

	// Set up a goroutine to create the file after 250 ms
	go func() {
		time.Sleep(250 * time.Millisecond)
		_, err := os.Create(filename)
		requireNoError(t, err)
	}()

	// Create a logger
	log := logger.New(logger.Debug)

	// Call waitForFile2 with an interval of 50 ms and a timeout of 1 second
	err = waitUntilFileExists(log, filename, 50*time.Millisecond, 1*time.Second)
	if err != nil {
		t.Fatalf("waitForFile2() error = %v", err)
	}
}
