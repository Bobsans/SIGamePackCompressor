package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sigamepackcompressor/internal/siq"
)

type Processor struct {
	Vips       string
	VipsHeader string
	FFmpeg     string
	FFprobe    string
	Timeout    time.Duration
	Threads    int
	MaxPixels  int64
	MaxFrames  int
}

func DefaultProcessor() Processor {
	return Processor{"vips", "vipsheader", "ffmpeg", "ffprobe", 5 * time.Minute, 1, 500_000_000, 1000}
}

type Details struct {
	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	Frames   int     `json:"frames,omitempty"`
	Duration float64 `json:"duration_seconds,omitempty"`
	Codec    string  `json:"codec,omitempty"`
	Format   string  `json:"format,omitempty"`
}
type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 65536 {
		keep := min(n, 65536-b.Len())
		_, _ = b.Buffer.Write(p[:keep])
	}
	return n, nil
}
func (p Processor) run(ctx context.Context, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = append(os.Environ(), "VIPS_CONCURRENCY="+strconv.Itoa(p.Threads), "VIPS_DISC_THRESHOLD=64m")
	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%s failed: %w: %s", filepath.Base(bin), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
func (p Processor) Capabilities() map[string]bool {
	result := map[string]bool{}
	for name, bin := range map[string]string{"vips": p.Vips, "vipsheader": p.VipsHeader, "ffmpeg": p.FFmpeg, "ffprobe": p.FFprobe} {
		_, err := exec.LookPath(bin)
		result[name] = err == nil
	}
	return result
}
func (p Processor) Check(o Options, items []siq.Media) error {
	c := p.Capabilities()
	for _, m := range items {
		switch m.Type {
		case "image":
			if o.Images.Enabled && (!c["vips"] || !c["vipsheader"]) {
				return fmt.Errorf("libvips tools are unavailable")
			}
		case "audio":
			if o.Audio.Enabled && (!c["ffmpeg"] || !c["ffprobe"]) {
				return fmt.Errorf("FFmpeg/ffprobe are unavailable")
			}
		case "video":
			if o.Video.Enabled && (!c["ffmpeg"] || !c["ffprobe"]) {
				return fmt.Errorf("FFmpeg/ffprobe are unavailable")
			}
		}
	}
	return nil
}

func (p Processor) Inspect(ctx context.Context, kind, filename string) (Details, error) {
	if kind == "image" {
		f, err := os.Open(filename)
		if err != nil {
			return Details{}, err
		}
		var head [64]byte
		n, _ := io.ReadFull(f, head[:])
		f.Close()
		b := head[:n]
		known := bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff}) || bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) || bytes.HasPrefix(b, []byte("GIF87a")) || bytes.HasPrefix(b, []byte("GIF89a")) || bytes.HasPrefix(b, []byte("BM")) || bytes.HasPrefix(b, []byte("II\x2a\x00")) || bytes.HasPrefix(b, []byte("MM\x00\x2a"))
		known = known || (n >= 12 && string(b[:4]) == "RIFF" && string(b[8:12]) == "WEBP") || (n >= 16 && string(b[4:8]) == "ftyp" && (bytes.Contains(b[8:], []byte("avif")) || bytes.Contains(b[8:], []byte("avis"))))
		if !known {
			return Details{}, fmt.Errorf("unsupported image signature; original retained")
		}
		out, err := p.run(ctx, p.VipsHeader, "-a", filename)
		if err != nil {
			return Details{}, err
		}
		d := Details{Frames: 1}
		height := 0
		for _, line := range strings.Split(out, "\n") {
			key, value, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			value = strings.TrimSpace(value)
			n, _ := strconv.Atoi(value)
			switch strings.TrimSpace(key) {
			case "width":
				d.Width = n
			case "height":
				height = n
			case "page-height":
				d.Height = n
			case "n-pages":
				d.Frames = n
			case "vips-loader":
				d.Format = value
			}
		}
		if d.Height == 0 {
			d.Height = height
		}
		if d.Width <= 0 || d.Height <= 0 || d.Frames <= 0 {
			return Details{}, fmt.Errorf("invalid image dimensions")
		}
		if d.Frames > p.MaxFrames || int64(d.Width)*int64(d.Height)*int64(d.Frames) > p.MaxPixels {
			return Details{}, fmt.Errorf("image pixel/frame limit exceeded")
		}
		return d, nil
	}
	out, err := p.run(ctx, p.FFprobe, "-v", "error", "-protocol_whitelist", "file,pipe", "-format_whitelist", "mov,matroska,webm,avi,mp3,wav,ogg,flac,aac,asf,mpeg,mpegts,aiff,ape", "-show_entries", "stream=codec_type,codec_name,width,height:format=duration,format_name", "-of", "json", filename)
	if err != nil {
		return Details{}, err
	}
	var probe struct {
		Streams []struct {
			Type   string `json:"codec_type"`
			Codec  string `json:"codec_name"`
			Width  int    `json:"width"`
			Height int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			Name     string `json:"format_name"`
		} `json:"format"`
	}
	if err := json.Unmarshal([]byte(out), &probe); err != nil {
		return Details{}, err
	}
	d := Details{Format: probe.Format.Name}
	d.Duration, _ = strconv.ParseFloat(probe.Format.Duration, 64)
	for _, s := range probe.Streams {
		if s.Type == kind {
			d.Codec = s.Codec
			d.Width = s.Width
			d.Height = s.Height
			return d, nil
		}
	}
	return d, fmt.Errorf("no %s stream", kind)
}

