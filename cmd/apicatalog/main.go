// Command apicatalog 生成后台端点元数据表 provider/api_catalog.json。
//
// 为什么需要它：端点参数绑定信息（handler 内 `var req request.Archive`）只能从源码
// AST 得到，编译后即消失。生产环境只发布二进制、没有 .go 源码，因此必须在编译期把
// 这份元数据固化下来，随二进制一起发布（go:embed）。
//
// 用法（仓库根目录执行）：
//
//	go run ./cmd/apicatalog              # 重新生成 provider/api_catalog.json
//	go run ./cmd/apicatalog -check       # 只检查是否过期，不写文件；有差异退出码 1
//	go run ./cmd/apicatalog -out /tmp/x.json
//	go generate ./provider               # 等价的 go:generate 入口
//
// 产物是一行一条端点的 JSON，**允许人工编辑**（例如补 desc、覆盖 risk/domain）。
// 手工编辑后 -check 会报差异，这是预期行为，不必强求一致。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kandaoni.com/anqicms/provider"
)

func main() {
	out := flag.String("out", "", "输出路径，默认 <仓库根>/provider/api_catalog.json")
	check := flag.Bool("check", false, "只比对现有表与源码解析的差异，不写文件；有差异退出码 1")
	report := flag.Bool("report", false, "列出仍缺描述的端点与参数，用于决定下一批补哪些注释；不写文件")
	merge := flag.Bool("merge", false, "生成时用旧表补齐缺失描述（默认关闭：JSON 应忠实反映源码，删掉的注释必须同步消失）")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		fatal("%v", err)
	}
	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(root, "provider", "api_catalog.json")
	}

	// 源码解析是唯一真值来源：嵌入表只是它的快照。
	src, err := provider.BuildAPICatalogFromSource()
	if err != nil {
		fatal("从源码派生目录失败（需要 controller/manageController 与 request 源码）: %v", err)
	}
	stats := src.CatalogStats()
	fmt.Printf("源码解析：%d 个端点（struct %d / urlparam %d / form %d / multipart %d / none %d），命中结构体 %d 个\n",
		len(src.Endpoints), stats.Struct, stats.URLParam, stats.Form, stats.Multipart, stats.None, src.StructTouched)

	if *report {
		printTodoReport(src)
		return
	}

	old, oldErr := loadFile(outPath)

	if *check {
		if oldErr != nil {
			fatal("读取现有表失败: %v", oldErr)
		}
		d := provider.DiffCatalogs(old, src)
		printDiff(d)
		if d.Empty() {
			fmt.Println("✔ 端点表与源码一致")
			return
		}
		fmt.Printf("✘ 端点表已过期，请运行：go run ./cmd/apicatalog\n")
		os.Exit(1)
	}

	// 正常流程下不做 merge：JSON 要忠实反映源码，删掉的注释必须同步消失。
	// -merge 用于"确实在 JSON 里手工补过内容"的例外场景。
	if *merge && oldErr == nil {
		src = provider.MergeCatalogPreserve(src, old)
	}
	if err := writeCatalog(outPath, src); err != nil {
		fatal("写入失败: %v", err)
	}
	fmt.Printf("✔ 已生成 %s（%d 个端点）\n", rel(root, outPath), len(src.Endpoints))
	printDomainStats(src)
	if oldErr == nil {
		if d := provider.DiffCatalogs(old, src); !d.Empty() {
			fmt.Println("\n与上一版差异：")
			printDiff(d)
		} else {
			fmt.Println("与上一版无差异")
		}
	}
}

