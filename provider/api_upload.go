package provider

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"sort"
	"strings"
)

// 本文件为「上传类端点」提供 base64 兼容：调用方无需拼 multipart，
// 只要把文件内容以裸 base64 或 data URI 字符串传入，本层负责还原成真实的 multipart 请求体。
//
// 背景：394 个后台端点里有 17 个是 FormFile 上传类（attachment/upload、setting/favicon、
// plugin 的各种 upload/import 等）。纯 JSON body 无法投递文件，故为它们单独实现这层转换。
//
// 两种输入格式：
//  1. data URI：data:image/png;base64,iVBORw0KGgo...
//     还可携带 ;name=xxx 指定文件名。
//  2. 裸 base64 字符串：iVBORw0KGgo...（含标准与 URL 安全两种字母表，允许无 padding）
//
// 误判防护（重要）：普通字符串也可能"碰巧"能解成 base64（例如 "hello"）。
// 因此对裸 base64 设置了三重门槛：必须是文件类字段名、长度足够、且能被规范化回编。
// data URI 因为有明确的 data: 前缀，不存在歧义，故不限字段名。

// filePart 还原出的一个待上传文件。
type filePart struct {
	Field string // 表单字段名，通常为 file
	Name  string // 文件名
	Data  []byte
}

// maxUploadFileBytes 单个文件的体积上限。超过直接拒绝，避免一次调用把进程内存打爆。
const maxUploadFileBytes = 32 << 20 // 32MB

// minRawBase64Len 裸 base64 被认定为文件的最小长度。
const minRawBase64Len = 16

// multipartBoundary 固定的 multipart 分界串，见 buildMultipartBody 中关于确定性的说明。
const multipartBoundary = "----anqicmsApiInvokeBoundary"

// fileFieldKeys 会被当作"文件内容"处理的参数名。
//
// 字段名取自源码实测：17 个上传端点一律使用 ctx.FormFile("file")，
// 这里额外放宽到常见别名，方便调用方自然书写。
// 注意：name / cover 等虽同为表单字段，但在源码里承载的是文本语义，故不纳入。
var fileFieldKeys = map[string]bool{
	"file": true, "file1": true, "files": true, "files[]": true,
	"attachment": true, "upload": true, "upload_file": true,
	"image": true, "img": true, "source": true, "avatar": true,
}

// partitionFileParams 把入参拆分成「文件部分」与「普通表单字段」。
//
// 命中规则：
//   - 值为 data URI（以 data: 开头且为 base64）→ 无论字段名一律视为文件；
//   - 其余仅当字段名属于 fileFieldKeys，且值能通过严格 base64 校验时才视为文件。
//
// 返回值 deterministic：文件部分按字段名排序，与 map 迭代顺序无关。
func partitionFileParams(params map[string]any, filenameHint map[string]string) ([]filePart, map[string]any, error) {
	rest := map[string]any{}
	var files []filePart

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := params[k]
		// []any / []string 允许一次性传多个文件
		if arr, ok := v.([]any); ok {
			converted := false
			for _, item := range arr {
				data, name, mimeType, ok := valueAsFile(k, item)
				if !ok {
					continue
				}
				converted = true
				files = append(files, filePart{
					Field: k,
					Name:  pickFileName(name, filenameHint, data, mimeType),
					Data:  data,
				})
			}
			if converted {
				continue
			}
		}
		data, name, mimeType, ok := valueAsFile(k, v)
		if ok {
			files = append(files, filePart{
				Field: k,
				Name:  pickFileName(name, filenameHint, data, mimeType),
				Data:  data,
			})
			continue
		}
		rest[k] = v
	}

	for _, f := range files {
		if len(f.Data) > maxUploadFileBytes {
			return nil, nil, fmt.Errorf("文件 %s 体积 %d 字节超过上限 %d 字节", f.Name, len(f.Data), maxUploadFileBytes)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Field != files[j].Field {
			return files[i].Field < files[j].Field
		}
		return files[i].Name < files[j].Name
	})
	return files, rest, nil
}

