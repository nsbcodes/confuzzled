package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"

	"gonum.org/v1/gonum/interp"
)

// Coords for image manipulation use
type Coords struct {
	x int
	y int
}

// Holds bitmap data for image along with width and height.
// Remember that the y-cords are flipped
// (i.e., a small y-cord is on the top, big y-cord is bottom on the image).
// Also note that a pixel is adressed by pixels[y][x], not pixels[x][y].
type ImageContainer struct {
	pixels [][]color.Color
	width  int
	height int
}

// Loads and decodes image, outputs it as a pixel array
func loadImage(src string) (ic *ImageContainer) {
	// Open image file
	file, err := os.Open(src)
	if err != nil {
		log.Fatalf("Error opening file: %v\n", err)
	}
	defer file.Close()

	// Decode image file
	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatalf("Error decoding image: %v\n", err)
		return
	}

	// Convert to 2d Array
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Properly initialize output
	ic = &ImageContainer{
		width:  width,
		height: height,
	}
	ic.FillBlankPixels()

	// Fill in image
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ic.pixels[y][x] = color.NRGBAModel.Convert(
				img.At(bounds.Min.X+x, bounds.Min.Y+y),
			)
		}
	}

	return ic
}

// Initializes all pixels according to ic.width and ic.height
func (ic *ImageContainer) FillBlankPixels() {
	ic.pixels = make([][]color.Color, ic.height)
	for i := range ic.pixels {
		ic.pixels[i] = make([]color.Color, ic.width)
	}
}

// This downscales the image such that there are no extra pixels when Split is called
func (ic *ImageContainer) Downsize(new_w int, new_h int) {
	if new_w != ic.width || new_h != ic.height {
		scale_w := float64(ic.width) / float64(new_w)
		scale_h := float64(ic.height) / float64(new_h)
		new_ic := &ImageContainer{
			width:  new_w,
			height: new_h,
		}
		new_ic.FillBlankPixels()
		for y := range new_ic.pixels {
			for x := range new_ic.pixels[y] {
				new_ic.pixels[y][x] = ic.pixels[int(float64(y)*scale_h)][int(float64(x)*scale_w)]
			}
		}
		ic.width = new_ic.width
		ic.height = new_ic.height
		ic.pixels = new_ic.pixels
	}
}

// Splits image into a smaller rectangular section
// with optional padding margins (transparent with color data)
// using top left and bottom right bounds.
// Make sure that the argument order is preserved.
// Lower bound is inclusive, upper bound is exclusive.
// If either bounds sample pixels that do not exist, make them transparent
func (ic *ImageContainer) Split(topLeft Coords, bottomRight Coords, margin int) ImageContainer {
	topLeft_margin := Coords{topLeft.x - margin, topLeft.y - margin}
	bottomRight_margin := Coords{bottomRight.x + margin, bottomRight.y + margin}
	split_ic := ImageContainer{
		width:  bottomRight_margin.x - topLeft_margin.x,
		height: bottomRight_margin.y - topLeft_margin.y,
	}
	split_ic.FillBlankPixels()

	// Iterate over every pixel of the new image
	// y and x are not relative to the source image ic,
	// they are to be used for split_ic instead
	for y := range split_ic.height {
		for x := range split_ic.width {
			// Get the source image's pixel relative to the split image's iterator
			// Use this on ic instead of split_ic
			origX := topLeft_margin.x + x
			origY := topLeft_margin.y + y

			// See if orig coords are within bounds
			if origY >= 0 && origY < len(ic.pixels) && origX >= 0 && origX < len(ic.pixels[origY]) {
				split_ic.pixels[y][x] = ic.pixels[origY][origX]
				// Make the pixel transparent with color values
				// This region is for the tabs (part sticking out of a puzzle piece)
				// We can optionally flip this pixel to be opaque later
				if x < margin || x > split_ic.width-1-margin || y < margin || y > split_ic.height-1-margin {
					SetA(&split_ic.pixels[y][x], 0)
				}
			} else {
				// Else make the pixel completely transparent with no color values
				split_ic.pixels[y][x] = color.Transparent
			}
		}
	}

	return split_ic
}