func (p Processor) Transform(workdir string, o Options) siq.Transform {
	return func(ctx context.Context, m siq.Media, r io.Reader) (result, warning string, err error) {
		if (m.Type == "image" && !o.Images.Enabled) || (m.Type == "audio" && !o.Audio.Enabled) || (m.Type == "video" && !o.Video.Enabled) {
			return "", "", nil
		}
		dir, err := os.MkdirTemp(workdir, "media-")
		if err != nil {
			return "", "", err
		}
		// The successful result is moved out so all decoder intermediates can be removed.
		defer os.RemoveAll(dir)
		src := filepath.Join(dir, "input"+m.Extension)
		f, err := os.Create(src)
		if err != nil {
			return "", "", err
		}
		_, err = io.Copy(f, r)
		closeErr := f.Close()
		if err != nil {
			return "", "", err
		}
		if closeErr != nil {
			return "", "", closeErr
		}
		d, err := p.Inspect(ctx, m.Type, src)
		if err != nil {
			return "", "", err
		}
		ext := m.Extension
		var args []string
		bin := p.FFmpeg
		switch m.Type {
		case "image":
			if o.Images.Format == "webp" {
				ext = ".webp"
			}
			if d.Frames > 1 && o.Images.Animation == "preserve" {
				ext = m.Extension
			}
			if d.Frames > 1 && o.Images.Animation == "webp" {
				ext = ".webp"
			}
			if ext == ".jpe" || ext == ".jpeg" {
				ext = ".jpg"
			}
			switch ext {
			case ".jpg", ".png", ".gif", ".webp", ".avif":
			default:
				return "", "Unsupported image format; original retained", nil
			}
			if d.Frames > 1 && ext != ".gif" && ext != ".webp" {
				return "", "Animated format retained unchanged", nil
			}
			bin = p.Vips
			input := src
			if d.Frames > 1 {
				input += "[n=-1]"
			}
			out := filepath.Join(dir, "output"+ext)
			save := ""
			switch ext {
			case ".webp":
				save = fmt.Sprintf("[Q=%d,effort=5,lossless=%t]", o.Images.Quality, o.Images.Lossless)
			case ".jpg":
				save = fmt.Sprintf("[Q=%d,optimize-coding=true]", o.Images.Quality)
			case ".png":
				save = "[compression=9]"
			case ".avif":
				save = fmt.Sprintf("[Q=%d,compression=av1]", o.Images.Quality)
			}
			args = []string{"thumbnail", input, out + save, strconv.Itoa(o.Images.MaxWidth), "--height", strconv.Itoa(o.Images.MaxHeight), "--size", "down"}
		case "audio":
			ext = "." + o.Audio.Format
			codec := "libmp3lame"
			if o.Audio.Format == "opus" {
				codec = "libopus"
			}
			args = []string{"-nostdin", "-y", "-v", "error", "-i", src, "-map", "0:a:0", "-vn", "-c:a", codec, "-b:a", fmt.Sprintf("%dk", o.Audio.Bitrate), "-threads", strconv.Itoa(p.Threads), filepath.Join(dir, "output"+ext)}
		case "video":
			ext = ".mp4"
			// Fit the displayed aspect ratio, including non-square source pixels,
			// before normalizing SAR. Independent width/height resizing would distort it.
			displayWidth := "iw*if(gt(sar,0),sar,1)"
			factor := fmt.Sprintf("min(1,min(%d/(%s),%d/ih))", o.Video.MaxWidth, displayWidth, o.Video.MaxHeight)
			filter := fmt.Sprintf("scale=w='max(2,trunc(%s*(%s)/2)*2)':h='max(2,trunc(ih*(%s)/2)*2)',setsar=1", displayWidth, factor, factor)
			args = []string{"-nostdin", "-y", "-v", "error", "-i", src, "-map", "0:v:0", "-map", "0:a?", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-preset", o.Video.Preset, "-crf", strconv.Itoa(o.Video.CRF), "-vf", filter, "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", o.Video.AudioBitrate), "-movflags", "+faststart", "-threads", strconv.Itoa(p.Threads), filepath.Join(dir, "output"+ext)}
		default:
			return "", "", nil
		}
		if _, err = p.run(ctx, bin, args...); err != nil {
			return "", "", err
		}
		out := filepath.Join(dir, "output"+ext)
		check, err := p.Inspect(ctx, m.Type, out)
		if err != nil {
			return "", "", fmt.Errorf("verify encoded media: %w", err)
		}
		if m.Type == "image" && d.Frames > 1 && check.Frames < 2 {
			return "", "", fmt.Errorf("encoder lost animation; original retained")
		}
		f, err = os.CreateTemp(workdir, "encoded-*")
		if err != nil {
			return "", "", err
		}
		name := f.Name()
		f.Close()
		os.Remove(name)
		name += ext
		if err = os.Rename(out, name); err != nil {
			return "", "", err
		}
		return name, "", nil
	}
}