// pickFileName 决定最终文件名，优先级：data URI 的 name 参数 > 显式 filename/file_name > 魔数嗅探 > 兜底。
func pickFileName(explicit string, hint map[string]string, data []byte, mimeType string) string {
	if explicit != "" {
		return sanitizeFileName(explicit)
	}
	for _, key := range []string{"filename", "file_name"} {
		if v, ok := hint[key]; ok && strings.TrimSpace(v) != "" {
			return sanitizeFileName(v)
		}
	}
	if ext := extFromMime(mimeType); ext != "" {
		return "upload" + ext
	}
	return "upload" + sniffExt(data)
}

// sanitizeFileName 去掉路径分隔符，避免传入 ../../etc/passwd 这类值。
func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "upload.bin"
	}
	return name
}

// valueAsFile 判定单个值是否为文件内容，是则还原出字节。
func valueAsFile(key string, value any) (data []byte, name, mimeType string, ok bool) {
	s, okVal := value.(string)
	if !okVal {
		return nil, "", "", false
	}
	return parseBase64Payload(key, s)
}

// parseBase64Payload 解析 data URI 或裸 base64 字符串。
func parseBase64Payload(key, raw string) (data []byte, name, mimeType string, ok bool) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "data:") {
		meta, enc, found := strings.Cut(s[len("data:"):], ",")
		if !found {
			return nil, "", "", false
		}
		isBase64 := false
		for _, part := range strings.Split(meta, ";") {
			p := strings.TrimSpace(part)
			switch {
			case p == "":
			case p == "base64":
				isBase64 = true
			case strings.HasPrefix(p, "name="):
				name = strings.TrimPrefix(p, "name=")
			case mimeType == "":
				mimeType = p
			}
		}
		if !isBase64 {
			// data:,xxxx 这类明文 URI 无法可靠还原为二进制，按普通字符串处理
			return nil, "", "", false
		}
		b, err := decodeBase64Forgiving(enc, false)
		if err != nil || len(b) == 0 {
			return nil, "", "", false
		}
		return b, name, mimeType, true
	}
	if !fileFieldKeys[key] {
		return nil, "", "", false
	}
	b, err := decodeBase64Forgiving(s, true)
	if err != nil {
		return nil, "", "", false
	}
	// 再过一道语义检查：解出来的东西要像文件内容，而不仅仅"能被解码"。
	if !looksLikeFileContent(b) {
		return nil, "", "", false
	}
	return b, "", "", true
}

// decodeBase64Forgiving 容错解码：忽略空白与换行，依次尝试标准 / URL 安全、带 padding / 无 padding。
//
// strict=true 时额外要求「解出来再编回去」与原串一致，用于把裸字符串误判为文件的概率降到最低。
func decodeBase64Forgiving(s string, strict bool) ([]byte, error) {
	var b strings.Builder
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\r' || r == '\n' {
			continue
		}
		b.WriteRune(r)
	}
	compact := b.String()
	if strict && len(compact) < minRawBase64Len {
		return nil, fmt.Errorf("长度不足，不认定为 base64")
	}
	candidates := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range candidates {
		decoded, err := enc.DecodeString(compact)
		if err != nil {
			continue
		}
		if len(decoded) == 0 {
			continue
		}
		if strict && enc.EncodeToString(decoded) != compact {
			continue
		}
		return decoded, nil
	}
	return nil, fmt.Errorf("不是合法的 base64")
}

