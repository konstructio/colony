package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func FileFromURL(url, filename string) error {

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf(("error copying file: %w"), err)
	}

	return err
}
