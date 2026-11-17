package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.yaml.in/yaml/v3"
	"sigamepackcompressor/internal/media"
	"sigamepackcompressor/internal/siq"
)

type Config struct {
	BuildVersion        string     `yaml:"-"`
	Listen              string     `yaml:"listen"`
	StoragePath         string     `yaml:"storage_path"`
	WebPath             string     `yaml:"web_path"`
	Workers             int        `yaml:"workers"`
	QueueSize           int        `yaml:"queue_size"`
	MaxUploadMB         int64      `yaml:"max_upload_mb"`
	Vips                string     `yaml:"vips"`
	VipsHeader          string     `yaml:"vipsheader"`
	FFmpeg              string     `yaml:"ffmpeg"`
	FFprobe             string     `yaml:"ffprobe"`
	MediaTimeoutSeconds int        `yaml:"media_timeout_seconds"`
	Threads             int        `yaml:"threads_per_process"`
	AllowedOrigins      []string   `yaml:"allowed_origins"`
	Limits              siq.Limits `yaml:"archive_limits"`
}

func Defaults() Config {
	return Config{Listen: "127.0.0.1:8000", StoragePath: "storage", WebPath: "web", Workers: 1, QueueSize: 16, MaxUploadMB: 1024, Vips: "vips", VipsHeader: "vipsheader", FFmpeg: "ffmpeg", FFprobe: "ffprobe", MediaTimeoutSeconds: 300, Threads: 1, Limits: siq.DefaultLimits()}
}
func Load(filename string) (Config, error) {
	c := Defaults()
	filename, err := filepath.Abs(filename)
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(filename)
	if err == nil {
		if err = yaml.Unmarshal(b, &c); err != nil {
			return c, err
		}
	} else if !os.IsNotExist(err) {
		return c, err
	}
	for key, dest := range map[string]*string{"SI_LISTEN": &c.Listen, "SI_STORAGE_PATH": &c.StoragePath, "SI_WEB_PATH": &c.WebPath, "SI_VIPS": &c.Vips, "SI_VIPSHEADER": &c.VipsHeader, "SI_FFMPEG": &c.FFmpeg, "SI_FFPROBE": &c.FFprobe} {
		if v := os.Getenv(key); v != "" {
			*dest = v
		}
	}
	if c.Workers < 1 || c.Workers > 32 || c.QueueSize < 1 || c.QueueSize > 1000 || c.MaxUploadMB < 1 || c.MaxUploadMB > 16384 || c.MediaTimeoutSeconds < 1 || c.Threads < 1 || c.Limits.Files < 1 || c.Limits.ExpandedBytes < 1 || c.Limits.FileBytes < 1 || c.Limits.XMLBytes < 1 {
		return c, fmt.Errorf("invalid server resource limits")
	}
	if c.StoragePath == "" {
		return c, fmt.Errorf("storage_path cannot be empty")
	}
	base := filepath.Dir(filename)
	if !filepath.IsAbs(c.StoragePath) {
		c.StoragePath = filepath.Join(base, c.StoragePath)
	}
	if c.WebPath != "" && !filepath.IsAbs(c.WebPath) {
		c.WebPath = filepath.Join(base, c.WebPath)
	}
	return c, nil
}
func (c Config) Processor() media.Processor {
	p := media.DefaultProcessor()
	p.Vips = c.Vips
	p.VipsHeader = c.VipsHeader
	p.FFmpeg = c.FFmpeg
	p.FFprobe = c.FFprobe
	p.Timeout = time.Duration(c.MediaTimeoutSeconds) * time.Second
	p.Threads = c.Threads
	return p
}
