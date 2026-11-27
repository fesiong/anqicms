// Package segmenter 持有进程级共享的 sego 中文分词器。
//
// 为什么单独拎一个包出来（2026-11-16 客户 106 站点事故的后续）：
// 词典是几十万词条的文件，sego 把它摊成 cedar 前缀树 + token 切片，加载完
// 就一直常驻，而且**只读** —— 天生可以共享。实测官方词典（5.3MB、35 万词条）
// 加载耗时 0.84s，堆常驻 +124MB，进程 Sys 峰值约 218MB。
// 但改造前有两处各自加载：
//
//   - provider.WordSplit 用 provider 包里的全局 Segmenter；
//   - provider/fulltext 的悟空引擎每个站点 NewWukongService 时，靠
//     SegmenterDictionaries 自己 LoadDictionary 一遍。
//
// 106 个站点就是 106 份词典常驻 + 106 次加载，机器直接被吃满，"启动期间
// 端口迟迟不开" 有一部分就是它。现在两边都走 Shared()：进程内只有一份，
// 加载一次（sync.Once），可以并发使用（Segment 只读词典，Jump/Value 不改状态）。
package segmenter

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/huichen/sego"
	"kandaoni.com/anqicms/config"
)

// DictionaryFileName 是词典文件名。发布包里它与可执行文件同目录。
const DictionaryFileName = "dictionary.txt"

// minReasonableTokens 是"这词典看起来像真的"的下限。
// 仓库里的开发用占位词典只有 1 个词条（10 字节），能加载、分词结果却等于没分，
// 属于典型静默失效；低于这个数量就告警，别让人以为分词在工作。
const minReasonableTokens = 1000

// Status 是词典加载结果快照，字段名直接用于日志/诊断输出。
type Status struct {
	Loaded  bool   `json:"loaded"`
	Path    string `json:"path,omitempty"`
	Tokens  int    `json:"tokens"`
	Elapsed string `json:"elapsed,omitempty"`
	Error   string `json:"error,omitempty"` // 不可用时的原因（中文，可直接给用户看）
}

var (
	once   sync.Once
	shared *sego.Segmenter
	state  Status
	// loads 只在 once 内自增，用来给测试证明"确实只加载了一次"。
	loads int
)

// Shared 返回进程内唯一的 sego 分词器；词典不可用时返回 nil。
//
// 调用方必须处理 nil（退回"不分词"），不要假设它一定可用：
// 词典文件缺失、损坏、或磁盘只读时都可能拿不到。
func Shared() *sego.Segmenter {
	once.Do(load)
	return shared
}

// Ready 报告词典是否已加载可用。
func Ready() bool {
	return Shared() != nil
}

// StatusNow 返回加载结果快照。会按需触发加载，因此首次调用可能耗时数秒。
func StatusNow() Status {
	once.Do(load)
	return state
}

// LoadCount 返回词典实际加载**尝试**次数（正常恒为 0 或 1）。
func LoadCount() int {
	once.Do(load)
	return loads
}

func load() {
	started := time.Now()
	// 计数放在最前面：失败也是一次尝试。放在成功后会让"从没开始加载"
	// 和"加载失败"都表现成 0，排查时分不清。
	loads++
	path, err := resolvePath()
	if err != nil {
		state = Status{Error: err.Error()}
		// 这是可用性优先的降级：没有词典不该让进程起不来（历史上
		// sego.LoadDictionary 会 log.Fatalf 直接 os.Exit），但必须吵一声，
		// 因为分词、关键词、标题图、悟空全文搜索都会跟着降级。
		slog.Error("中文分词词典不可用：分词将退化为整串返回，悟空全文搜索无法初始化",
			"reason", err.Error())
		return
	}

	seg := &sego.Segmenter{}
	ok := true
	func() {
		// LoadDictionary 在文件打不开时直接 log.Fatalf（os.Exit，recover 拦不住），
		// 所以前面必须先 Stat 确认存在；这里这层 recover 只兜解析期的 panic
		// （词典格式坏了、token 位置越界之类）。
		defer func() {
			if r := recover(); r != nil {
				ok = false
				state = Status{Path: path, Error: fmt.Sprintf("词典解析失败：%v", r)}
				slog.Error("中文分词词典解析失败，分词功能降级", "path", path, "panic", r)
			}
		}()
		seg.LoadDictionary(path)
	}()
	if !ok {
		return
	}

	tokens := 0
	if dict := seg.Dictionary(); dict != nil {
		tokens = dict.NumTokens()
	}
	shared = seg
	state = Status{
		Loaded:  true,
		Path:    path,
		Tokens:  tokens,
		Elapsed: time.Since(started).Round(time.Millisecond).String(),
	}
	slog.Info("中文分词词典已加载（全站共用一份）",
		"path", path, "tokens", tokens, "cost", state.Elapsed)
	if tokens < minReasonableTokens {
		slog.Warn("分词词典词条过少，分词结果可能等于没分（发布包里应带完整词典）",
			"path", path, "tokens", tokens, "expectAtLeast", minReasonableTokens)
	}
}

// resolvePath 是"去哪儿找词典"的唯一入口。
// 抽成变量只为一件事：测试要能构造"词典不存在"的场景，
// 验证降级是记日志，而不是 sego.LoadDictionary 里的 os.Exit。
var resolvePath = func() (string, error) {
	return resolveIn(candidateDirs())
}

// resolveIn 在给定目录里找词典，返回第一个存在且非空的文件。
// 目录顺序 = candidateDirs()：可执行文件目录优先（发布包把 dictionary.txt
// 和主程序放在一起，而服务多由 systemd / 计划任务拉起，工作目录并不固定），
// 其次才是当前工作目录。抽成独立函数是为了能脱离进程级 once 单测
// "找不到 / 空文件" 这两条降级路径。
func resolveIn(dirs []string) (string, error) {
	var tried []string
	for _, dir := range dirs {
		p := filepath.Join(dir, DictionaryFileName)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() {
			tried = append(tried, p)
			continue
		}
		if info.Size() == 0 {
			tried = append(tried, p+"（空文件）")
			continue
		}
		return p, nil
	}
	return "", fmt.Errorf("%s 未找到（已查找：%s）", DictionaryFileName, strings.Join(tried, "、"))
}

func candidateDirs() []string {
	var dirs []string
	if config.ExecPath != "" {
		dirs = append(dirs, config.ExecPath)
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}
	// 去重但保序：ExecPath 在测试环境下就等于仓库根，会和 cwd 撞上。
	seen := make(map[string]bool, len(dirs))
	uniq := make([]string, 0, len(dirs))
	for _, p := range dirs {
		if seen[p] {
			continue
		}
		seen[p] = true
		uniq = append(uniq, p)
	}
	return uniq
}
