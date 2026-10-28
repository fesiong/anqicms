package intent

import (
	"encoding/json"
	"strings"
)

// MaskedSecret 是凭据类字段的统一掩码占位符。
//
// 与 request.MysqlPasswordMask / manageController.SendmailPasswordMask 保持同一串，
// 让「读到的掩码」在任意通道里都有唯一含义：这里本该有值，但被藏了。
const MaskedSecret = "********"

// secretFieldNames 是需要掩码的字段名（小写精确匹配）。
//
// 只收**确凿是凭据**的名字，不做宽泛的子串匹配：
// 误掩码会把 token_expire / token_type 这类普通字段也打成 ********，
// 反而让调用方误判配置状态。新增资源时按实际字段名补进来。
var secretFieldNames = map[string]bool{
	"password":           true,
	"passwd":             true,
	"pwd":                true,
	"secret":             true,
	"app_secret":         true,
	"client_secret":      true,
	"secret_id":          true,
	"secret_key":         true,
	"access_key":         true,
	"access_key_id":      true,
	"access_key_secret":  true,
	"private_key":        true,
	"api_key":            true,
	"apikey":             true,
	"token":              true,
	"token_secret":       true,
	"encoding_aes_key":   true,
	"verify_key":         true,
	"license":            true,
	"mcp_token":          true,
	"aliyun_access_key_id":     true,
	"aliyun_access_key_secret": true,
	"tencent_secret_id":        true,
	"tencent_secret_key":       true,
	"s3_access_key":            true,
	"s3_secret_key":            true,
	"qiniu_access_key":         true,
	"qiniu_secret_key":         true,
	"upai_token":               true,
}

// IsSecretField 判断字段名是否为凭据类字段。
func IsSecretField(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if secretFieldNames[n] {
		return true
	}
	// 资源前缀形式：mysql_password / smtp_password / auth_secret …
	return strings.HasSuffix(n, "_password") || strings.HasSuffix(n, "_secret") ||
		strings.HasSuffix(n, "_access_key") || strings.HasSuffix(n, "_api_key")
}

// MaskSecrets 递归把凭据字段的值替换成掩码，返回新对象（不修改入参）。
//
// 背景（2026-10-27 全接口测试）：6 个只读意图明文吐凭据——存储配置的
// s3 access_key/secret、公众号 app_secret、Google client_secret、
// 多站点列表里的 mysql.password、导入配置、站点 JWT 密钥。
// 收口放在 Kernel 出口而不是逐个 handler：工具面会持续新增，
// 按端点补漏必然漏掉下一个；出口只有一处，新工具自动继承。
//
// 只处理字符串值：已经是空串的保持空串（区分「没配」与「配了但被藏了」，
// 后者才该显示掩码）。
func MaskSecrets(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if IsSecretField(k) {
				if s, ok := val.(string); ok && s != "" {
					out[k] = MaskedSecret
					continue
				}
			}
			out[k] = MaskSecrets(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = MaskSecrets(val)
		}
		return out
	default:
		return v
	}
}

// MaskSecretsInJSON 对一段 JSON 文本做同样的掩码；非 JSON 原样返回。
//
// 用于「没有结构化载荷、只能直出原始文本」的通道（res.Data 为 nil 时）。
func MaskSecretsInJSON(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return text
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return text
	}
	masked := MaskSecrets(v)
	b, err := json.Marshal(masked)
	if err != nil {
		return text
	}
	return string(b)
}

// ContainsMaskedSecret 报告参数里是否含掩码值（用于写前拦截）。
func ContainsMaskedSecret(v any) bool {
	switch t := v.(type) {
	case string:
		return t == MaskedSecret
	case map[string]any:
		for _, val := range t {
			if ContainsMaskedSecret(val) {
				return true
			}
		}
	case []any:
		for _, val := range t {
			if ContainsMaskedSecret(val) {
				return true
			}
		}
	}
	return false
}
