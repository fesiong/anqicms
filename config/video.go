package config

import (
	"slices"
	"strings"
)

const (
	VideoFormatMP4 = "mp4"
	VideoFormatHLS = "hls"

	VideoQuality480      = "480p"
	VideoQuality720      = "720p"
	VideoQuality1080     = "1080p"
	VideoQualityOriginal = "original"
)

type VideoProcessingConfig struct {
	Enabled       int      `json:"enabled"`
	Format        string   `json:"format"`
	Qualities     []string `json:"qualities"`
	PosterEnabled int      `json:"poster_enabled"`
	PosterTime    int      `json:"poster_time"`
	PosterWidth   int      `json:"poster_width"`
	PosterQuality int      `json:"poster_quality"`
	VideoCodec    string   `json:"video_codec"`
	AudioCodec    string   `json:"audio_codec"`
	HLSSegment    int      `json:"hls_segment"`
	Concurrency   int      `json:"concurrency"`
	MaxRetries    int      `json:"max_retries"`
	Timeout       int      `json:"timeout"`
}

func DefaultVideoProcessingConfig() VideoProcessingConfig {
	return VideoProcessingConfig{
		Enabled:       1,
		Format:        VideoFormatMP4,
		Qualities:     []string{VideoQuality480, VideoQuality720, VideoQuality1080, VideoQualityOriginal},
		PosterEnabled: 1,
		PosterTime:    1,
		PosterWidth:   1280,
		PosterQuality: 85,
		VideoCodec:    "libx264",
		AudioCodec:    "aac",
		HLSSegment:    6,
		Concurrency:   1,
		MaxRetries:    2,
		Timeout:       3600,
	}
}

func (c *VideoProcessingConfig) Normalize() {
	defaults := DefaultVideoProcessingConfig()
	if c.Enabled == 0 && c.Format == "" && len(c.Qualities) == 0 && c.VideoCodec == "" &&
		c.PosterWidth == 0 && c.PosterQuality == 0 && c.Concurrency == 0 && c.Timeout == 0 {
		*c = defaults
		return
	}
	if c.Format != VideoFormatHLS {
		c.Format = VideoFormatMP4
	}
	if !isSafeCodec(c.VideoCodec) {
		c.VideoCodec = defaults.VideoCodec
	}
	if !isSafeCodec(c.AudioCodec) {
		c.AudioCodec = defaults.AudioCodec
	}
	if c.Enabled != 0 {
		c.Enabled = defaults.Enabled
	}
	if c.PosterEnabled != 0 {
		c.PosterEnabled = defaults.PosterEnabled
	}
	if c.PosterTime <= 0 {
		c.PosterTime = defaults.PosterTime
	}
	if c.PosterTime > 86400 {
		c.PosterTime = defaults.PosterTime
	}
	if c.PosterWidth == 0 {
		c.PosterWidth = defaults.PosterWidth
	}
	if c.PosterWidth < 160 || c.PosterWidth > 3840 {
		c.PosterWidth = defaults.PosterWidth
	}
	if c.PosterQuality < 1 || c.PosterQuality > 100 {
		c.PosterQuality = defaults.PosterQuality
	}
	if c.HLSSegment < 2 || c.HLSSegment > 60 {
		c.HLSSegment = defaults.HLSSegment
	}
	if c.Concurrency < 1 {
		c.Concurrency = defaults.Concurrency
	}
	if c.Concurrency > 4 {
		c.Concurrency = 4
	}
	if c.MaxRetries < 0 {
		c.MaxRetries = 0
	}
	if c.MaxRetries > 10 {
		c.MaxRetries = 10
	}
	if c.Timeout < 60 {
		c.Timeout = defaults.Timeout
	}
	if c.Timeout > 86400 {
		c.Timeout = 86400
	}

	qualities := make([]string, 0, len(c.Qualities))
	for _, quality := range c.Qualities {
		quality = strings.ToLower(strings.TrimSpace(quality))
		if quality == "" || slices.Contains(qualities, quality) {
			continue
		}
		switch quality {
		case VideoQuality480, VideoQuality720, VideoQuality1080, VideoQualityOriginal:
			qualities = append(qualities, quality)
		}
	}
	if len(qualities) == 0 {
		qualities = []string{VideoQualityOriginal}
	}
	c.Qualities = qualities
}

func isSafeCodec(codec string) bool {
	if codec == "" || len(codec) > 64 {
		return false
	}
	for _, char := range codec {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && char != '_' && char != '-' && char != '.' {
			return false
		}
	}
	return true
}
