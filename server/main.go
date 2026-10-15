package main

import (
	"fmt"
	_ "image/jpeg"
	"log"
	"os"
	"path/filepath"
)

// Puzzle Piece Images Output Directory
const outputDir = "out"

// Margin for puzzle piece tabs (the knobs that stick out)
// Note that this is a multiplier
// Should be 1.0 < margin < 1.5
const margin = 1.2

func main() {
	// Create puzzle object
	puzzle := Puzzle{rows: 5, cols: 5}

	// Can be jpg/jpeg/png
	ic := loadImage("image.png")
	fmt.Printf("Successfully loaded image into a %dx%d array.\n", ic.width, ic.height)

	// Downscale image so that there are no remaining pixels
	// Note that this might change the aspect ratio very slightly
	// i.e. ic.width  % puzzle.cols == 0
	//      ic.height % puzzle.rows == 0
	ic.Downsize(ic.width-(ic.width%puzzle.cols), ic.height-(ic.height%puzzle.rows))
	fmt.Printf("Successfully downsized image into %dx%d.\n", ic.width, ic.height)

	gridColWidth := int((1.0 / float64(puzzle.cols)) * float64(ic.width))
	gridRowHeight := int((1.0 / float64(puzzle.rows)) * float64(ic.height))

	// Allocate puzzle pieces
	puzzle.pieces = make(map[string]*Piece)
	for row := range puzzle.rows {
		for col := range puzzle.cols {
			puzzle.pieces[ID(row, col)] = &Piece{}
		}
	}

	// Set tabs and blanks for each puzzle piece
	for row := range puzzle.rows {
		for col := range puzzle.cols {
			id := ID(row, col)

			// The top and left connectors are based on previous pieces
			// The bottom and right connectors are randomly-generated
			// This ensures that the whole puzzle looks completely random,
			// while still maintaining connectivity between pieces.

			// Set the top connector
			if row == 0 {
				// Edge rows (top side of the puzzle)
				puzzle.pieces[id].topConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Reference the bottom connector of the top piece and invert it
				puzzle.pieces[id].topConnector = puzzle.pieces[ID(row-1, col)].bottomConnector.Invert()
			}

			// Set the left connector
			if col == 0 {
				// Edge rows (left side of the puzzle)
				puzzle.pieces[id].leftConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Reference the right connector of the left piece and invert it
				puzzle.pieces[id].leftConnector = puzzle.pieces[ID(row, col-1)].rightConnector.Invert()
			}

			// Set the bottom connector
			if row == puzzle.rows-1 {
				// Edge rows (bottom side of the puzzle)
				puzzle.pieces[id].bottomConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Randomize the bottom connector
				puzzle.pieces[id].bottomConnector = RandomConnection()
			}

			// Set the right connector
			if col == puzzle.cols-1 {
				// Edge rows (right side of the puzzle)
				puzzle.pieces[id].rightConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Randomize the right connector
				puzzle.pieces[id].rightConnector = RandomConnection()
			}
		}
	}

	marginWidth := int(float64(gridColWidth) * (margin - 1))

	// Split main image and render the tabs/blanks mask
	for row := range puzzle.rows {
		for col := range puzzle.cols {
			// Added to the bottom, top, left, and right of the puzzle piece
			pos1 := Coords{gridColWidth * col, gridRowHeight * row}
			pos2 := Coords{gridColWidth * (col + 1), gridRowHeight * (row + 1)}
			split_ic := ic.Split(pos1, pos2, marginWidth)
			split_ic.DrawConnections(*puzzle.pieces[ID(row, col)], marginWidth)

			puzzle.pieces[ID(row, col)].image = &split_ic
		}
	}

	// // Apply noise perturbation to each piece
	// for row := range puzzle.rows - 1 {
	// 	for col := range puzzle.cols - 1 {

	// 	}
	// }

	// Export all several pngs
	for row := range puzzle.rows {
		for col := range puzzle.cols {
			err := os.MkdirAll(outputDir, os.ModePerm)
			if err != nil {
				log.Fatalf("Could not create image output directory \"%s\" due to error \"%s\"", outputDir, err.Error())
			}
			id := ID(row, col)
			err = puzzle.pieces[ID(row, col)].image.ExportPNG(filepath.Join(outputDir, id+".png"))
			if err != nil {
				log.Fatalf("Could not export split image due to error \"%s\"", err.Error())
			}
		}
	}
}
