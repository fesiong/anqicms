package sms

import (
	"io"
	"strings"
)

// 三家适配器共用的 HTTP 响应体读取契约（裁决 R38）。阿里云／腾讯云／Twilio
// 面对同一个敌意 body 威胁模型（网关插 HTML 错误页、CDN 拦截、WAF 返回 GBK），
// 逐字复制同一段逻辑三次会把「上限」与「UTF-8 边界」两个易错点分散到三处。
// 错误码表与响应解析形状**不共用**：阿里云是扁平 Code/Message/BizId，
// 腾讯云是嵌套 Response.SendStatusSet[]，各厂商保持分离。

// smsMaxResponseBytes 响应体读取上限 64 KiB。适配器明确预设了「网关插 HTML／
// CDN 拦截」这种敌意 body，而 10s 超时内高速链路能灌进数百 MB：先 io.ReadAll
// 再截 200 字节等于把整条响应收进内存。片段只需要 200 字节，64 KiB 足够留现场。
const smsMaxResponseBytes = 64 << 10

// smsBodySnippetMaxBytes 日志片段上限 200 字节。
const smsBodySnippetMaxBytes = 200

// readSMSResponseBody 带 64 KiB 上限读取响应体。调用方负责 Close。
func readSMSResponseBody(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, smsMaxResponseBytes))
}

// smsBodySnippet 原始 body 截断到 200 字节，供日志保留现场。
// 200 不是 UTF-8 rune 边界：国内网关／WAF／CDN 插进来的中文拦截页是 3 字节 rune，
// 硬切会留下半个汉字。这条 Message 的唯一目的地是 sms_logs.err_msg（规格 §10.4
// 「失败也留痕」），utf8mb4 列拒收非法字节序列时整条发送流水写入失败——最需要
// 留痕的「HTTP 200 非 JSON」反而什么线索都没留下。所以切完必须回到合法边界。
// 用 ToValidUTF8 而不是「整段 utf8.Valid 就退一字节」的循环：后者会在第一个非法
// 字节处停住，把其后的现场全部丢掉（非法字节在开头时更是整段清空）；ToValidUTF8
// 只丢弃非法字节（含被切开的半个 rune），其余现场原样保留，且字节数只减不增。
func smsBodySnippet(body []byte) string {
	if len(body) > smsBodySnippetMaxBytes {
		body = body[:smsBodySnippetMaxBytes]
	}
	return strings.ToValidUTF8(string(body), "")
}
