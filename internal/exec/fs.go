package exec

import (
	"fmt"
	"os"
	"path/filepath"
)

func CreateDirIfNotExist(dir string) error {
	if err := os.MkdirAll(dir, 0o777); err != nil {
		return fmt.Errorf("unable to create directory %q: %w", dir, err)
	}

	return nil
}

func DeleteDirectory(location string) error {
	if err := os.RemoveAll(location); err != nil {
		return fmt.Errorf("failed to remove directory %q: %w", location, err)
	}
	return nil
}

func ReadFilesInDir(dir string) ([]string, error) {
	//nolint:prealloc // We don't know the number of files in the directory
	var templateFiles []string

	// Open the directory
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open directory: %w", err)
	}

	// Loop through each file in the directory
	for _, file := range files {
		if file.IsDir() { // Check if it's a regular file
			continue
		}

		filePath := filepath.Join(dir, file.Name())
		content, err := os.ReadFile(filePath) // Read file content
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
		templateFiles = append(templateFiles, string(content))
	}

	return templateFiles, nil
}
