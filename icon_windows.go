//go:build windows

package anqicms

import (
	"embed"
)

//go:embed icon.ico
var AppIcon []byte

//go:embed system
var SystemFiles embed.FS
