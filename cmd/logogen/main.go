package main

// logogen generates simple stand-in PNG placeholders for the login page
// logos so the /static/images/ references resolve. Replace these files
// with the real artwork when available (same filenames, any dimensions).

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

var blue = color.RGBA{41, 55, 127, 255}
var gold = color.RGBA{240, 180, 30, 255}
var white = color.RGBA{255, 255, 255, 255}

func circle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	for y := int(cy - r); y <= int(cy+r); y++ {
		for x := int(cx - r); x <= int(cx+r); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
}

func ring(img *image.RGBA, cx, cy, r, w float64, c color.RGBA) {
	for y := int(cy - r - w); y <= int(cy+r+w); y++ {
		for x := int(cx - r - w); x <= int(cx+r+w); x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			d := math.Sqrt(dx*dx + dy*dy)
			if d <= r+w && d >= r-w {
				img.Set(x, y, c)
			}
		}
	}
}

func rect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			img.Set(x, y, c)
		}
	}
}

func save(name string, img *image.RGBA) {
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func main() {
	// org-logo.png — 128x128 blue ring badge with a gold inner ring and a
	// white "PWA" monogram block.
	org := image.NewRGBA(image.Rect(0, 0, 128, 128))
	ring(org, 64, 64, 56, 7, blue)
	ring(org, 64, 64, 45, 3, gold)
	rect(org, 44, 52, 84, 76, white) // clears space behind the monogram
	rect(org, 48, 56, 58, 72, blue)
	rect(org, 58, 56, 62, 72, blue)
	rect(org, 70, 56, 80, 60, blue)
	rect(org, 70, 64, 80, 68, blue)
	rect(org, 70, 72, 80, 76, blue)
	save("web/static/images/org-logo.png", org)

	// vendor-logo.png — 240x80 blue double-arc mark with a wordmark bar.
	v := image.NewRGBA(image.Rect(0, 0, 240, 80))
	ring(v, 36, 40, 24, 5, blue)
	ring(v, 52, 44, 14, 4, blue)
	for y := 20; y <= 28; y++ {
		for x := 90; x <= 220; x++ {
			v.Set(x, y, blue)
		}
	}
	for y := 40; y <= 46; y++ {
		for x := 90; x <= 200; x++ {
			v.Set(x, y, blue)
		}
	}
	for y := 60; y <= 66; y++ {
		for x := 90; x <= 230; x++ {
			v.Set(x, y, blue)
		}
	}
	save("web/static/images/vendor-logo.png", v)
}
