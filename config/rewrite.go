package config

const (
	RewriteNumberMode  = 0 //数字模式
	RewriteStringMode1 = 1 //命名模式1
	RewriteStringMode2 = 2 //命名模式2
	RewriteStringMode3 = 3 //命名模式3
	RewritePattenMode  = 4 //正则模式
)

type PluginRewriteConfig struct {
	Mode   int    `json:"mode"`
	Patten string `json:"patten"`
}
