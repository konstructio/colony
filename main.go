package main

import (
	"fmt"
	"os"

	"github.com/konstructio/colony/cmd"
)

func main() {
	if err := cmd.GetRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
