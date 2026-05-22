//go:build !windows

package anqicms

import (
	"embed"
	_ "embed"
)

//go:embed icon.png
var AppIcon []byte

//go:embed system
var SystemFiles embed.FS
