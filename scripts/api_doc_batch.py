#!/usr/bin/env python3
"""批量给后台 handler 补 API 文档注释。

用途：端点表 provider/api_catalog.json 是**从 Go 源码注释派生**的，
想让端点/参数带上说明，唯一正确的地方是 controller 源码注释（改 JSON 会被重新生成覆盖）。
本脚本负责"按映射表插入注释"这一步，省去逐个文件手工编辑。

用法：

    1. 看还缺哪些：go run ./cmd/apicatalog -report
    2. 读源码确认语义（**不要凭路径猜**），把结果填进下面的 DOCS
    3. 运行：python3 scripts/api_doc_batch.py
    4. 重新生成并校验：go run ./cmd/apicatalog && go run ./cmd/apicatalog -check

安全约束：
    - 只插入注释，不改动任何代码行；
    - 目标函数上方若已有含该函数名的注释则跳过（重复运行不会重复插入）；
    - 插入后必须 go build + gofmt 验证。

格式约定见 doc/api-catalog-annotations.md：首行按 godoc 惯例以函数名开头，
「参数说明：」段落每行一个 `- 查询参数 "x": 说明`。
"""
import os
import sys

BASE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "controller", "manageController")

# handler -> (文件名, 摘要, [(参数名, 说明)])
# 只填**能从源码确认**的语义；不确定的参数宁可留空，错误描述比没有描述更危险。
DOCS = {
    # ── 已补批次（保留作格式示例）──────────────────────────────
    "GetWebsiteList": ("website.go", "获取多站点的站点列表，支持分页和名称搜索。", [
        ("current", "当前页码，默认为 1。"),
        ("pageSize", "每页条数，默认为 20。"),
        ("name", "站点名称模糊搜索。"),
    ]),
    "AttachmentList": ("attachment.go", "分页查询附件列表，支持按分类与关键词筛选。", [
        ("current", "当前页码，默认为 1。"),
        ("pageSize", "每页条数，默认为 20。"),
        ("category_id", "按附件分类 ID 筛选，0 表示不限。"),
        ("q", "按文件名关键词模糊搜索。"),
    ]),
    # ── 待补：往下面继续加 ────────────────────────────────────
}


def build_comment(name, summary, params):
    lines = ["// %s %s" % (name, summary)]
    if params:
        lines.append("//")
        lines.append("// 参数说明：")
        for p, d in params:
            lines.append('//   - 查询参数 "%s": %s' % (p, d))
    return lines


def insert(fn, name, comment_lines):
    path = os.path.join(BASE, fn)
    if not os.path.exists(path):
        print("!! 文件不存在 %s" % fn)
        return False
    lines = open(path, encoding="utf-8").read().split("\n")
    target = None
    for i, l in enumerate(lines):
        if l.startswith("func %s(ctx " % name):
            target = i
            break
    if target is None:
        print("!! 未找到函数 %s (%s)" % (name, fn))
        return False
    # 已存在判定：向上扫描**整段连续注释**，只要块内出现函数名就跳过。
    # 只看紧邻上一行是不够的——注释块最后一行是「参数说明」的列表项，不含函数名，
    # 会导致重复插入（实测踩过）。
    j = target - 1
    while j >= 0 and lines[j].strip().startswith("//"):
        if name in lines[j]:
            return False
        j -= 1
    lines[target:target] = comment_lines
    open(path, "w", encoding="utf-8").write("\n".join(lines))
    print("++ %-32s %s" % (name, fn))
    return True


def main():
    n = 0
    skipped = 0
    for name, (fn, summary, params) in DOCS.items():
        if not insert(fn, name, build_comment(name, summary, params)):
            skipped += 1
        else:
            n += 1
    print("\n新增 %d 个，跳过 %d 个（已有注释）" % (n, skipped))
    if n:
        print("下一步：gofmt -l controller/manageController/ && go build ./... && go run ./cmd/apicatalog")
    return 0


if __name__ == "__main__":
    sys.exit(main())