// buildMultipartBody 把普通字段与文件拼成 multipart/form-data 请求体。
// 字段写入顺序经过排序，保证同样的入参得到同样的请求。
func buildMultipartBody(fields map[string]any, files []filePart) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	// 固定边界：这是一次进程内调用而非真实网络请求，不存在边界猜测攻击面；
	// 换来的是请求体逐字节可复现，便于回归测试与审计摘要比对。
	if err := mw.SetBoundary(multipartBoundary); err != nil {
		return nil, "", fmt.Errorf("设置 multipart 边界失败: %w", err)
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := mw.WriteField(k, scalarToString(fields[k])); err != nil {
			return nil, "", fmt.Errorf("写入表单字段 %s 失败: %w", k, err)
		}
	}
	for _, f := range files {
		w, err := mw.CreateFormFile(f.Field, f.Name)
		if err != nil {
			return nil, "", fmt.Errorf("创建文件字段 %s 失败: %w", f.Field, err)
		}
		if _, err = w.Write(f.Data); err != nil {
			return nil, "", fmt.Errorf("写入文件 %s 失败: %w", f.Name, err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return buf, mw.FormDataContentType(), nil
}

// looksLikeFileContent 对 AI 猜测出的"文件"做最后一道把关。
//
// 为什么需要：任意 ASCII 字符串都可能碰巧通过 base64 校验（"hello"、"1234567890123456" 都能解码），
// 单靠"能被解码"会把普通字符串误当成文件。真正的文件内容通常满足以下之一：
//   - 带已知文件头（图片/PDF/zip 等）；
//   - 含非文本字节（二进制文件）；
//   - 纯文本但足够长（短文本几乎一定是参数值而不是文件）。
func looksLikeFileContent(data []byte) bool {
	if len(data) < minDecodedFileBytes {
		return false
	}
	if hasKnownMagic(data) {
		return true
	}
	for _, c := range data {
		if c < 0x09 || (c > 0x0D && c < 0x20) || c > 0x7E {
			return true // 二进制特征
		}
	}
	return len(data) >= minDecodedTextBytes // 纯文本需更长的长度才采信
}

// minDecodedFileBytes 解码后至少这么大才可能是文件（小于此值一律按普通字符串处理）。
const minDecodedFileBytes = 16

// minDecodedTextBytes 纯文本内容的最小长度门槛。
const minDecodedTextBytes = 64

// hasKnownMagic 命中已知文件头。
func hasKnownMagic(data []byte) bool {
	if sniffExt(data) != ".bin" {
		return true
	}
	return false
}

// extFromMime 由 MIME 类型推导扩展名，带 "." 前缀。
//
// 注意不直接采用 mime.ExtensionsByType 的首个结果：image/jpeg 会返回 ".jfif"，
// 而上传场景更习惯 .jpg，故对常见类型按优先顺序修正。
func extFromMime(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	preferred := map[string]string{
		"application/octet-stream": ".bin",
		"image/jpeg":               ".jpg",
		"image/png":                ".png",
		"image/gif":                ".gif",
		"image/webp":               ".webp",
		"image/svg+xml":            ".svg",
		"image/tiff":               ".tiff",
		"image/avif":               ".avif",
		"video/mp4":                ".mp4",
		"video/webm":               ".webm",
		"application/pdf":          ".pdf",
		"application/zip":          ".zip",
		"application/json":         ".json",
		"text/plain":               ".txt",
		"text/csv":                 ".csv",
		"text/xml":                 ".xml",
	}
	if ext, ok := preferred[mimeType]; ok {
		return ext
	}
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

// sniffExt 按文件头魔数嗅探扩展名，用于在缺少文件名时给出合理后缀。
// 覆盖上传场景最常见的几类：图片、PDF、压缩包（docx/xlsx 同为 zip）。
func sniffExt(data []byte) string {
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return ".png"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}) {
		return ".jpg"
	}
	if len(data) >= 6 && string(data[:6]) == "GIF87a" || len(data) >= 6 && string(data[:6]) == "GIF89a" {
		return ".gif"
	}
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	if len(data) >= 4 && string(data[:4]) == "%PDF" {
		return ".pdf"
	}
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{'P', 'K', 0x03, 0x04}) {
		return ".zip"
	}
	if len(data) >= 3 && bytes.Equal(data[:3], []byte{0xEF, 0xBB, 0xBF}) {
		return ".txt"
	}
	if len(data) > 0 {
		switch data[0] {
		case '{', '[':
			return ".json"
		case '<':
			return ".xml"
		}
	}
	return ".bin"
}
