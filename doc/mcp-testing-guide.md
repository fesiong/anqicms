# MCP 工具全量测试方法论

> 来源：2026-10-03 对 51 个 MCP 工具的完整测试。这是**流程文档**，不是结果记录
> （结果见 `.workbuddy/memory/2026-10-02.md`）。下次新增工具或改意图层时照此执行。

## 一、准备

### 1.1 拿清单与契约

- 工具总览：`doc/tools.txt`（`tools/list` 的文本快照，51 个工具）
- 精确契约：调 MCP 的 `tools/list`，拿 `inputSchema.properties` 与 `action.enum`
- **字段真相源**：`provider/api_catalog.json` 里每个端点的 `params`（name + type + desc）。
  意图层的 desc 可能有错，**以端点为准**。加过 REST 路由后必须跑：
  ```bash
  go run ./cmd/apicatalog          # 重新生成
  go run ./cmd/apicatalog -check   # 校验一致
  ```
  该文件是 `//go:embed` 编译期嵌入的，不重新生成则新端点在 MCP 侧等于不存在。

### 1.2 测试脚手架

纯 Python urllib（**不要在 heredoc 里写 curl**，会被安全策略拦）：

```python
# /tmp/mcp.py 关键结构
init()                          # 建会话，存 Mcp-Session-Id
call(tool, action, **kw)        # 统一调用；遇 502/连接断开自动 init() 重试一次
brief(r) / pp(r)                # 摘要 / 完整打印
dat(r)                          # 取信封里的 data
schema_of(name) / actions(name) # 查契约
```

**崩溃自愈是必须的**：后台异步任务 panic 会打挂进程，后续调用全 502。
`call()` 里捕获 `502`/`closed connection`/`Connection` 后重建会话重试。

### 1.3 重启服务

```bash
cd <repo> && go run kandaoni.com/anqicms/main > /tmp/anqicms_server.log 2>&1
```

用 `run_in_background=true`。**必须 `go run`，不能 `go build -o /tmp/xxx`** ——
`config.initPath()` 靠二进制路径定位 `config.json`，编到 `/tmp` 会读不到配置、连错库、
AI/MCP 不初始化。

判活：日志出现 `MCP intent kernel registered` + `Agent scheduler started`，
且 `lsof -ti :8001` 有占用。

## 二、逐个工具的测试顺序

按 `action.enum` **逐个**测，每个 action 至少覆盖：正常路径 → 回读验证 → 错误路径。

```
1. 只读 action（list/get/detail）先测完 —— 无副作用，可放心跑
2. 配置类（*_save）必须：先 get 备份 → 改 → 回读 → **回滚并验证回滚成功**
3. 数据类（create）建测试数据 → 回读 → delete → 确认已删
4. 破坏性 action（transfer/backup/upgrade/website）只测「无参数/不存在 id」的拒绝行为，不实际执行
5. 有异步副作用的（collector start/dig、cache_build、group_send）注意它们会真的触发任务
```

### 2.1 必查的 6 个高危模式

这 6 类占实测发现问题的三分之二：

| 模式 | 症状 | 检查方法 |
|---|---|---|
| **全量覆盖** | 只传一个字段，其余被清零 | 见 2.2.1。**先查根因**（控制器是否写死了默认值），别急着在调用层补齐 |
| **类型不匹配** | `json: cannot unmarshal X into ... of type Y` | 对照 api_catalog 的 `params[].type` |
| **字段名不符** | 传 A 端点要 B → 静默忽略 / ReadJSON 失败 | 对照 `params[].name` |
| **失败报成功** | `ok:false` 但回执 `ok:true` | 看 `text` 原文，别只看 `structuredContent` |
| **语义取反** | 「已完成」被报成错误 | 读端点源码确认 `nil` 的真实含义 |
| **nil panic** | 进程崩溃，后续全 502 | 查日志 `panic:` 段 |
| **查不到≠失败** | 对不存在的 id 回「已更新/已删除」 | 见 2.1.1，AnQiCMS 最隐蔽的一类 |
| **静默空结果** | `ok:true` + 空列表 + 正确的 total | 见 2.1.2 |

