package config

const (
	RewriteNumberMode  = 0 //数字模式
	RewriteStringMode1 = 1 //命名模式1
	RewriteStringMode2 = 2 //命名模式2
	RewriteStringMode3 = 3 //命名模式3
	RewritePatternMode = 4 //正则模式
)

type PluginRewriteConfig struct {
	Mode   int    `json:"mode"`   // 伪静态模式：0-4是内置模式。0=ID模式，1=模型+URL别名，2=URL别名+ID，3=分类+URL别名，4=自定义正则模式
	Patten string `json:"patten"` // 正则表达式内容，模式4的时候填写。
}
