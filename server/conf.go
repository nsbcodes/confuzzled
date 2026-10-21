package main

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
)

// These can be dynamically loaded during runtime
// by a file the user supplies through an argument
type Config struct {
	// Puzzle Rows
	puzzleRows int
	// Puzzle Columns
	puzzleCols int
	// Puzzle Size
	// 1.0 occupies as much space as possible,
	// but the pieces will be large and cramped
	puzzleSize float64
	// Margin for puzzle piece tabs (the knobs that stick out)
	// Note that this is a multiplier
	// Should be 1.0 < margin < 1.5
	margin float64
	// Determines what proportion of the margin should be
	// reserved for perturbations (waves) in each puzzle piece
	// Basically the amplitude of the perturbations
	perturb float64
	// Frequency of perturbation sine wave
	// Higher values produce more extreme waves
	perturbFrequency float64
}

func InitConfig() Config {
	c := new(Config)
	c.puzzleRows = 5
	c.puzzleCols = 5
	c.puzzleSize = 0.7
	c.margin = 1.3
	c.perturb = 0.3
	c.perturbFrequency = 0.02
	return *c
}

func (c *Config) LoadConfig() {
	// Check if config file exists first
	stat, err := os.Stat(configFile)
	if errors.Is(err, os.ErrNotExist) {
		log.Printf("No custom json config file found, using defaults.")
	} else if stat.IsDir() {
		log.Fatalf("Could not read json config file %s because it is a directory", configFile)
	} else if err != nil {
		log.Fatalf("Could not read json config file %s due to error \"%s\"", configFile, err)
	}

	// Reads the entire file into memory
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Error reading config file %s due to error \"%s\"", configFile, err)
		return
	}

	err = json.Unmarshal(configBytes, &c)
	if err != nil {
		log.Fatalf("Could not parse config file %s due to error \"%s\"", configFile, err)
	}
	log.Printf("Successfully read and parsed custom json config file provided!")
}

// The rest of these are compiled constants

// All server output will be stored in this folder
const outputDir = "out"

// Puzzle Piece Images Output Directory
const imageOutputDir = outputDir + string(filepath.Separator) + "img"

// Image File Location
const imageInputFile = "image.png"

// Puzzle Data File Location
const puzzleOutputFile = outputDir + string(filepath.Separator) + "puzzle.json"

// State Data File Location
const stateOutputFile = outputDir + string(filepath.Separator) + "state.json"

// Compiled Website Location
const serveFolder = "serve"

// Config file location
const configFile = "config.json"