#### 2.1.1 「查不到不算错误」——AnQiCMS 写端点的系统性陷阱

`provider` 里所有批量写操作都用 GORM 的 `Find` 查目标：

```go
w.DB.Model(&model.ArchiveDraft{}).Where("`id` IN (?)", req.Ids).Find(&drafts)
for _, d := range drafts { ... }   // 查不到 → 空切片 → 循环零次执行
ctx.JSON(iris.Map{"code": config.StatusOK, "msg": ctx.Tr("ArticleUpdated")})
```

**`Find` 查不到时 `error` 是 `nil`**（`Find` 不像 `First` 会返回 `ErrRecordNotFound`），
所以控制器完全无法感知，所有分支都回成功。实测 `publish id=999999`、
`delete id=999999` 都回 `ok:true`。

**判定与修法**：

```python
# 测法：用绝不存在的 id，逐个写动作试
call('content_article', 'publish', id=999999, status='ok')   # 应报错
call('content_article', 'delete',  id=999999)                 # 应报错
```

在意图层补 `requireArchivesExist`（`catalog_merged.go`）：

- 写操作前先用对应的 `*_get` 探活
- **只认明确的 `record not found`**，5xx / 网络异常一律放行 ——
  把端点抖动误报成「文档不存在」比不检查更糟，会让本可成功的操作凭空失败
- 存在性校验的读失败**不要**当成不存在

⚠️ 探活用 `*_get` 有个前提：它得和写操作读同一套模型。文档的 detail
先查草稿表再查正式表，与 publish/delete 的表优先级一致，所以适用。

#### 2.1.2 静默空结果 ——词法校验 ≠ 语义校验

排序类参数最容易中招。`ParseOrderBy`（`provider/apiService.go:1485`）
只做**词法**安全校验（`fieldNameRegex` 匹配即放行），**不校验列是否存在**：

```
order_by=created_time  → "archives.created_time desc"  ✅
order_by=nonexistent   → "archives.nonexistent desc"   ← MySQL Unknown column
```

控制器丢弃了 `Find` 的 error，于是 `ok:true` + `list:[]` + `total:1856`。
AI 看到 `ok:true` 会认为「筛选条件太窄所以没数据」，得出完全相反的结论。

**测法**：每个字符串入参都试一个明显非法的值，看是否静默。

```python
for col in ['nonexistent_col', 'bad', '']:
    call('content_article', 'list', order_by=col)
```

修法是加白名单（`archiveOrderColumns`），不是改成正则——排序字段是有业务含义的
列名，猜不出调用方想按什么排，**只校验不纠正**。

⚠️ 端点把 `order_by` 重命名为 `sort`（`capEndpoints`），而排序方向那个叫 `order`；
查端点时别按意图层的参数名去 grep。

### 2.2 全量覆盖的识别与回滚

```python
bak = dat(call(tool, 'xxx_get'))          # 1. 备份
open('/tmp/bak.json','w').write(json.dumps(bak, ensure_ascii=False))
call(tool, 'xxx_save', values={'one_field': 1})   # 2. 只改一个
now = dat(call(tool, 'xxx_get'))
lost = [k for k, v in bak.items() if now.get(k) != v]   # 3. diff
call(tool, 'xxx_save', values=bak)         # 4. 回滚
assert all(dat(call(tool,'xxx_get')).get(k) == v for k, v in bak.items())  # 5. 验证
```

⚠️ **回滚备份必须是「污染前」取的**。踩过：先测了 `values:{x:1}` 把 password 清空，
再取备份 —— 备份里已经是空值，回滚无效。**顺序是：先备份，再测。**

#### 2.2.1 「未传字段被清零」——先查根因，别在调用层打补丁

**已根治（2026-10-03）**，这里是当时的分析与最终修法，供同类问题参考。

**根因不在调用方。** `request.*` 的 `UpdateAll` 本来就带 `json:"update_all"` tag，
是留给调用方决定的，但 8 处控制器全都**无条件** `req.UpdateAll = true`：

