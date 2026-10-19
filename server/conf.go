package main

import "path/filepath"

// Puzzle Rows
const puzzleRows = 5

// Puzzle Columns
const puzzleCols = 5

// Puzzle Size
// 1.0 occupies as much space as possible,
// but the pieces will be large and cramped
const puzzleSize = 0.7

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
