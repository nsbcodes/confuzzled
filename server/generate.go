package main

import (
	"encoding/json"
	_ "image/jpeg"
	"log"
	"math/rand/v2"
	"os"
	"path/filepath"
)

func generatePuzzle(config Config) {
	// Scaffold directory
	os.MkdirAll(imageOutputDir, os.ModePerm)

	// Create puzzle object
	puzzle := Puzzle{Rows: config.puzzleRows, Cols: config.puzzleCols}

	// Can be jpg/jpeg/png
	ic := loadImage(imageInputFile)
	log.Printf("Successfully loaded image into a %dx%d array.\n", ic.width, ic.height)

	// Downscale image so that there are no remaining pixels
	// Note that this might change the aspect ratio very slightly
	// i.e. ic.width  % puzzle.cols == 0
	//      ic.height % puzzle.rows == 0
	ic.Downsize(ic.width-(ic.width%puzzle.Cols), ic.height-(ic.height%puzzle.Rows))
	log.Printf("Successfully downsized image to %dx%d.\n", ic.width, ic.height)
	puzzle.Width = ic.width
	puzzle.Height = ic.height

	// Enforce 4:3 aspect for the HTML Canvas for our client
	puzzle.CanvasHeight = int(float64(puzzle.Height) * (1.0 / config.puzzleSize))
	puzzle.CanvasWidth = int(float64(puzzle.CanvasHeight) * (4.0 / 3.0))

	gridColWidth := int(float64(ic.width) / float64(puzzle.Cols))
	gridRowHeight := int(float64(ic.height) / float64(puzzle.Rows))
	puzzle.PieceWidth = gridColWidth
	puzzle.PieceHeight = gridRowHeight

	// Allocate puzzle pieces
	puzzle.pieces = make(map[string]*Piece)
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
			puzzle.pieces[ID(row, col)] = &Piece{}
		}
	}

	// Set tabs and blanks for each puzzle piece
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
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
			if row == puzzle.Rows-1 {
				// Edge rows (bottom side of the puzzle)
				puzzle.pieces[id].bottomConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Randomize the bottom connector
				puzzle.pieces[id].bottomConnector = RandomConnection()
			}

			// Set the right connector
			if col == puzzle.Cols-1 {
				// Edge rows (right side of the puzzle)
				puzzle.pieces[id].rightConnector = Connection{connectionType: ConnectionNone}
			} else {
				// Randomize the right connector
				puzzle.pieces[id].rightConnector = RandomConnection()
			}
		}
	}

	margin := int(float64(gridColWidth) * (config.margin - 1))
	puzzle.Margin = margin

	// Split main image and fill puzzle.pieces[id].image
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
			// Added to the bottom, top, left, and right of the puzzle piece
			pos1 := Coords{gridColWidth * col, gridRowHeight * row}
			pos2 := Coords{gridColWidth * (col + 1), gridRowHeight * (row + 1)}
			split_ic := ic.Split(pos1, pos2, margin)

			puzzle.pieces[ID(row, col)].image = &split_ic
		}
	}

	// Apply noise perturbation to each piece
	// Our noise is just a sine wave sampled and multiplied randomly

	// Left/Right Side Perturbation
	for col := range puzzle.Cols - 1 {
		xs := rand.Float64() * config.perturbFrequency
		c := rand.Float64() * 10.0
		a := int(float64(margin) * config.perturb)

		for row := range puzzle.Rows {
			imgLeft := puzzle.pieces[ID(row, col)].image
			imgRight := puzzle.pieces[ID(row, col+1)].image

			span := imgLeft.height - 2*margin

			for y := range span {
				// Fade perturbation toward both corners
				taper := 1.0
				if margin > 0 {
					taper = float64(y) / float64(margin)
					end := float64(span-1-y) / float64(margin)
					if end < taper {
						taper = end
					}
					if taper > 1 {
						taper = 1
					}
					if taper < 0 {
						taper = 0
					}
				}

				w := int(Sin(float64(row*gridRowHeight+y), float64(a)*taper, xs, c))

				if w > 0 {
					for n := range w {
						SetA(&imgLeft.pixels[y+margin][imgLeft.width-1-margin+n], 255)
						SetA(&imgRight.pixels[y+margin][margin-1+n], 0)
					}
				} else {
					for n := range -w {
						SetA(&imgLeft.pixels[y+margin][imgLeft.width-1-margin-n], 0)
						SetA(&imgRight.pixels[y+margin][margin-1-n], 255)
					}
				}
			}
		}
	}

	// Top/Bottom Side Perturbation
	for row := range puzzle.Rows - 1 {
		xs := rand.Float64() * config.perturbFrequency
		c := rand.Float64() * 10.0
		a := int(float64(margin) * config.perturb)

		for col := range puzzle.Cols {
			imgTop := puzzle.pieces[ID(row, col)].image
			imgBottom := puzzle.pieces[ID(row+1, col)].image

			span := imgTop.width - 2*margin

			for x := range span {
				// Fade perturbation toward both corners
				taper := 1.0
				if margin > 0 {
					taper = float64(x) / float64(margin)
					end := float64(span-1-x) / float64(margin)
					if end < taper {
						taper = end
					}
					if taper > 1 {
						taper = 1
					}
					if taper < 0 {
						taper = 0
					}
				}

				w := int(Sin(float64(col*gridColWidth+x), float64(a)*taper, xs, c))

				px := x + margin

				if w > 0 {
					for n := range w {
						SetA(&imgTop.pixels[imgTop.height-1-margin+n][px], 255)
						SetA(&imgBottom.pixels[margin-1+n][px], 0)
					}
				} else {
					for n := range -w {
						SetA(&imgTop.pixels[imgTop.height-1-margin-n][px], 0)
						SetA(&imgBottom.pixels[margin-1-n][px], 255)
					}
				}
			}
		}
	}

	// Draw puzzle piece tabs and blanks
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
			puzzle.pieces[ID(row, col)].DrawConnections(margin)
		}
	}

	// Export all several pngs
	for row := range puzzle.Rows {
		for col := range puzzle.Cols {
			err := os.MkdirAll(imageOutputDir, os.ModePerm)
			if err != nil {
				log.Fatalf("Could not create image output directory \"%s\" due to error \"%s\"", imageOutputDir, err.Error())
			}
			id := ID(row, col)
			err = puzzle.pieces[ID(row, col)].image.ExportPNG(filepath.Join(imageOutputDir, id+".png"))
			if err != nil {
				log.Fatalf("Could not export split image due to error \"%s\"", err)
			}
		}
	}

	// Export json
	jsonB, err := json.Marshal(puzzle)
	if err != nil {
		log.Fatalf("Error marshaling to JSON due to error \"%s\"", err)
	}
	err = os.WriteFile(puzzleOutputFile, jsonB, os.ModePerm)
	if err != nil {
		log.Fatalf("Error writing JSON file due to error \"%s\"", err)
	}

	log.Println("All puzzle pieces generatedly correctly!")
}