// Export PNG at path provided
// Note that the parent directory should exist
func (ic *ImageContainer) ExportPNG(filename string) error {
	img := image.NewRGBA(image.Rect(0, 0, ic.width, ic.height))

	for y := 0; y < ic.height; y++ {
		for x := 0; x < ic.width; x++ {
			img.Set(x, y, ic.pixels[y][x])
		}
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

type TabGenerator struct {
	curve interp.PiecewiseCubic
	c     float64
}

func semicircle(x float64) float64 {
	return math.Sqrt(0.25 - (x-0.5)*(x-0.5))
}

func semicircleDerivative(x float64) float64 {
	return (0.5 - x) / semicircle(x)
}

// Generates an interpolated piecewise func that
// resembles the horizontal tab shape of a puzzle piece
// using a semicircle split at random x=c {0.1<c<0.4}
// {0<x<c} is filled in using cubic interpolation
// to the point (0, b)
//
// b -> [0.10, 0.40]
// c -> [0.05, 0.45]
//
// Paste this into Desmos to see how the shape looks like:
//
// f\left(x\right)=\sqrt{\left(\frac{1}{2}\right)^{2}-\left(x-0.5\right)^{2}}\left\{0<x<1\right\}
// g\left(x\right)=f\left(x\right)\left\{x>c\right\}
// c=0.2
// y=b+\frac{3(f(c)-b)-cf'(c)}{c^{2}}x^{2}+\frac{cf'(c)-2(f(c)-b)}{c^{3}}x^{3}\left\{0<x<c\right\}
// b=0.1
func MakeTabGenerator(b float64, c float64) TabGenerator {
	p := TabGenerator{c: c}
	p.curve.FitWithDerivatives(
		[]float64{0, c},
		[]float64{b, semicircle(c)},
		[]float64{0, semicircleDerivative(c)},
	)
	return p
}

// x must be (inclusive) between 0 and 1
// returns y value between (inclusive) 0 and 0.5
func (p *TabGenerator) Get(x float64) float64 {
	if x >= p.c {
		return semicircle(x)
	}
	return p.curve.Predict(x)
}

// Change color type to NRGBA and set alpha of pixel
func SetA(a *color.Color, b uint8) {
	c := (*a).(color.NRGBA)
	c.A = b
	*a = c
}

// Generates tabs/blanks for each image
// Intended to only be called on split image containers
// with transparent margins that contain color data (nRGBA)
// See TabGenerator for the shape
func (p *Piece) DrawConnections(margin int) {
	if p.topConnector.connectionType == ConnectionTab {
		for row := range margin {
			w := p.topConnector.tabGenerator.Get(1.0 - (float64(row) / float64(margin-1)))
			lb := (p.image.width / 2) - int(w*float64(margin))
			ub := (p.image.width / 2) + int(w*float64(margin))
			for col := lb; col <= ub; col++ {
				SetA(&p.image.pixels[row][col], 255)
				SetA(&p.image.pixels[row+margin][col], 255)
			}
		}
	} else if p.topConnector.connectionType == ConnectionBlank {
		for row := range margin {
			w := p.topConnector.tabGenerator.Get((float64(row) / float64(margin-1)))
			lb := (p.image.width / 2) - int(w*float64(margin))
			ub := (p.image.width / 2) + int(w*float64(margin))
			for col := range p.image.width {
				if col > lb && col < ub {
					SetA(&p.image.pixels[row+margin][col], 0)
					SetA(&p.image.pixels[row][col], 0)
				}
			}
		}
	}

	if p.bottomConnector.connectionType == ConnectionTab {
		for row := range margin {
			w := p.bottomConnector.tabGenerator.Get(1.0 - (float64(row) / float64(margin-1)))
			lb := (p.image.width / 2) - int(w*float64(margin))
			ub := (p.image.width / 2) + int(w*float64(margin))
			for col := lb; col <= ub; col++ {
				SetA(&p.image.pixels[p.image.height-1-row][col], 255)
				SetA(&p.image.pixels[p.image.height-margin-row][col], 255)
			}
		}
	} else if p.bottomConnector.connectionType == ConnectionBlank {
		for row := range margin {
			w := p.bottomConnector.tabGenerator.Get((float64(row) / float64(margin-1)))
			lb := (p.image.width / 2) - int(w*float64(margin))
			ub := (p.image.width / 2) + int(w*float64(margin))
			for col := range p.image.width {
				if col > lb && col < ub {
					SetA(&p.image.pixels[p.image.height-margin-row][col], 0)
					SetA(&p.image.pixels[p.image.height-1-row][col], 0)
				}
			}
		}
	}

	if p.rightConnector.connectionType == ConnectionTab {
		for col := range margin {
			w := p.rightConnector.tabGenerator.Get((float64(col) / float64(margin-1)))
			lb := (p.image.height / 2) - int(w*float64(margin))
			ub := (p.image.height / 2) + int(w*float64(margin))
			for row := lb; row <= ub; row++ {
				SetA(&p.image.pixels[row][p.image.width-1-margin+col], 255)
				SetA(&p.image.pixels[row][p.image.width-1-margin-col], 255)
			}
		}
	} else if p.rightConnector.connectionType == ConnectionBlank {
		for col := range margin {
			w := p.rightConnector.tabGenerator.Get((float64(col) / float64(margin-1)))
			lb := (p.image.height / 2) - int(w*float64(margin))
			ub := (p.image.height / 2) + int(w*float64(margin))
			for row := lb; row <= ub; row++ {
				SetA(&p.image.pixels[row][p.image.width-1-margin-col], 0)
				SetA(&p.image.pixels[row][p.image.width-1-margin+col], 0)
			}
		}
	}

	if p.leftConnector.connectionType == ConnectionTab {
		for col := range margin {
			w := p.leftConnector.tabGenerator.Get((float64(col) / float64(margin-1)))
			lb := (p.image.height / 2) - int(w*float64(margin))
			ub := (p.image.height / 2) + int(w*float64(margin))
			for row := lb; row <= ub; row++ {
				SetA(&p.image.pixels[row][margin-col], 255)
				SetA(&p.image.pixels[row][margin+col], 255)
			}
		}
	} else if p.leftConnector.connectionType == ConnectionBlank {
		for col := range margin {
			w := p.leftConnector.tabGenerator.Get((float64(col) / float64(margin-1)))
			lb := (p.image.height / 2) - int(w*float64(margin))
			ub := (p.image.height / 2) + int(w*float64(margin))
			for row := lb; row <= ub; row++ {
				SetA(&p.image.pixels[row][margin+col], 0)
				SetA(&p.image.pixels[row][margin-col], 0)
			}
		}
	}
}

func Sin(x float64, a float64, xs float64, c float64) float64 {
	return a * math.Sin(xs*(x-c))
}
