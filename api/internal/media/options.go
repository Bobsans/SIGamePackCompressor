package media

import "fmt"

// Full-HD SIGame normal-layout content area, rounded down to even dimensions.
// Measurement and layout assumptions: docs/game-field-size.md.
const DefaultMaxWidth = 1152
const DefaultMaxHeight = 658

type Options struct {
	OutputVersion string       `json:"output_version"`
	Images        ImageOptions `json:"images"`
	Audio         AudioOptions `json:"audio"`
	Video         VideoOptions `json:"video"`
}
type ImageOptions struct {
	Enabled   bool   `json:"enabled"`
	MaxWidth  int    `json:"max_width"`
	MaxHeight int    `json:"max_height"`
	Quality   int    `json:"quality"`
	Format    string `json:"format"`
	Lossless  bool   `json:"lossless"`
	Animation string `json:"animation"`
}
type AudioOptions struct {
	Enabled bool   `json:"enabled"`
	Format  string `json:"format"`
	Bitrate int    `json:"bitrate_kbps"`
}
type VideoOptions struct {
	Enabled      bool   `json:"enabled"`
	MaxWidth     int    `json:"max_width"`
	MaxHeight    int    `json:"max_height"`
	CRF          int    `json:"crf"`
	Preset       string `json:"preset"`
	AudioBitrate int    `json:"audio_bitrate_kbps"`
}

func Defaults() Options {
	return Options{"5", ImageOptions{true, DefaultMaxWidth, DefaultMaxHeight, 80, "webp", false, "preserve"}, AudioOptions{true, "mp3", 96}, VideoOptions{true, DefaultMaxWidth, DefaultMaxHeight, 26, "medium", 96}}
}
func (o Options) Validate() error {
	if o.OutputVersion != "5" && o.OutputVersion != "source" {
		return fmt.Errorf("output_version must be 5 or source")
	}
	if o.Images.MaxWidth < 16 || o.Images.MaxWidth > 8192 || o.Images.MaxHeight < 16 || o.Images.MaxHeight > 8192 || o.Images.Quality < 1 || o.Images.Quality > 100 {
		return fmt.Errorf("invalid image dimensions or quality")
	}
	if o.Images.Format != "webp" && o.Images.Format != "source" {
		return fmt.Errorf("images.format must be webp or source")
	}
	if o.Images.Animation != "preserve" && o.Images.Animation != "webp" {
		return fmt.Errorf("images.animation must be preserve or webp")
	}
	if o.Audio.Format != "mp3" && o.Audio.Format != "opus" {
		return fmt.Errorf("audio.format must be mp3 or opus")
	}
	if o.Audio.Bitrate < 32 || o.Audio.Bitrate > 320 {
		return fmt.Errorf("audio bitrate must be 32..320 kbps")
	}
	if o.Video.MaxWidth < 16 || o.Video.MaxWidth > 8192 || o.Video.MaxHeight < 16 || o.Video.MaxHeight > 8192 || o.Video.CRF < 0 || o.Video.CRF > 51 || o.Video.AudioBitrate < 32 || o.Video.AudioBitrate > 320 {
		return fmt.Errorf("invalid video settings")
	}
	switch o.Video.Preset {
	case "ultrafast", "superfast", "veryfast", "faster", "fast", "medium", "slow", "slower", "veryslow":
	default:
		return fmt.Errorf("invalid video preset")
	}
	return nil
}
