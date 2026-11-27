package provider

import (
	"reflect"
	"testing"

	"kandaoni.com/anqicms/config"
)

func TestLoadContentSettingInitializesVideoDefaults(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		enabled int
	}{
		{name: "missing content setting", value: "", enabled: 1},
		{name: "legacy content setting", value: `{"max_page":200}`, enabled: 1},
		{name: "explicitly disabled video", value: `{"video":{"enabled":0}}`, enabled: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			site := &Website{}
			site.LoadContentSetting(tt.value)
			want := config.DefaultVideoProcessingConfig()
			want.Enabled = tt.enabled
			if !reflect.DeepEqual(site.Content.Video, want) {
				t.Fatalf("video config = %#v, want %#v", site.Content.Video, want)
			}
		})
	}
}
