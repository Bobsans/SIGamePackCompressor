package media

import (
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigamepackcompressor/internal/siq"
)

func TestOptionsValidation(t *testing.T) {
	o := Defaults()
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	o.Video.Preset = "invalid"
	if o.Validate() == nil {
		t.Fatal("accepted invalid preset")
	}
	o = Defaults()
	o.Images.Quality = 101
	if o.Validate() == nil {
		t.Fatal("accepted invalid quality")
	}
}
func integrationProcessor(t *testing.T) Processor {
	t.Helper()
	if os.Getenv("SI_MEDIA_TESTS") != "1" {
		t.Skip("set SI_MEDIA_TESTS=1 to exercise native codecs")
	}
	p := DefaultProcessor()
	for key, dest := range map[string]*string{"SI_VIPS": &p.Vips, "SI_VIPSHEADER": &p.VipsHeader, "SI_FFMPEG": &p.FFmpeg, "SI_FFPROBE": &p.FFprobe} {
		if v := os.Getenv(key); v != "" {
			*dest = v
		}
	}
	for name, available := range p.Capabilities() {
		if !available {
			t.Fatalf("missing %s", name)
		}
	}
	return p
}
func imageFixture(t *testing.T, dir string) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 160, 120))
	for y := 0; y < 120; y++ {
		for x := 0; x < 160; x++ {
			a := uint8(255)
			if x < 20 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(x*13 + y*7), uint8(x + y*17), uint8(x*3 + y), a})
		}
	}
	name := filepath.Join(dir, "input.png")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return name
}
func TestNativeImageFormats(t *testing.T) {
	p := integrationProcessor(t)
	dir := t.TempDir()
	src := imageFixture(t, dir)
	ctx := context.Background()
	for _, ext := range []string{".png", ".jpg", ".gif", ".webp", ".avif"} {
		t.Run(ext, func(t *testing.T) {
			input := src
			if ext != ".png" {
				input = filepath.Join(dir, "fixture"+ext)
				output := input
				if ext == ".avif" {
					output += "[compression=av1]"
				}
				if _, err := p.run(ctx, p.Vips, "copy", src, output); err != nil {
					t.Fatal(err)
				}
			}
			f, _ := os.Open(input)
			defer f.Close()
			st, _ := f.Stat()
			o := Defaults()
			o.Images.MaxWidth = 80
			o.Images.MaxHeight = 80
			out, warning, err := p.Transform(dir, o)(ctx, siq.Media{Name: input, Type: "image", Size: st.Size(), Extension: ext}, f)
			if err != nil || out == "" {
				t.Fatalf("%s: %v %s", ext, err, warning)
			}
			defer os.Remove(out)
			d, err := p.Inspect(ctx, "image", out)
			if err != nil || d.Width != 80 || d.Height != 60 {
				t.Fatalf("dimensions: %+v %v", d, err)
			}
			if ext == ".png" {
				decoded := filepath.Join(dir, "decoded.png")
				if _, err = p.run(ctx, p.Vips, "copy", out, decoded); err != nil {
					t.Fatal(err)
				}
				r, _ := os.Open(decoded)
				img, err := png.Decode(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				_, _, _, alpha := img.At(0, 0).RGBA()
				if alpha != 0 {
					t.Fatal("transparency lost")
				}
			}
		})
	}
}
func TestNativeAnimationPreservesFramesAndTiming(t *testing.T) {
	p := integrationProcessor(t)
	dir := t.TempDir()
	palette := color.Palette{color.Transparent, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 0, 255, 255}}
	a := image.NewPaletted(image.Rect(0, 0, 100, 80), palette)
	b := image.NewPaletted(a.Rect, palette)
	for y := 0; y < 80; y++ {
		for x := 0; x < 100; x++ {
			a.SetColorIndex(x, y, 1)
			b.SetColorIndex(x, y, 2)
		}
	}
	input := filepath.Join(dir, "animated.gif")
	f, _ := os.Create(input)
	err := gif.EncodeAll(f, &gif.GIF{Image: []*image.Paletted{a, b}, Delay: []int{8, 12}, LoopCount: 0})
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"preserve", "webp"} {
		t.Run(mode, func(t *testing.T) {
			o := Defaults()
			o.Images.MaxWidth = 50
			o.Images.MaxHeight = 50
			o.Images.Animation = mode
			f, _ := os.Open(input)
			defer f.Close()
			out, _, err := p.Transform(dir, o)(context.Background(), siq.Media{Type: "image", Extension: ".gif"}, f)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(out)
			d, err := p.Inspect(context.Background(), "image", out)
			if err != nil || d.Frames != 2 || d.Width != 50 || d.Height != 40 {
				t.Fatalf("animation: %+v %v", d, err)
			}
			header, err := p.run(context.Background(), p.VipsHeader, "-a", out)
			if err != nil || !strings.Contains(header, "80 120") {
				t.Fatalf("timing lost: %s %v", header, err)
			}
			if mode == "webp" {
				r, _ := os.Open(out)
				defer r.Close()
				again, _, err := p.Transform(dir, o)(context.Background(), siq.Media{Type: "image", Extension: ".webp"}, r)
				if err != nil {
					t.Fatal(err)
				}
				os.Remove(again)
			}
		})
	}
}
func TestNativeAudioAndVideo(t *testing.T) {
	p := integrationProcessor(t)
	dir := t.TempDir()
	ctx := context.Background()
	audio := filepath.Join(dir, "in.wav")
	if _, err := p.run(ctx, p.FFmpeg, "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", audio); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(dir, "in.mp4")
	if _, err := p.run(ctx, p.FFmpeg, "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=10", "-t", "1", "-c:v", "libx264", video); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"audio", "video"} {
		t.Run(kind, func(t *testing.T) {
			input := audio
			if kind == "video" {
				input = video
			}
			f, _ := os.Open(input)
			defer f.Close()
			o := Defaults()
			o.Video.MaxWidth = 160
			o.Video.MaxHeight = 120
			o.Video.Preset = "ultrafast"
			out, _, err := p.Transform(dir, o)(ctx, siq.Media{Type: kind, Extension: filepath.Ext(input)}, f)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(out)
			d, err := p.Inspect(ctx, kind, out)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "video" && (d.Width != 160 || d.Height != 120 || d.Codec != "h264") {
				t.Fatal(d)
			}
			if kind == "audio" && d.Codec != "mp3" {
				t.Fatal(d)
			}
		})
	}
}
