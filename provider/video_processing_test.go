package provider

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"kandaoni.com/anqicms/config"
	"kandaoni.com/anqicms/model"
)

func TestSelectVideoRenditions(t *testing.T) {
	tests := []struct {
		name   string
		format string
		height int
		want   []string
	}{
		{name: "mp4 keeps original and skips upscale", format: config.VideoFormatMP4, height: 720, want: []string{"480p", "720p", "original"}},
		{name: "hls original keeps source resolution", format: config.VideoFormatHLS, height: 2160, want: []string{"480p", "720p", "1080p", "original"}},
		{name: "low resolution requires original", format: config.VideoFormatMP4, height: 360, want: []string{"original"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DefaultVideoProcessingConfig()
			cfg.Format = tt.format
			probe := videoProbe{Width: 1920, Height: tt.height, Duration: 10}
			renditions, err := selectVideoRenditions(cfg, probe)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(renditions))
			for _, rendition := range renditions {
				got = append(got, rendition.Name)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("renditions = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSelectVideoRenditionsRejectsOnlyUpscales(t *testing.T) {
	cfg := config.DefaultVideoProcessingConfig()
	cfg.Qualities = []string{config.VideoQuality1080}
	if _, err := selectVideoRenditions(cfg, videoProbe{Width: 1280, Height: 720}); err == nil {
		t.Fatal("expected an error when every selected quality would upscale")
	}
}

func TestBuildFFmpegRenderArgs(t *testing.T) {
	cfg := config.DefaultVideoProcessingConfig()
	rendition := videoRendition{Name: "720p", Height: 720}
	mp4 := buildFFmpegRenderArgs(rendition, "/tmp/input.mp4", "/tmp/720p.mp4", "/tmp/hls", cfg)
	for _, want := range []string{"-i", "/tmp/input.mp4", "-vf", "scale=-2:720", "-c:v", "libx264", "-c:a", "aac", "-movflags", "+faststart", "-progress", "pipe:1", "/tmp/720p.mp4"} {
		if !containsArg(mp4, want) {
			t.Fatalf("mp4 args %#v missing %q", mp4, want)
		}
	}
	if containsArg(mp4, ";") || containsArg(mp4, "|") {
		t.Fatalf("mp4 args contain shell metacharacters: %#v", mp4)
	}

	cfg.Format = config.VideoFormatHLS
	token := "0123456789abcdef0123456789abcdef"
	hls := buildFFmpegRenderArgs(rendition, "/tmp/input.mp4", "/tmp/hls/"+token+"/"+token+".m3u8", "/tmp/hls/"+token, cfg)
	for _, want := range []string{"-hls_time", "6", "-hls_playlist_type", "vod", "-hls_segment_filename", "/tmp/hls/" + token + "/" + token + "_%03d.ts", "/tmp/hls/" + token + "/" + token + ".m3u8"} {
		if !containsArg(hls, want) {
			t.Fatalf("hls args %#v missing %q", hls, want)
		}
	}
}

func TestBuildHLSMasterPlaylistUsesVariantFiles(t *testing.T) {
	variants := []model.VideoVariant{
		{Name: "480p", File: "video/7/run/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8", Width: 854, Height: 480},
		{Name: "720p", File: "video/7/run/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.m3u8", Width: 1280, Height: 720},
	}
	got := buildHLSMasterPlaylist(variants)
	want := "#EXTM3U\n#EXT-X-VERSION:3\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=700000,RESOLUTION=854x480\naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.m3u8\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=1400000,RESOLUTION=1280x720\nbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.m3u8\n"
	if got != want {
		t.Fatalf("playlist =\n%s\nwant:\n%s", got, want)
	}
}

func TestParseVideoProbe(t *testing.T) {
	probe, err := parseVideoProbe(`{"format":{"duration":"12.5"},"streams":[{"codec_type":"video","width":1920,"height":1080},{"codec_type":"audio"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if probe.Duration != 12.5 || probe.Width != 1920 || probe.Height != 1080 || !probe.HasAudio {
		t.Fatalf("probe = %#v", probe)
	}
	if _, err := parseVideoProbe(`{"streams":[{"codec_type":"audio"}]}`); err == nil {
		t.Fatal("expected missing video stream error")
	}
}

func TestParseFFmpegProgress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		duration float64
		want     int
	}{
		{name: "half", input: "out_time_us=5000000", duration: 10, want: 50},
		{name: "milliseconds fallback", input: "out_time_ms=2500", duration: 10, want: 25},
		{name: "clamped", input: "out_time_us=20000000", duration: 10, want: 100},
		{name: "invalid", input: "frame=10", duration: 10, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFFmpegProgress(tt.input, tt.duration); got != tt.want {
				t.Fatalf("progress = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRenderVideoProducesPlayableArtifacts(t *testing.T) {
	tests := []struct {
		name          string
		format        string
		qualities     []string
		defaultName   string
		artifactCount int
	}{
		{
			name:          "mp4 uses random task-owned filenames",
			format:        config.VideoFormatMP4,
			qualities:     []string{config.VideoQuality720, config.VideoQualityOriginal},
			defaultName:   config.VideoQuality720,
			artifactCount: 3,
		},
		{
			name:          "hls master playlist references random rendition paths",
			format:        config.VideoFormatHLS,
			qualities:     []string{config.VideoQuality480, config.VideoQualityOriginal},
			defaultName:   config.VideoQuality480,
			artifactCount: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			sourcePath := filepath.Join(tempDir, "source.mov")
			if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
				t.Fatal(err)
			}
			posterToken := strings.Repeat("a", 32)
			posterLocation := "uploads/2026/poster_" + posterToken + ".jpg"
			posterArgs := []string{}
			result, err := renderVideo(context.Background(), videoRenderOptions{
				SourcePath:     sourcePath,
				TempDir:        tempDir,
				RootLocation:   "video/7/run",
				PosterLocation: posterLocation,
				StorageURL:     "https://cdn.example",
				FFmpegPath:     "ffmpeg",
				FFprobePath:    "ffprobe",
				Config: config.VideoProcessingConfig{
					Enabled:       1,
					Format:        tt.format,
					Qualities:     tt.qualities,
					PosterEnabled: 1,
					PosterTime:    2,
					PosterWidth:   640,
					PosterQuality: 80,
					VideoCodec:    "libx264",
					AudioCodec:    "aac",
					HLSSegment:    6,
				},
				Run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
					if filepath.Base(name) == "ffprobe" {
						emit(`{"format":{"duration":"10"},"streams":[{"codec_type":"video","width":1920,"height":1080},{"codec_type":"audio"}]}`)
						return nil
					}
					output := args[len(args)-1]
					if strings.HasSuffix(output, ".jpg") {
						posterArgs = append([]string{}, args...)
						return os.WriteFile(output, []byte("poster"), 0o600)
					}
					if strings.HasSuffix(output, ".m3u8") {
						if err := os.WriteFile(output, []byte("#EXTM3U\n"), 0o600); err != nil {
							return err
						}
						segmentIndex := indexArg(args, "-hls_segment_filename")
						if segmentIndex < 0 || segmentIndex+1 >= len(args) {
							t.Fatalf("missing HLS segment filename in %#v", args)
						}
						segment := strings.ReplaceAll(args[segmentIndex+1], "%03d", "000")
						return os.WriteFile(segment, []byte("segment"), 0o600)
					}
					emit("out_time_us=5000000")
					return os.WriteFile(output, []byte("rendered"), 0o600)
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Playlist.Format != tt.format || result.Playlist.DefaultType != tt.defaultName {
				t.Fatalf("playlist = %#v", result.Playlist)
			}
			if result.Playlist.PosterURL != "https://cdn.example/"+posterLocation {
				t.Fatalf("poster URL = %q", result.Playlist.PosterURL)
			}
			if len(result.Playlist.Variants) != 2 || len(result.Artifacts) != tt.artifactCount {
				t.Fatalf("variants = %#v, artifacts = %#v", result.Playlist.Variants, result.Artifacts)
			}
			locations := make(map[string]string, len(result.Artifacts))
			for _, artifact := range result.Artifacts {
				locations[artifact.Location] = artifact.Path
				if _, err := os.Stat(artifact.Path); err != nil {
					t.Fatalf("artifact %q is not readable: %v", artifact.Location, err)
				}
				if artifact.Location != posterLocation {
					if !strings.HasPrefix(artifact.Location, "video/7/run/") {
						t.Fatalf("video artifact escaped task root: %q", artifact.Location)
					}
					if strings.Contains(artifact.Location, posterToken) {
						t.Fatalf("poster token leaked into video path: %q", artifact.Location)
					}
				}
			}
			if _, ok := locations[posterLocation]; !ok {
				t.Fatalf("poster is not beside the original upload: %#v", locations)
			}
			for index, variant := range result.Playlist.Variants {
				if variant.Name != tt.qualities[index] {
					t.Fatalf("variant name = %q, want quality label %q", variant.Name, tt.qualities[index])
				}
				if variant.URL != "https://cdn.example/"+variant.File {
					t.Fatalf("variant URL and file disagree: %#v", variant)
				}
				if _, ok := locations[variant.File]; !ok {
					t.Fatalf("variant file is not an uploaded artifact: %#v", variant)
				}
			}
			if tt.format == config.VideoFormatMP4 {
				mp4Name := regexp.MustCompile(`^video/7/run/[0-9a-f]{32}\.mp4$`)
				seenTokens := make(map[string]bool)
				for _, variant := range result.Playlist.Variants {
					if !mp4Name.MatchString(variant.File) {
						t.Fatalf("mp4 filename must be a random task-owned name: %q", variant.File)
					}
					token := strings.TrimSuffix(path.Base(variant.File), ".mp4")
					if seenTokens[token] {
						t.Fatalf("renditions reused a filename token: %q", token)
					}
					seenTokens[token] = true
				}
				if result.Playlist.DefaultURL != result.Playlist.Variants[0].URL {
					t.Fatalf("default URL = %q", result.Playlist.DefaultURL)
				}
			} else {
				variantPath := regexp.MustCompile(`^video/7/run/([0-9a-f]{32})/([0-9a-f]{32})\.m3u8$`)
				masterPattern := regexp.MustCompile(`^video/7/run/[0-9a-f]{32}\.m3u8$`)
				var masterLocation string
				for location := range locations {
					if masterPattern.MatchString(location) {
						masterLocation = location
					}
				}
				if masterLocation == "" || result.Playlist.DefaultURL != "https://cdn.example/"+masterLocation {
					t.Fatalf("master playlist = %#v, locations = %#v", result.Playlist, locations)
				}
				master, err := os.ReadFile(locations[masterLocation])
				if err != nil {
					t.Fatal(err)
				}
				for _, variant := range result.Playlist.Variants {
					matches := variantPath.FindStringSubmatch(variant.File)
					if len(matches) != 3 || matches[1] != matches[2] {
						t.Fatalf("HLS rendition path must use one random token: %q", variant.File)
					}
					token := matches[1]
					segmentLocation := "video/7/run/" + token + "/" + token + "_000.ts"
					if _, ok := locations[segmentLocation]; !ok {
						t.Fatalf("missing randomized HLS segment %q in %#v", segmentLocation, locations)
					}
					relative := token + "/" + token + ".m3u8"
					if !strings.Contains(string(master), "\n"+relative+"\n") {
						t.Fatalf("master playlist does not reference %q:\n%s", relative, master)
					}
				}
			}
			if containsArg(locationsAsSlice(locations), "source.mov") {
				t.Fatalf("source must not be registered as an artifact: %#v", locations)
			}
			if !containsArg(posterArgs, "-q:v") || !containsArg(posterArgs, "8") || !containsArg(posterArgs, "-vf") {
				t.Fatalf("poster args = %#v", posterArgs)
			}
		})
	}
}

func TestRenderVideoUsesFreshNamesPerRun(t *testing.T) {
	render := func() string {
		tempDir := t.TempDir()
		sourcePath := filepath.Join(tempDir, "source.mp4")
		if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
			t.Fatal(err)
		}
		cfg := config.DefaultVideoProcessingConfig()
		cfg.Format = config.VideoFormatMP4
		cfg.Qualities = []string{config.VideoQualityOriginal}
		cfg.PosterEnabled = 0
		result, err := renderVideo(context.Background(), videoRenderOptions{
			SourcePath:   sourcePath,
			TempDir:      tempDir,
			RootLocation: "video/7/15",
			FFmpegPath:   "ffmpeg",
			FFprobePath:  "ffprobe",
			Config:       cfg,
			Run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
				if name == "ffprobe" {
					emit(`{"format":{"duration":"10"},"streams":[{"codec_type":"video","width":1920,"height":1080}]}`)
					return nil
				}
				return os.WriteFile(args[len(args)-1], []byte("rendered"), 0o600)
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		return result.Playlist.Variants[0].File
	}
	first := render()
	second := render()
	if first == second {
		t.Fatalf("separate renders reused a predictable artifact path: %q", first)
	}
}

func locationsAsSlice(locations map[string]string) []string {
	items := make([]string, 0, len(locations))
	for location := range locations {
		items = append(items, location)
	}
	return items
}

func TestRenderVideoRetriesPosterAtFirstFrame(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultVideoProcessingConfig()
	cfg.Qualities = []string{config.VideoQualityOriginal}
	posterCalls := 0
	firstFrameRetry := false
	result, err := renderVideo(context.Background(), videoRenderOptions{
		SourcePath:     sourcePath,
		TempDir:        tempDir,
		RootLocation:   "video/7/15",
		PosterLocation: "uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg",
		StorageURL:     "/uploads",
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
		Config:         cfg,
		Run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
			if name == "ffprobe" {
				emit(`{"format":{"duration":"10"},"streams":[{"codec_type":"video","width":1920,"height":1080}]}`)
				return nil
			}
			output := args[len(args)-1]
			if strings.HasSuffix(output, ".jpg") {
				posterCalls++
				if posterCalls == 1 {
					return errors.New("poster seek failed")
				}
				seekIndex := indexArg(args, "-ss")
				if seekIndex < 0 || seekIndex+1 >= len(args) || args[seekIndex+1] != "0.000" {
					t.Fatalf("poster retry must start at the first frame: %#v", args)
				}
				firstFrameRetry = true
				return os.WriteFile(output, []byte("poster"), 0o600)
			}
			return os.WriteFile(output, []byte("rendered"), 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if posterCalls != 2 || !firstFrameRetry {
		t.Fatalf("poster calls = %d, first-frame retry = %v", posterCalls, firstFrameRetry)
	}
	if result.Playlist.PosterURL != "/uploads/uploads/2026/poster_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.jpg" {
		t.Fatalf("poster URL = %q", result.Playlist.PosterURL)
	}
}

func TestRenderVideoContinuesWithoutPosterWhenExtractionFails(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultVideoProcessingConfig()
	cfg.Qualities = []string{config.VideoQualityOriginal}
	posterCalls := 0
	result, err := renderVideo(context.Background(), videoRenderOptions{
		SourcePath:     sourcePath,
		TempDir:        tempDir,
		RootLocation:   "video/7/16",
		PosterLocation: "uploads/2026/poster_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb.jpg",
		StorageURL:     "/uploads",
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
		Config:         cfg,
		Run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
			if name == "ffprobe" {
				emit(`{"format":{"duration":"10"},"streams":[{"codec_type":"video","width":1920,"height":1080}]}`)
				return nil
			}
			output := args[len(args)-1]
			if strings.HasSuffix(output, ".jpg") {
				posterCalls++
				return errors.New("poster unavailable")
			}
			return os.WriteFile(output, []byte("rendered"), 0o600)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if posterCalls != 2 {
		t.Fatalf("poster calls = %d, want configured-time and first-frame attempts", posterCalls)
	}
	if result.Playlist.PosterURL != "" {
		t.Fatalf("failed poster must not be published: %q", result.Playlist.PosterURL)
	}
	if len(result.Playlist.Variants) != 1 || result.Playlist.DefaultURL == "" {
		t.Fatalf("transcode must complete without a poster: %#v", result.Playlist)
	}
	for _, artifact := range result.Artifacts {
		if strings.HasSuffix(artifact.Location, ".jpg") {
			t.Fatalf("failed poster became an artifact: %#v", result.Artifacts)
		}
	}
}

func TestRenderVideoFailurePreservesSource(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source.mp4")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := renderVideo(context.Background(), videoRenderOptions{
		SourcePath:     sourcePath,
		TempDir:        tempDir,
		RootLocation:   "video/8/run",
		PosterLocation: "uploads/2026/poster_cccccccccccccccccccccccccccccccc.jpg",
		StorageURL:     "/uploads",
		FFmpegPath:     "ffmpeg",
		FFprobePath:    "ffprobe",
		Config:         config.DefaultVideoProcessingConfig(),
		Run: func(_ context.Context, name string, args []string, _ []string, emit func(string)) error {
			if filepath.Base(name) == "ffprobe" {
				emit(`{"format":{"duration":"10"},"streams":[{"codec_type":"video","width":1920,"height":1080}]}`)
				return nil
			}
			return os.ErrPermission
		},
	})
	if err == nil || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("error = %v", err)
	}
	if data, readErr := os.ReadFile(sourcePath); readErr != nil || string(data) != "source" {
		t.Fatalf("source changed after failure: data=%q err=%v", data, readErr)
	}
}

func indexArg(args []string, name string) int {
	for i, arg := range args {
		if arg == name {
			return i
		}
	}
	return -1
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want || strings.Contains(arg, want) {
			return true
		}
	}
	return false
}
