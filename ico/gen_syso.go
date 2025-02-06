//go:build windows
// +build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
)

//go:embed anqicms
var sysoData []byte

func main() {
	err := os.WriteFile("anqicms.syso", sysoData, 0644)
	if err != nil {
		fmt.Println("生成 syso 失败:", err)
		os.Exit(1)
	}
}
