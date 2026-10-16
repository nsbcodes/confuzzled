package main

import (
	"errors"
	"os"
)

func main() {
	_, err := os.Stat(outputDir)
	if errors.Is(err, os.ErrNotExist) {
		generate()
	}

}
