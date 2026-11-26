package provider

import (
	"encoding/json"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

type videoProbe struct {
	Duration float64
	Width    int
	Height   int
	HasAudio bool
}

type videoRendition struct {
	Name       string
	Width      int
	Height     int
	IsOriginal bool
}

func selectVideoRenditions(cfg config.VideoProcessingConfig, probe videoProbe) ([]videoRendition, error) {
	if probe.Width <= 0 || probe.Height <= 0 {
		return nil, fmt.Errorf("invalid source video dimensions: %dx%d", probe.Width, probe.Height)
	}
	heights := map[string]int{
		config.VideoQuality480:  480,
		config.VideoQuality720:  720,
		config.VideoQuality1080: 1080,
	}
	renditions := make([]videoRendition, 0, len(cfg.Qualities))
	for _, quality := range cfg.Qualities {
		if quality == config.VideoQualityOriginal {
			renditions = append(renditions, videoRendition{
				Name:       quality,
				Width:      probe.Width,
				Height:     probe.Height,
				IsOriginal: true,
			})
			continue
		}
		height, ok := heights[quality]
		if !ok || probe.Height < height {
			continue
		}
		width := int(math.Round(float64(probe.Width) * float64(height) / float64(probe.Height)))
		width -= width % 2
		if width < 2 {
			width = 2
		}
		renditions = append(renditions, videoRendition{Name: quality, Width: width, Height: height})
	}
	if len(renditions) == 0 {
		return nil, fmt.Errorf("source resolution %dp is lower than every selected quality; select original", probe.Height)
	}
	return renditions, nil
}

func buildFFmpegRenderArgs(
	rendition videoRendition,
	input string,
	output string,
	segmentDir string,
	cfg config.VideoProcessingConfig,
) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-i", input,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-vf", "scale=-2:" + strconv.Itoa(rendition.Height),
		"-c:v", cfg.VideoCodec,
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", cfg.AudioCodec,
		"-b:a", "128k",
	}
	if cfg.Format == config.VideoFormatHLS {
		segmentName := filepath.Base(segmentDir) + "_%03d.ts"
		args = append(args,
			"-hls_time", strconv.Itoa(cfg.HLSSegment),
			"-hls_playlist_type", "vod",
			"-hls_segment_filename", filepath.Join(segmentDir, segmentName),
		)
	} else {
		args = append(args, "-movflags", "+faststart")
	}
	return append(args, "-progress", "pipe:1", "-nostats", output)
}

func buildHLSMasterPlaylist(variants []model.VideoVariant) string {
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n#EXT-X-VERSION:3\n")
	for _, variant := range variants {
		bandwidth := hlsBandwidth(variant.Height)
		builder.WriteString("#EXT-X-STREAM-INF:BANDWIDTH=")
		builder.WriteString(strconv.Itoa(bandwidth))
		if variant.Width > 0 && variant.Height > 0 {
			builder.WriteString(",RESOLUTION=")
			builder.WriteString(strconv.Itoa(variant.Width))
			builder.WriteString("x")
			builder.WriteString(strconv.Itoa(variant.Height))
		}
		builder.WriteString("\n")
		builder.WriteString(path.Base(path.Dir(variant.File)))
		builder.WriteString("/")
		builder.WriteString(path.Base(variant.File))
		builder.WriteString("\n")
	}
	return builder.String()
}

func hlsBandwidth(height int) int {
	switch {
	case height <= 480:
		return 700000
	case height <= 720:
		return 1400000
	case height <= 1080:
		return 2800000
	default:
		return 5000000
	}
}

func parseVideoProbe(output string) (videoProbe, error) {
	var payload struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		return videoProbe{}, err
	}
	duration, err := strconv.ParseFloat(payload.Format.Duration, 64)
	if err != nil || duration <= 0 {
		return videoProbe{}, fmt.Errorf("invalid video duration %q", payload.Format.Duration)
	}
	probe := videoProbe{Duration: duration}
	for _, stream := range payload.Streams {
		switch stream.CodecType {
		case "video":
			if probe.Width == 0 && probe.Height == 0 {
				probe.Width = stream.Width
				probe.Height = stream.Height
			}
		case "audio":
			probe.HasAudio = true
		}
	}
	if probe.Width <= 0 || probe.Height <= 0 {
		return videoProbe{}, fmt.Errorf("no usable video stream found")
	}
	return probe, nil
}

func parseFFmpegProgress(line string, duration float64) int {
	if duration <= 0 {
		return 0
	}
	var elapsed float64
	if strings.HasPrefix(line, "out_time_us=") {
		value, err := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_us="), 64)
		if err != nil {
			return 0
		}
		elapsed = value / 1000000
	} else if strings.HasPrefix(line, "out_time_ms=") {
		value, err := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_ms="), 64)
		if err != nil {
			return 0
		}
		elapsed = value / 1000
	}
	progress := int(elapsed / duration * 100)
	if progress < 0 {
		return 0
	}
	if progress > 100 {
		return 100
	}
	return progress
}
