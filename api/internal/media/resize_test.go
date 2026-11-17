package media

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"sigamepackcompressor/internal/siq"
)

func TestNativeFitWithinBounds(t *testing.T) {
	p := integrationProcessor(t)
	ctx := context.Background()
	for _, sample := range []struct {
		name                                                            string
		width, height, imageWidth, videoWidth, imageHeight, videoHeight int
		custom                                                          bool
	}{
		{"landscape", 1920, 1080, 1152, 1152, 648, 648, false},
		{"portrait", 1000, 2000, 329, 328, 658, 658, false},
		{"square", 1600, 1600, 658, 658, 658, 658, false},
		{"small", 640, 360, 640, 640, 360, 360, false},
		{"custom", 1920, 1080, 400, 400, 225, 224, true},
	} {
		for _, kind := range []string{"image", "video"} {
			t.Run(sample.name+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				ext := ".png"
				if kind == "video" {
					ext = ".mp4"
				}
				src := filepath.Join(dir, "source"+ext)
				if kind == "image" {
					f, err := os.Create(src)
					if err != nil {
						t.Fatal(err)
					}
					err = png.Encode(f, image.NewNRGBA(image.Rect(0, 0, sample.width, sample.height)))
					f.Close()
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := p.run(ctx, p.FFmpeg, "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", fmt.Sprintf("color=c=blue:size=%dx%d:rate=1", sample.width, sample.height), "-frames:v", "1", "-c:v", "libx264", "-preset", "ultrafast", src); err != nil {
						t.Fatal(err)
					}
				}
				o := Defaults()
				o.Video.Preset = "ultrafast"
				if sample.custom {
					o.Images.MaxWidth = 400
					o.Images.MaxHeight = 300
					o.Video.MaxWidth = 400
					o.Video.MaxHeight = 300
				}
				f, err := os.Open(src)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				out, warning, err := p.Transform(dir, o)(ctx, siq.Media{Type: kind, Extension: ext}, f)
				if err != nil || out == "" {
					t.Fatalf("transform: %v, %s", err, warning)
				}
				defer os.Remove(out)
				d, err := p.Inspect(ctx, kind, out)
				if err != nil {
					t.Fatal(err)
				}
				wantWidth, wantHeight := sample.imageWidth, sample.imageHeight
				if kind == "video" {
					wantWidth, wantHeight = sample.videoWidth, sample.videoHeight
				}
				if d.Width != wantWidth || d.Height != wantHeight {
					t.Fatalf("%dx%d became %dx%d, want %dx%d without stretching or enlargement", sample.width, sample.height, d.Width, d.Height, wantWidth, wantHeight)
				}
			})
		}
	}
}
