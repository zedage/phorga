package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rwcarlsen/goexif/exif"
)

func main() {
	files, _ := filepath.Glob("data/source/*.jpg")
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			fmt.Printf("%s: open error: %v\n", f, err)
			continue
		}
		x, err := exif.Decode(r)
		r.Close()
		if err != nil {
			fmt.Printf("%s: no EXIF (%v)\n", f, err)
			continue
		}
		t, err := x.DateTime()
		if err != nil {
			fmt.Printf("%s: no DateTime (%v)\n", f, err)
			continue
		}
		fmt.Printf("%s: EXIF date = %v\n", f, t)
	}
}