```bash
grep -rn "UpdateAll = true" controller/     # 8 处
```

那个 `true` 是为**表单提交**准备的：前端提交整个表单（`values` 来自 antd Form），
某字段没出现 = 用户清空了它。**AI/程序化调用的语义正相反：没传 = 别碰。**
两种语义共用一个字段，而控制器替调用方做了选择。

**修法：加一个反向开关，别动既有字段的类型。**

```go
// request 结构体
UpdateAll bool `json:"update_all" ast:"-"`
// Partial 走 PATCH 语义：只覆盖显式传入的字段。
Partial   bool `json:"partial" ast:"-"`

// 控制器
if !req.Partial { req.UpdateAll = true }
```

⚠️ **为什么用 `Partial` 而不是直接暴露 `update_all`**：Go 的 `bool` 零值
**无法区分「没传」和「传了 false」**。前端恰恰不传该字段，若把默认值改成 false
就会让前端表单漏存；改成 `*bool` 又会波及 provider 里 85 处 `if req.UpdateAll || …`。

调用方侧用 **Fixed**（常量注入）而非 Defaults（缺省补齐）：

```go
var partialUpdate = map[string]any{"partial": true}
"archive_update": {…, Fixed: partialUpdate},
```

Fixed 无条件覆盖，**调用方无法关掉 PATCH**；Defaults 允许覆盖，误传
`partial:false` 就会悄悄退回全量覆盖——正是要消除的行为。

`ast:"-"` 让 api_catalog 生成器排除它，所以它不出现在 catalog 里是**正确的**
（`TestIntentParamsRecognizedByEndpoints` 需要把它加进 `exempt`）。

#### 2.2.2 ⚠️ 先分清两类成因，再决定补齐还是 partial

**全量覆盖有两种不同的成因，修法相反。** 判定前必须先分类：

| 类 | provider 写法 | `partial` 是否有效 | 意图层补齐 |
|---|---|---|---|
| ① 有 `UpdateAll` 开关 | `if req.UpdateAll \|\| req.X != ""` | ✅ 有效 | ❌ 冗余**且有风险** |
| ② 逐字段无条件赋值 | `material.Title = req.Title` | ❌ 没有开关 | ✅ 唯一防线 |

```bash
# 判定方法 —— 不能只搜 `req.UpdateAll = true`，那只是类 ①
grep -c "req.UpdateAll" provider/<资源>.go              # 0 → 类 ②
grep -n "req\.[A-Z][A-Za-z]* = req\." provider/<资源>.go # 类 ② 的特征
```

实测分布：`user.go` 15 处 `UpdateAll`（类 ①）；`material.go`/`place.go` **0 处**、
`SaveUserGroupInfo` 也是逐字段赋值（类 ②）。

**类 ① 不要补齐。** 我第一反应是回查旧值补齐（`preserveOnUpdate`/`fillMissingOnUpdate`），
它能工作，但因此连续踩了两个坑：

1. **读端点会覆写字段** → 补齐把派生值写回去
   （`GetNavList` 返回前执行 `tmpList[i].Link = w.GetUrl("nav", ...)`，
   而 `GetUrl` 对 `nav_type=0` 恒返回 `/`。实测把真实外链
   `http://example.com/keepme` 改写成 `http://127.0.0.1:8001/`——**比清空更隐蔽**，
   因为派生值看起来完全正常。）
2. **数据源要额外维护** → 列表端点按参数分组（导航 `type_id`），
   我先硬编码 `{0,1}`（漏掉 `type_id=2` 那 3 条），再改成动态取分组…

两次都是在解一个**本不该存在**的问题。改成 PATCH 后「不传就不碰」，
上面两类问题从根上消失。

**类 ② 仍需补齐**（`invokeRouteOverwriteFields` + `preserveOnInvokeRoute`，
在 `catalog_domains.go`）。它与 `partialInvokePaths` **互斥**——
同一端点不能同时登记在两处，否则两套机制叠加。

