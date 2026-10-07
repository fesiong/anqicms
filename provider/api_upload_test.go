package provider

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

// pngFixture 一个足够长的 PNG 头，满足 looksLikeFileContent 的长度门槛。
var pngFixture = []byte{
	0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
}

// jpegFixture 用于验证 URL 安全字母表与魔数嗅探。
var jpegFixture = []byte{
	0xFF, 0xD8, 0xFF, 0xE0, 0x11, 0x22, 0x33, 0x44,
	0x55, 0x66, 0x77, 0x88, 0x99, 0xAA, 0xBB, 0xCC,
}

// TestParseBase64Payload 校验 data URI 与裸 base64 两种输入格式的还原能力。
func TestParseBase64Payload(t *testing.T) {
	std := base64.StdEncoding.EncodeToString(pngFixture)

	cases := []struct {
		name     string
		key      string
		raw      string
		wantOK   bool
		wantName string
	}{
		{"data URI 带 mime", "file", "data:image/png;base64," + std, true, ""},
		{"data URI 带 name", "file", "data:image/png;name=logo.png;base64," + std, true, "logo.png"},
		{"data URI 不带 mime", "file", "data:;base64," + std, true, ""},
		{"data URI 无 fields", "whatever", "data:application/zip;base64," + std, true, ""},
		{"裸 base64 文件字段", "file", std, true, ""},
		{"裸 base64 换行容差", "file", std[:6] + "\n" + std[6:], true, ""},
		{"URL 安全字母表", "file", base64.URLEncoding.EncodeToString(jpegFixture), true, ""},
		// 防误判：以下都不得被当成文件
		{"data URI 明文非 base64", "file", "data:text/plain,hello", false, ""},
		{"普通短字符串", "file", "hello", false, ""},
		{"普通 URL", "file", "https://example.com/a.png", false, ""},
		{"非文件字段名不给裸base64特权", "cover", std, false, ""},
		{"数字不误判", "file", "1234567890123456", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, fn, _, ok := parseBase64Payload(c.key, c.raw)
			if ok != c.wantOK {
				t.Fatalf("ok=%v want %v", ok, c.wantOK)
			}
			if !ok {
				return
			}
			if len(data) == 0 {
				t.Fatal("解码结果为空")
			}
			if fn != c.wantName {
				t.Fatalf("name=%q want %q", fn, c.wantName)
			}
		})
	}
}

// TestPartitionFileParams 校验分流：文件单独成份，普通字段保留，base64 不进 query。
func TestPartitionFileParams(t *testing.T) {
	png := base64.StdEncoding.EncodeToString(pngFixture)
	files, rest, err := partitionFileParams(map[string]any{
		"file":        png,
		"category_id": 3,
		"purge":       true,
	}, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("期望 1 个文件，得到 %d", len(files))
	}
	if files[0].Field != "file" {
		t.Fatalf("字段名错误: %s", files[0].Field)
	}
	if len(rest) != 2 {
		t.Fatalf("剩余字段应为 2 个，得到 %d: %v", len(rest), rest)
	}
	if _, leaked := rest["file"]; leaked {
		t.Fatal("base64 内容不应留在普通字段里（会撑爆 query）")
	}
}

// TestPickFileName 校验文件名优先级与路径穿越防护。
func TestPickFileName(t *testing.T) {
	data := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if got := pickFileName("", nil, data, ""); got != "upload.png" {
		t.Fatalf("魔数嗅探应得到 upload.png，实际 %s", got)
	}
	if got := pickFileName("", nil, data, "image/jpeg"); got != "upload.jpg" {
		t.Fatalf("mime 应优先得到 upload.jpg，实际 %s", got)
	}
	if got := pickFileName("", map[string]string{"file_name": "a/b/c/seed.txt"}, data, ""); got != "seed.txt" {
		t.Fatalf("应取 file_name 提示且剥离路径，实际 %s", got)
	}
	if got := pickFileName("../../etc/passwd", nil, data, ""); got != "passwd" {
		t.Fatalf("路径穿越未被剥离: %s", got)
	}
}

// TestBuildMultipartBody 校验 multipart 构造的确定性与完整性。
func TestBuildMultipartBody(t *testing.T) {
	files := []filePart{{Field: "file", Name: "a.png", Data: []byte{1, 2, 3}}}
	fields := map[string]any{"id": 7, "tag": "x"}
	buf1, ct1, err := buildMultipartBody(fields, files)
	if err != nil {
		t.Fatal(err)
	}
	buf2, _, err := buildMultipartBody(fields, files)
	if err != nil {
		t.Fatal(err)
	}
	if buf1.String() != buf2.String() {
		t.Fatal("同样的入参应产出完全一致的 multipart（顺序必须确定）")
	}
	if len(ct1) < 20 || ct1[:19] != "multipart/form-data" {
		t.Fatalf("content type 不正确: %s", ct1)
	}
	body := buf1.String()
	for _, want := range []string{`name="file"`, `filename="a.png"`, `name="id"`, `name="tag"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("multipart 缺少 %s", want)
		}
	}
}

// TestUploadTooLargeIsRejected 校验超大文件被拒绝而非打爆内存。
func TestUploadTooLargeIsRejected(t *testing.T) {
	big := base64.StdEncoding.EncodeToString(make([]byte, maxUploadFileBytes+1))
	if _, _, err := partitionFileParams(map[string]any{"file": "data:application/octet-stream;base64," + big}, nil); err == nil {
		t.Fatal("超过体积上限时应报错")
	} else {
		fmt.Printf("超大文件被正确拒绝: %v\n", err)
	}
}