// printTodoReport 输出"还缺什么"，用来决定下一批补哪些 handler 注释。
//
// 排序逻辑：先按缺失端点数降序给出域（成片补比零散补划算），
// 再给出"最该先补"的清单——读端点优先（模型调用频率最高），去重到资源粒度。
func printTodoReport(cat *provider.APICatalog) {
	byDomain := map[string]int{}
	byDomainTotal := map[string]int{}
	missing := make([]provider.EndpointMeta, 0)
	for _, ep := range cat.Endpoints {
		// 硬规则拦截的端点（login/captcha/password/aigenerate）永远不会被调用，不纳入待补
		if provider.EndpointBlockReason(ep) != "" {
			continue
		}
		byDomainTotal[ep.Domain]++
		if ep.Desc == "" {
			byDomain[ep.Domain]++
			missing = append(missing, ep)
		}
	}
	domains := make([]string, 0, len(byDomainTotal))
	for k := range byDomainTotal {
		domains = append(domains, k)
	}
	sort.Slice(domains, func(i, j int) bool {
		if byDomain[domains[i]] != byDomain[domains[j]] {
			return byDomain[domains[i]] > byDomain[domains[j]]
		}
		return domains[i] < domains[j]
	})
	fmt.Printf("\n缺端点描述：%d / %d（%.0f%%）\n", len(missing), len(cat.Endpoints),
		float64(len(missing))*100/float64(len(cat.Endpoints)))
	for _, d := range domains {
		fmt.Printf("  %-12s 缺 %3d / %3d\n", d, byDomain[d], byDomainTotal[d])
	}

	noParamDesc := 0
	paramTotal := 0
	for _, ep := range cat.Endpoints {
		for _, p := range ep.Params {
			paramTotal++
			if p.Desc == "" {
				noParamDesc++
			}
		}
	}
	fmt.Printf("\n缺参数说明：%d / %d（%.0f%%）\n", noParamDesc, paramTotal,
		float64(noParamDesc)*100/float64(paramTotal))

	// 建议优先补：读端点（调用频率最高）+ 按资源去重
	seen := map[string]bool{}
	var picks []provider.EndpointMeta
	for _, ep := range missing {
		if ep.Risk != "read" {
			continue
		}
		if seen[ep.Resource] {
			continue
		}
		seen[ep.Resource] = true
		picks = append(picks, ep)
	}
	sort.Slice(picks, func(i, j int) bool {
		if picks[i].Domain != picks[j].Domain {
			return picks[i].Domain < picks[j].Domain
		}
		return picks[i].Path < picks[j].Path
	})
	fmt.Printf("\n建议优先补（读端点、按资源去重）共 %d 个：\n", len(picks))
	for i, ep := range picks {
		if i >= 40 {
			fmt.Printf("  … 其余 %d 个\n", len(picks)-40)
			break
		}
		fmt.Printf("  %-6s %-46s %s\n", ep.Method, ep.Path, ep.Handler)
	}
}

// writeCatalog 输出「一行一条端点」的 JSON：便于 git diff 与人工编辑。
func writeCatalog(path string, cat *provider.APICatalog) error {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("  \"generated_at\": " + mustJSON(time.Now().Format(time.RFC3339)) + ",\n")
	sb.WriteString("  \"source\": " + mustJSON(cat.Source) + ",\n")
	sb.WriteString("  \"struct_touched\": " + fmt.Sprint(cat.StructTouched) + ",\n")
	sb.WriteString("  \"endpoints\": [\n")
	for i, ep := range cat.Endpoints {
		raw, err := json.Marshal(ep)
		if err != nil {
			return err
		}
		sb.WriteString("    ")
		sb.Write(raw)
		if i < len(cat.Endpoints)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  ]\n}\n")
	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func loadFile(path string) (*provider.APICatalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c provider.APICatalog
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	return &c, nil
}

func printDiff(d provider.CatalogDiff) {
	if len(d.Missing) > 0 {
		fmt.Printf("  + 源码新增、表中缺失 %d 个：\n", len(d.Missing))
		for _, k := range d.Missing {
			fmt.Println("      " + k)
		}
	}
	if len(d.Extra) > 0 {
		fmt.Printf("  - 表中有、源码已无 %d 个：\n", len(d.Extra))
		for _, k := range d.Extra {
			fmt.Println("      " + k)
		}
	}
	if len(d.Changed) > 0 {
		fmt.Printf("  ~ 元数据变化 %d 个：\n", len(d.Changed))
		for _, k := range d.Changed {
			fmt.Println("      " + k)
		}
	}
}

func printDomainStats(cat *provider.APICatalog) {
	count := map[string]int{}
	for _, ep := range cat.Endpoints {
		count[ep.Domain]++
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, count[k]))
	}
	fmt.Println("  域分布：" + strings.Join(parts, " · "))
}

// repoRoot 定位仓库根（route/manage.go 的上两层）。
func repoRoot() (string, error) {
	manage, err := provider.FindManageRoute()
	if err != nil {
		return "", err
	}
	return filepath.Dir(filepath.Dir(manage)), nil
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		fatal("序列化失败: %v", err)
	}
	return string(raw)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