> **判断信号**：修复某个问题需要「回查 + 数据源 + 分支兜底」时，
> 先停下来问 —— 是不是在解决一个不该存在的问题？往上翻一层看默认行为是为谁设计的。

> **清理时的盘点判据**：按「**机制**」而不是按「函数名」grep。
> 我删掉 `switchCompose` 那套补齐时，漏了 `invokeRoutes` 里一套**同名但独立**的
> 实现（`preserveOnInvokeRoute`）——`grep 补齐函数名` 找不到它。
> 正确问法是「**还有哪些路径能触发全量覆盖**」。

### 2.3 破坏性 action 的边界测试

**用「不存在的 id」而不是真实 id。**

踩过：测 `system_multilang delete` 时想当然以为「主站会被拒绝」，
结果它真把 `settings.multi_lang.sub_sites` 里的一条子站配置删了。

```python
call(tool, 'delete', id=999999)     # 安全：不存在的 id
# 而非 call(tool, 'delete', id=1) # 危险：可能是真实数据
```

## 三、发现 bug 后的定位与修复

> 追加于 2026-10-03：两条实测教训

### 3.0 循环里的 return/continue 是 bug 高发区（同构错误，一天犯两次）

- `provider/subscriber.go` 群发：渲染失败时 `return` 漏 `continue`，一封出错整批漏发
- `provider/aiChat.go` loadAgentsFromDB：过期分支 `continue` 漏注册，过期 Agent 根本没进 `svc.agents`

共同特征：**在循环里用跳转语句跳过尾部公共代码。**

```go
// 错：公共代码在尾部，被 continue 跳过
for _, x := range items {
    if expired(x) { log(...); continue }  // ← 跳过了下面的注册
    advance(x)
}
svc.cache[x.id] = x   // 永远执行不到

// 对：公共代码放最前
for _, x := range items {
    svc.cache[x.id] = x      // 先注册
    if expired(x) { log(...); continue }
    advance(x)
}
```

**写完用测试断言关键语句的位置与数量**，别只断言日志文本 ——
第一次写的测试就因为只查「有没有那句日志」而漏掉了这个 bug。

### 3.0.1 desc 与实现不一致时，先判断该改哪边

发现「desc 说的和实现做的不一样」时，**不要直接改 desc 去迁就实现** ——
那等于把 bug 文档化，还让人以为已处理。正确顺序：

1. 读实现，判断它是**有意为之**还是**写错了**
2. 有意为之（如刻意的向后兼容取舍）→ 才改 desc，并在 desc 里写清取舍
3. 写错了 → 改实现，desc 保持描述正确行为

2026-10-03 我曾把 `fs_glob` 的 desc 改成「实测不支持 `**`」来掩盖实现缺陷，
被用户指出后改实现，才发现底下还压着「Agent 调度器从未真正执行过」这个更严重的问题。

### 3.0.2 平台相关的标准库行为要实测，别想当然

`filepath.ToSlash` 在**非 Windows 平台是 no-op**（实测 macOS 上
`ToSlash("a\b")` 原样返回 `a\b`）。做路径归一化时不要依赖它，
用 `strings.ReplaceAll(s, "\\", "/")` 才有跨平台一致行为。

### 3.0.3 glob 类 pattern 的实现要点

标准库 `filepath.Match` 语法正确（支持 `*` ? `[]`），但**每段匹配、不跨分隔符**。
要支持 `**` 必须自己按 `/` 分段后递归组合。两个易错点：

1. **以 `**` 开头的 pattern**（如 `**/catalog_*.go`）：若保留首段空串，
   `**` 吃掉零层后会剩「空段 + catalog_*.go」匹配不上单段的 `catalog.go`。
   切分时要丢弃空段。
2. **多个 `**`**（如 `a/**/b/**/c`）：拆成 prefix/suffix 的写法只能处理一个，
   必须递归。

### 3.1 端点业务失败 ≠ Go error

