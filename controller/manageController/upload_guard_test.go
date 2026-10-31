package manageController

import (
	"strings"
	"testing"
)

// 本文件守护「上传图片落地时扩展名由谁决定」。
//
// 事故背景：2026-10-31 uploads 目录出现 shell.php5 / s2.phtml。
// 插件里的水印图、标题图上传走的是「image.Decode 校验内容 + 原样写盘」，
// 使用者可以上传一张合法 GIF（尾部追加 <?php ... ?>）并命名为 x.phtml，
// 落地后就是可执行的图片马。内容校验挡不住这种 polyglot。

func TestImageFormatExtMapsKnownFormats(t *testing.T) {
	cases := map[string]string{
		"jpeg": ".jpg",
		"jpg":  ".jpg",
		"png":  ".png",
		"gif":  ".gif",
		"webp": ".webp",
		"bmp":  ".bmp",
		"tiff": ".tiff",
	}
	for format, want := range cases {
		if got := imageFormatExt(format); got != want {
			t.Errorf("imageFormatExt(%q) = %q, 期望 %q", format, got, want)
		}
	}
}

func TestImageFormatExtNeverEchoesUnknownFormat(t *testing.T) {
	// 关键断言：这个函数的入参虽然是 image.Decode 的 format，但一旦有人
	// 改成「用户文件名后缀」直接传进来，下面这些值就是木马后缀。
	// 白名单外一律回落 .png，绝不把入参原样拼成后缀。
	dangerous := []string{
		"phtml", "php5", "php", "pht", "phar",
		"<?php", "../shell", "sh", "jsp", "",
	}
	for _, format := range dangerous {
		got := imageFormatExt(format)
		if got != ".png" {
			t.Errorf("imageFormatExt(%q) = %q, 期望回落 .png", format, got)
		}
		if strings.ContainsAny(got, "<>/\\") {
			t.Errorf("imageFormatExt(%q) 产出含路径/标签字符的后缀 %q", format, got)
		}
	}
}
