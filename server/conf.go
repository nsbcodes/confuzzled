package main

import "path/filepath"

// All server output will be stored in this folder
const outputDir = "out"

// Puzzle Piece Images Output Directory
const imageOutputDir = outputDir + string(filepath.Separator) + "img"

// Image File Location
const imageInputFile = "image.png"

// Puzzle Data File Location
const puzzleOutputFile = outputDir + string(filepath.Separator) + "puzzle.json"

// Margin for puzzle piece tabs (the knobs that stick out)
// Note that this is a multiplier
// Should be 1.0 < margin < 1.5
const margin = 1.3

// Determines what proportion of the margin should be
// reserved for perturbations (waves) in each puzzle piece
// Basically the amplitude of the perturbations
const perturb = 0.3

// Frequency of perturbation sine wave
// Higher values produce more extreme waves
const perturbFrequency = 0.02