`api_invoke` 遇到 `{"ok":false,"msg":"..."}` **不返回 error**，只写在 `Text` 里。
所以 `callCap` 不会报错，Compose 若无条件构造回执就会**把失败报成成功**。

统一判定（`pkg/mcp/intent/catalog.go` 的 `endpointFailure`）覆盖三种：

```go
status >= 500                      // 控制器没写响应体，iris 返回默认 500
ok == false                        // 业务失败
双层信封的 data.code != 0          // 控制器层失败
```

### 3.2 类型/字段转换放哪

- `capEndpoints`（`cap_routes.go`）已有 `Rename` / `Arrays` / `Fixed` / `Defaults`
  → **优先用它们**，别在 Compose 里重复实现
- 意图对外的字段名与端点不同时（如 `id` → `order_id`），用 `Rename`
- 需要值转换时（如枚举字符串 → uint），在 Compose 分支里显式转换
- 转换函数要**统一返回类型**（都用 `int64`），混用 int/int64 会被测试抓到

### 3.3 nil panic 分两层

异步任务里的 panic **会带走整个进程**。两类都要防：

```go
w2 := provider.GetWebsite(id)
if w2 == nil { return }                        // 第一层：函数可能返回 nil
w2.MarkHtmlCacheFinished()                     // 第二层：字段是指针且可能为 nil
```

第二层最隐蔽：`Build*Cache` 在 `Open==false` 时**直接 return，不分配 Status 字段**，
调用方随后裸写 `w2.HtmlCacheStatus.XXX` 就 panic。
正解是加 nil 安全方法（内部懒分配），而不是在每个调用点判 nil。

### 3.4 测试要锁住修复

- **desc 类修复** → 加 desc 断言测试（`mustHaveAll` 辅助函数）。desc 是写给 AI 看的，
  最容易在后续「优化描述」时无声丢失。
- **panic 修复** → 读源文件断言守卫/安全方法存在（`os.ReadFile` + `strings.Contains`）。
- **行为修复** → 用**真实响应形状**做 fixture，不要自造结构。

### 3.5 写完修复必须做反向对照，否则测试可能空转

新加的回归测试**自己也会错**。断言写歪了、mock 形状对不上，
测试会一直绿，而 bug 其实还在。判据只有一个：**临时把修复改回错误版，测试必须红。**

```bash
cp pkg/mcp/intent/catalog_merged.go /tmp/x.bak
# 把修复代码替换成注释/删掉
python3 - <<'PY'
p='pkg/mcp/intent/catalog_merged.go'
s=open(p).read()
old='''原修复代码'''
assert old in s, "PATTERN NOT FOUND"   # ← 关键！见下
s=s.replace(old,'''// NEGATIVE CONTROL''')
open(p,'w').write(s)
PY
go test ./pkg/mcp/intent/ -run '受影响用例'   # 必须 FAIL
cp /tmp/x.bak pkg/mcp/intent/catalog_merged.go && rm /tmp/x.bak
go test ./pkg/mcp/intent/                      # 必须全绿
```

⚠️⚠️ **`assert old in s` 不能省**。本轮踩过：替换模式与源码实际写法不符
（我按记忆写 `range []string{...}`，源码已改成 `delete` 形式），
`str.replace` 静默什么都不做 → 测试「通过」了 → 我差点把
**空转测试当成有效验证**。加了 assert 后立刻正确报错。

**「测试没红」有两种原因，assert 用来区分**：① 修复没被真正移除（模式没匹配上）；
② 测试本身是空转。只有 assert 能告诉你该信哪个。

另外注意**被测分支的边界**：桩通常只拦到 `Compose → cap` 这一层，
而 `capEndpoints` 的 `Rename`（如 `order_by → sort`）发生在更外层，桩里**看不到**。
断言字段名时要清楚自己能看到哪一层，否则会写出永远抓不到 bug 的用例。

两个关键点：

1. **必须用 `-run` 精确选中受影响用例**，且确认「红的原因是断言不通过」而非编译错误
2. **还原后必须再跑一次全包**，确认字节一致、没有残留的对照代码

