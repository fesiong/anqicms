package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDefaultVideoProcessingConfig(t *testing.T) {
	got := DefaultVideoProcessingConfig()
	want := VideoProcessingConfig{
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
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultVideoProcessingConfig() = %#v, want %#v", got, want)
	}
}

func TestVideoProcessingConfigNormalize(t *testing.T) {
	tests := []struct {
		name    string
		initial VideoProcessingConfig
		want    VideoProcessingConfig
	}{
		{
			name: "empty config receives safe defaults",
			want: DefaultVideoProcessingConfig(),
		},
		{
			name: "invalid codecs fall back to defaults",
			initial: VideoProcessingConfig{
				Enabled:       0,
				PosterEnabled: 1,
				Format:        "mpeg",
				Qualities:     []string{"8k"},
				PosterTime:    -1,
				PosterWidth:   0,
				PosterQuality: 101,
				VideoCodec:    "invalid;rm",
				AudioCodec:    "invalid codec",
				HLSSegment:    0,
				Concurrency:   20,
				MaxRetries:    -1,
				Timeout:       0,
			},
			want: VideoProcessingConfig{
				Enabled:       0,
				Format:        VideoFormatMP4,
				Qualities:     []string{VideoQualityOriginal},
				PosterEnabled: 1,
				PosterTime:    1,
				PosterWidth:   1280,
				PosterQuality: 85,
				VideoCodec:    "libx264",
				AudioCodec:    "aac",
				HLSSegment:    6,
				Concurrency:   4,
				MaxRetries:    0,
				Timeout:       3600,
			},
		},
		{
			name: "duplicates and unknown qualities are removed",
			initial: VideoProcessingConfig{
				Enabled:       1,
				PosterEnabled: 1,
				PosterTime:    1,
				PosterWidth:   1280,
				PosterQuality: 85,
				Format:        VideoFormatHLS,
				Qualities:     []string{"720p", "720p", "8k", "original"},
				HLSSegment:    12,
				Concurrency:   2,
				MaxRetries:    3,
				Timeout:       120,
			},
			want: VideoProcessingConfig{
				Enabled:       1,
				Format:        VideoFormatHLS,
				Qualities:     []string{VideoQuality720, VideoQualityOriginal},
				PosterEnabled: 1,
				PosterTime:    1,
				PosterWidth:   1280,
				PosterQuality: 85,
				VideoCodec:    "libx264",
				AudioCodec:    "aac",
				HLSSegment:    12,
				Concurrency:   2,
				MaxRetries:    3,
				Timeout:       120,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.initial
			got.Normalize()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Normalize() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestVideoProcessingConfigJSONRoundTrip(t *testing.T) {
	original := ContentConfig{Video: DefaultVideoProcessingConfig()}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ContentConfig
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Video, original.Video) {
		t.Fatalf("round trip = %#v, want %#v", decoded.Video, original.Video)
	}
}
