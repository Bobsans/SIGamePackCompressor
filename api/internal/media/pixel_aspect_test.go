package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sigamepackcompressor/internal/siq"
)

func TestNativeVideoPreservesDisplayedAspect(t *testing.T) {
	p := integrationProcessor(t)
	ctx := context.Background()
	dir := t.TempDir()
	src := filepath.Join(dir, "anamorphic.mp4")
	if _, err := p.run(ctx, p.FFmpeg, "-nostdin", "-y", "-v", "error", "-f", "lavfi", "-i", "color=c=blue:size=1600x900:rate=1", "-frames:v", "1", "-vf", "setsar=4/3", "-c:v", "libx264", "-preset", "ultrafast", src); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	o := Defaults()
	o.Video.Preset = "ultrafast"
	out, _, err := p.Transform(dir, o)(ctx, siq.Media{Type: "video", Extension: ".mp4"}, f)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(out)
	d, err := p.Inspect(ctx, "video", out)
	if err != nil {
		t.Fatal(err)
	}
	// The input display ratio is (1600 * 4/3) / 900, not 16:9.
	if d.Width != 1152 || d.Height != 486 {
		t.Fatalf("display aspect distorted: %dx%d", d.Width, d.Height)
	}
}