本轮实测：移除 3 处修复 → 4 个用例全红；移除回收站分支 → 4 个别名全红；
禁用 save 分支的参数剔除 → `TestSaveKeeps` 由绿转红（补强断言后）。
反过来，**穷举校验测试会主动抓你**——加 `status=delete` 枚举时
`TestIntentParamConsistency` 立刻失败（它把三值写死了）；给 `order_by` 加运行时白名单后
`TestIntentParamsRecognizedByEndpoints` 报「参数未被端点认识」，直接暴露出
**列表参数被透传到非列表端点**这个此前没人发现的真缺陷。
这类测试的价值正在于此：改契约必然连带改测试，测试不会静默失效成永久绿灯。

### 3.6 改白名单/枚举前先想清楚谁在依赖它

给参数加运行时校验时，**下游还有穷举门禁测试**在用哨兵值探测参数去向
（如 `provider/api_intent_routes_test.go` 的 `TestIntentParamsRecognizedByEndpoints`）：

- 带 `Enum` 的参数走**合法值分支**（`p.Enum[0]`），不会被运行时校验拦
- 不带 `Enum` 的走**唯一哨兵值**（`«param_name»`），会被白名单拦下而报「无法执行」

所以「运行时白名单」通常应该**同时**暴露成 `Enum`：既让 AI 从 schema 直接看到合法值，
又让下游门禁走合法分支。构造 enum 时记得**排序**——map 迭代顺序随机，
不排序会让 `tools/list` 每次响应的 enum 顺序都不同。

## 四、收尾核对

1. `go build ./...` / `go vet`（注意 `controller/common.go:436` 是既有告警）/ 全量 `go test`
2. 重启服务，**逐条复验修复项**（不是只看「服务能起来」）
3. 全站数据核对：把每个 list 类 action 的条数与测试前基线对比
   - ⚠️ 基线要**当场记**，事后回忆容易记错（本轮误把「会员 1500+」「系统预置的微信菜单」
     当成测试残留）
4. 差异项逐个查清是「真残留」还是「我记错基线」，不要含糊过去
5. ⚠️ **别用 SQL 结论替代接口结论去判「删除是否生效」**。本轮查 `archives` 表发现
   id 还在就判成「删除失败」，实际是查错了表——`delete` 把记录移到了 `archive_drafts`。
   判据用**接口**：`status=delete` 的 list 里能查到，才是真的进了回收站。

## 五、本项目特有的高频坑

| 坑 | 说明 |
|---|---|
| 改 `route/manage.go` 后必须跑 `go run ./cmd/apicatalog` | `api_catalog.json` 是编译期嵌入的 |
| 密码/密钥类字段「全量覆盖 + 掩码」必须三处一起改 | 读端点掩码 / 写端点识别哨兵值 / desc 说明 |
| 读端点掩码要**值拷贝**不能改指针 | `setting := *currentSite.PluginX`，指针类型改字段会污染内存全局配置 |
| 配置类字段启动时载入内存 | 改数据库不热更新，需重启 |
| `archives` 表**没有 status 列** | 已发布文档无 status；草稿/回收站状态都在 `archive_drafts`（0 草稿 / 99 待发布 / ContentStatusDelete 回收站）。**查 `archives` 别写 `SELECT ... status`**，会报 Unknown column 让人误以为表坏了 |
| 删正式文档 ≠ 物理删除 | `DeleteArchive` 是移到 `archive_drafts` status=99；**再删一次**才物理删除。所以「删除后 `get` 仍查得到」是正常的 |
| `flag` 是**另一张表** | `archive_flags`（INNER JOIN 过滤），不是 `archives` 列。库里 0 条时 `flag=` 过滤返回空是**数据真相**，不是 bug |
| 前台读草稿必须带 `?preview=true` | 实测带后缀 200、不带 404 |
| mysql 客户端连库 | `mysql -h127.0.0.1 -P3306 -uroot -proot`（密码取 `config.json` 的 `mysql.password`） |
| provider 整包测试会被沙箱 SIGTERM | 按 `-run` 分批跑 |
