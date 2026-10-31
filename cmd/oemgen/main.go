// Command oemgen 校验品牌数据并生成前端注入文件。
//
// 为什么需要它：品牌内容必须同时喂给后端（go:embed 进二进制）和前端（编译进
// JS chunk）。允许两边各自手写，就一定会出现“后台叫甲、前台叫乙”的漂移，
// 而且只有客户截图时才会发现。所以真相源只有 config/oem/<id>/brand.json 一处。
//
// 用法（仓库根目录执行）：
//
//	go run ./cmd/oemgen                 # 校验全部品牌，并按官方品牌生成前端文件
//	go run ./cmd/oemgen -oem acme       # 同上，但生成 acme 的前端文件
//	go run ./cmd/oemgen -check          # 只校验（不写文件），有问题退出码 1
//	go run ./cmd/oemgen -list           # 列出已登记的品牌
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"kandaoni.com/anqicms/config"
)

func main() {
	oemID := flag.String("oem", config.DefaultOEMID, "要生成前端文件的品牌标识")
	check := flag.Bool("check", false, "只校验品牌数据，不写文件；有问题退出码 1")
	list := flag.Bool("list", false, "列出已登记的品牌标识")
	current := flag.Bool("current", false, "打印当前二进制编译期固化的品牌（用于核对产物）")

	flag.Parse()

	// -current 直接读编译期标识，不受 -check 的加载校验影响：
	// 产线上拿到一个包时，这是唯一能确认“它到底是谁的”的手段。
	if *current {
		b := config.Brand()
		out, err := json.MarshalIndent(b, "", "  ")
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(out))
		return
	}

	// 先暴露加载期问题（JSON 语法、目录名与 id 不一致、未知字段）。
	if err := config.OEMLoadError(); err != nil {
		fatal("品牌数据加载失败：\n%v", err)
	}

	ids := config.OEMIDs()
	if len(ids) == 0 {
		fatal("config/oem/ 下没有任何 brand.json")
	}
	if _, ok := config.LookupOEM(config.DefaultOEMID); !ok {
		fatal("缺少官方品牌 config/oem/%s/brand.json：它是 OEMID 的默认值与未知标识的兜底来源，不允许删除", config.DefaultOEMID)
	}

	if *list {
		for _, id := range ids {
			b, _ := config.LookupOEM(id)
			fmt.Printf("%-16s %s\n", b.ID, b.Name)
		}
		return
	}

	// 每个品牌都要能完整渲染，缺字段的配置不允许进入构建。
	problems := 0
	for _, id := range ids {
		b, _ := config.LookupOEM(id)
		for _, e := range b.Validate() {
			fmt.Fprintf(os.Stderr, "config/oem/%s/brand.json: %s\n", id, e)
			problems++
		}
	}
	if problems > 0 {
		fatal("共 %d 处品牌配置问题，已中止", problems)
	}

	if *check {
		fmt.Printf("品牌数据校验通过（%d 个：%s）\n", len(ids), joinIDs(ids))
		return
	}

	brand, ok := config.LookupOEM(*oemID)
	if !ok {
		fatal("未登记的品牌 %q，可用：%s", *oemID, joinIDs(ids))
	}

	root, err := repoRoot()
	if err != nil {
		fatal("%v", err)
	}
	if err := writeFrontendBrand(root, brand); err != nil {
		fatal("%v", err)
	}
	fmt.Printf("已生成前端品牌文件（%s）\n", brand.ID)
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}

// repoRoot 从 cwd 向上找 go.mod，使工具在仓库任意子目录都能跑。
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("向上查找 go.mod 未命中，请在仓库内运行")
		}
		dir = parent
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
