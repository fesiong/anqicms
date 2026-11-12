# AGENTS.md — anqicms 后端协作规范

> 本文件从项目长期记忆（`.workbuddy/memory/`）提炼，供 AI Agent 在本仓库工作时遵守。
> 每条 ⚠️ 背后都是真实踩过的坑，不要凭直觉"优化"掉。

## 1. 项目速览

| 项 | 值 |
|---|---|
| 模块路径 | `kandaoni.com/anqicms`（Go 1.26，iris + cloudwego/eino） |
| 仓库 | **单仓**：`~/data/backend/anqicms`，前端在其子目录 `anqicms-admin/`（2026-10-31 并入） |
| 前端目录 | `anqicms-admin/`（UmiJS + antd/ProForm，包名 `AnQiCMS`），源码随主仓跟踪，无独立 `.git` |
| 前后端关系 | **无代码级引用**，只通过 HTTP 契约耦合 |
| 前端 baseUrl | `'/system/api'` ↔ 后端 `app.Party("/system").Party("/api")`（`route/manage.go`） |
| 版本号 | `config/constant.go:3` 的 `const Version`（**不在 package.json**） |
| 本地 MCP | `http://127.0.0.1:8001/api/mcp` |

主要目录：`route/`（路由表，端点真相源）、`controller/manageController/`（后台 handler）、
`provider/`（业务 + 站点配置）、`model/`（GORM 模型）、`pkg/mcp/intent/`（意图内核）、
`library/`、`view/`（pongo2 模板引擎）、`cmd/{apicatalog,apilive}`（代码生成 + 冒烟工具）。

## 2. 构建 / 运行 / 测试命令

```bash
go build ./...                     # 改动后第一道
go vet ./...                       # 第二道
go run ./cmd/apicatalog            # 端点清单/api_catalog.json 重新生成（改 route/ 后必跑）
go run ./cmd/apicatalog -check     # 校验 catalog 与源码是否一致
go run ./cmd/apilive -mode all -ns "" -expose "*"   # 15/15 域冒烟（含写操作可逆验证）
go run ./cmd/apilive -wire         # MCP 协议层冒烟
```

**复测三板斧**：`apilive -mode all` → `apilive -wire` → `apicatalog -check`。

- ⚠️ `go test ./provider/` 整包会被沙箱 SIGTERM（exit 137，既有现象）→ **必须按 `-run` 分批跑**。
  `library` / `controller/manageController` / `pkg/mcp/intent` 三包可整跑。
- ⚠️ `make build` 走 `release/` 目录发布打包；日常验证**不要用**它（见下条）。

## 3. 本地起服务：只能用 `go run`

- `config.initPath()` 用 `ExecPath`，仅当二进制名含 `go_build` 才回退 `os.Getwd()`。
  → **`go build -o /tmp/x` 会读不到 config.json**，连错库、MCP 报 disabled。**切勿编译到 /tmp**。
- 正确姿势（后台任务模式）：
  ```bash
  go run kandaoni.com/anqicms/main > /tmp/anqicms_server.log 2>&1   # 可加 -port 8011
  ```
  判活看日志里的 `AI tools initialized (intent mode)`。
- ✅ `main.go` 支持 **`-port`**：8001 被占用时**不要杀原服务**，`-port 8011` 另起一个，测完即停。
- ⚠️ `ai_setting` 走进程缓存（TTL 86400s，`LoadAiSetting`）。**直接改 DB 的 `$.mcp.api_exposure.mode` 必须重启服务才生效**；8011 与 8001 共库，改库前先用进程缓存隔离判断影响面。

## 4. 意图内核（MCP 工具面）

链路：`route/manage.go`（端点真相源）→ `capEndpoints`（能力表）→ `pkg/mcp/intent`（意图目录）→ Eino / MCP 双适配器。

- 模型面 = **53 个意图工具**（`IntentCatalog`），是模型可见的**唯一出口**；端点与内置能力只作 cap，不进模型面。
  其中默认可见 28 个；商城合并补的 4 个（`commerce_marketing`/`commerce_fulfillment`/`commerce_review`/`seo_merchant`，见 `catalog_shop.go`）为 DefaultOff。
- ⚠️ **改 `route/manage.go` 增删端点后必须跑 `go run ./cmd/apicatalog`**，否则 `api_invoke` 报「未找到端点」。
- 15 个域映射在 `intent/domain.go`，未知 ns 报 `DomainUnknown`（**不兜底**）。
  三道闸门：`DefaultOff` → `ExposedIntents` → `ApiExposure`，**fail closed**。
- ⚠️ **改 `DefaultOff` 会牵动 4 处平行名单，漏一处就红**（2026-10-07 实测）：
  1. `intent.gatedDomainIntents`
  2. 测试内的 `intent.defaultOpenDomainIntents`
  3. `RecommendedExposed()`（**不能含 DefaultOff 意图**）
  4. `provider/api_intent_routes_test.go` 的 `mustStayGated` + `dailyOpen`（**provider 侧独立维护的一份**）
- ⚠️ `DefaultOff` 门禁原本只覆盖"补齐域"（`domainIntentCatalog`）；`catalog.go`/`catalog_merged.go` 的 19 个非补齐域意图曾完全无门禁，已补 `openNonDomainIntents`/`gatedNonDomainIntents` + `TestNonDomainIntentsExposureIsDeliberate`。新增非补齐域意图时同步登记。
- ⚠️ 意图层参数名 ≠ handler 字段名（`switchCompose` 只按 action 分派、**不改字段名**）。需要换名走 `invokeRoutes` 的 rename（例：`siteops_maintain` 的 `content` → 端点 `robots`）。**漏了不报错，只是值被静默清空**。
- ⚠️ `ParamSpec` 的类型必须与端点请求结构体的 **json tag 严格一致**。加 cap 时逐个核对"handler 有、schema 无"的参数。
- `StructuredContent` 统一信封 `{code,msg,ok,status,data}`；新增意图必须走它。
- ⚠️ **工具一旦返回 `data`，端点那句 `msg` 会被吞掉**：出口 `structuredObject` 对非信封的普通 map 调 `envelopeOf`，而它把 `msg` 写死成 `""`（只看得到业务载荷，看不到端点响应）。`data == nil` 时反而是文本直出、`msg` 完整 —— 所以「加个回执字段」这类改动会让提示语突然消失（2026-10-28 给 `user_create` 回 `data.id` 时实测：「保存成功」变空串）。出口已用 `mergeEndpointMsg(obj, res.Text)` 只补空位（不覆盖 Compose 自己写的 msg）。**新增/修改 Compose 时别指望 envelope 里的 msg**，需要提示语就自己写进 Data，或确认走文本路径。
- ⚠️ 业务失败必须置 `isError=true`（SDK 是 `omitempty`，不置会伪装成成功）。
- ⚠️ 掩码字段要配**回写保护**三件套：读时掩码 + 写端点识别哨兵沿用真值 + desc 说明。脱敏前先确认字段是值类型还是指针（`Website.PluginSendmail` 是 `*`，**必须值拷贝**，否则一次 `setting_get` 就把内存真值抹成哨兵）；写端点的 `currentSite.X = v` 是故意就地改，别"顺手修"。
- `mcp_set_scope` 受 `Config.EnableSetScope` 门控且**默认关闭**：其 SetScope 是**进程级全局**，一个客户端设窄会影响所有并发客户端。改成会话级之前**不要放开**。`mcp_list_intents` 只读，始终注册。

### 工具面 token 预算（2026-10-05 实测）
49 个意图 ≈ 16.4k token（params schema 74% / desc 25%）；2026-10-25 增至 53 个（新增 4 个商城意图均 DefaultOff，默认不进工具面，常驻成本不变）。省 token 杠杆排序：
① **整域裁剪**（配 `ExposedIntents`，零代码，content 域一步省 1965）→
② 压 desc 重复陈述（单工具仅 40~50 tok，性价比低）→
③ 拆参数组（**反向 +2.5 倍**，1245→3127：合并工具共享参数声明才是减法，`catalog_merged.go` 105→49 就是这么来的）。
⚠️ 压 desc 的真实价值是**消除歧义**不是省 token；原则是只删跨层级重复，独有语义一律保留，且**必须先写护栏测试**（丢语义不会让任何原有测试变红）。
⚠️ 别高估 enum 的成本（order_by enum 14→5 净省仅 ~3 tok）。"推荐 ⊂ 允许"要写成两个独立集合（`archiveOrderRecommended` / `archiveOrderColumns`）。

## 5. 设置端点的部分更新语义

- 设置类 POST 端点已支持**部分更新**：`library.MergeJSONPayload` 按 json tag 只覆盖请求体里出现的字段，未传字段保留现值，显式传空才算清空。MCP `system_config` 的 `values` **可放心只传要改的字段**。
- 覆盖范围 26 个同构端点（index + anchor/fulltext/guestbook/interference/order/push/retailer/rewrite/sendmail/sitemap/titleimage/user/watermark/weapp/wechat/storage/htmlcache + akismet/google/limiter/llms/jsonld/place/translate + multilang）。
- **判定不动**的：`diy_field` / `banner` / `sensitive_words`（列表语义）、ai 设置（密钥复合体）、`importapi`（已是非空才更新）、`timefactor`（已保留运行时字段）、`tag`（已有 Partial 开关）。
- 标准改造模式：**保留具名 `ctx.ReadJSON(&req)`**（apicatalog AST 靠它生成参数 schema）→ 值拷贝现值作基底 → `json.Unmarshal(ctx.GetBody(), &payload)` 取字段清单 → `MergeJSONPayload`。
  ⚠️ 二次 `ctx.ReadJSON` 会把 AST 的 schema 目标覆盖成 map，**params 归零**。
- ⚠️ **给 `ContentConfig` 等配置结构体加字段，必须同步 `SettingContentForm` 的手写赋值白名单**，否则 schema 有、保存被吞（只有端到端才暴露）。

## 6. 测试约定

- **反向对照（negative control）是硬要求**：每个护栏测试都要证明"改坏了会变红"。
  实测教训：只在 desc 里改文字没真改字段 → 对照不红；坏签名在任何错误下都被拒 → 必须先证明校验器抓得住错。
- **声明式路由/清单必须穷举校验目标存在**，用 **go/ast**，**别写死端点总数断言**。
- 断言关键语句数量而非日志文本；锚点要**从真实文件文本取**（gofmt 对齐变化会让凭印象写的锚点失效）。
- 现有护栏示例：`pluginSitemap_build_test.go`（用 ast 断言 `PluginSitemapBuild` 内不得出现 `SaveSettingValue`/`ReadJSON`，并反向断言 `PluginSitemapForm` **必须**有，防两条断言同时空转）、`view/django_test.go`、`provider/storage/r2_sigv4_test.go`。
- ⚠️ 仓库里 `provider/aiBuiltinTools.go`、`pkg/mcp/intent/catalog_api.go` 等**大量文件本来就 gofmt 不干净**（历史欠账），不要误当自己引入的；但**自己新增的文件必须 gofmt**。

## 7. MCP / 数据库 联调实操

- Bearer token：表 **`settings`（复数）**、行 **`key='ai_setting'`** 的 `$.mcp.token`。⚠️ 主键列是 `key` **不是 `name`**。
- JSON-RPC 流程：`initialize` → 取响应头 `Mcp-Session-Id` → `tools/call`。
  ⚠️ **响应是 SSE 形态**（`event: message` + `data: {...}`），要抽 `data:` 行再解析，直接管道 `json.loads` 会失败。
- ⚠️ `/usr/local/bin/mysql` 因缺 openssl@1.1 dylib **已无法运行**（且不在默认 PATH）。查库改用
  `/Users/fesion/.workbuddy/binaries/python/envs/default/bin/python` + `pymysql`。
- ⚠️ 改完 `pkg/mcp` 必须重编译重启才生效。
- 端到端验证套路：起 8011 临时服务 → 真实调 MCP → 验证 → 恢复 `api_exposure.mode=read` → 停 8011 → 必要时重启 8001。

## 8. 前端（`anqicms-admin/`）协作约定

前端源码在 `~/data/backend/anqicms/anqicms-admin/`，与后端**同一个 git 仓**（2026-10-31 并入；
此前是嵌在工作区里的独立仓库 + `.git/info/exclude` 排除，该限制已取消）。

- 该目录内**没有** `.git`，在里面跑 `git rev-parse --show-toplevel` 返回后端根目录 —— 这是正常的。
- 追踪范围：508 个源文件（`src/` `config/` `public/` + 各类配置文件），**不含**依赖与构建产物。
  `node_modules` / `dist` / `graphify-out` / `.DS_Store` 由 `anqicms-admin/.gitignore` 继承排除。
  ⚠️ `yarn.lock` 也被那份 `.gitignore` 忽略 → 主仓里没有前端锁文件，clone 后装依赖拿到的是浮动版本。
- 前端改动与配套后端改动**可以同批提交**，不必再拆仓库。
- ⚠️ **别再往 `.gitignore` 里写 `anqicms-admin/`**：那会让 graphify 跳过整个前端语料，
  而 `.graphifyignore` 只能额外排除、**无法把 `.gitignore` 排除的内容重新纳入**（见 `.graphifyignore` 顶部说明）。
- 前后端仍**无代码级引用**，只通过 HTTP 契约耦合（`baseUrl='/system/api'`）。
- ⚠️ **tab 式设置页必须提交全量字段**：`setting/system|content|safe|contact` 是 tab 条件渲染 + 后端整包覆盖保存，`onFinish` 只带当前 tab 字段会**清空其它 tab**。
  `onSubmit` 必须用 `formRef.current?.getFieldsValue(true)`。新增 tab 式设置页同样遵守。
  其余多 tab 页（ai/limiter/sitemap/multilang）改 `visitedTabs` **常驻挂载 + `display:none` 切换**。
- React 三元分支**同类型组件不卸载、异类型才真卸载**（实测 rc-field-form 默认 `preserve=true`）。判断"切 tab 丢输入"先看分支两侧组件类型。
- ⚠️ 前端 `npx tsc --noEmit` 全量在本沙箱**会被 SIGTERM**（umi 类型体系过重）。小改动改用 `./node_modules/.bin/esbuild` 语法校验，或 `--strict` 只查纯逻辑模块。
  ⚠️ 仓库**已有 38 个存量类型错误**（PlaceCascader/bizcharts/CopyToClipboard 等），判断"是否引入新错误"要看是否命中改动文件，**不能看总数**。
- ⚠️ `npm run format` 会动全仓，不要为几个文件跑它（噪声）。
- `config/menu.go` 的 `Path` 不只用于导航，还被当**管理员分组权限 KEY** 使用；改前端路由必须同步它。
  `DefaultMenuGroups` 只在首次安装/重建菜单时写库，已运行站点需触发菜单重新同步。

## 9. 已知地雷清单（改这些文件前先读）

- ⚠️ **pongo2 v6**：`FromBytes`/`newTemplateString` **不写** `set.templateCache`，预解析对渲染无效；真正进缓存的是 `FromCache`（按文件名）。**Set 的 loader fs 在创建时固化绑定模板目录 → 切模板必须重建 Set**。相关回归测试 `view/django_test.go`。
- ⚠️ **`POST /plugin/sitemap/build` 只按已保存配置生成，既不读也不存请求体**。配置保存只归 `PluginSitemapForm`（`POST /plugin/sitemap`）。意图层 `seo action=sitemap` **不再传 `type`/`auto_build`**（传了静默失效），护栏 `TestSeoSitemapSendsNoConfigParams` + `TestSitemapBuildMustNotPersistConfig` 双锁。
- ⚠️ **验证码有两套独立实现，必须成对改**：`manageController.GenerateCaptcha`（看 `Safe.AdminCaptchaOff`）与 `controller.GenerateCaptcha`（看 `Safe.Captcha`）。`captcha_id` 是一次性凭据（有 TTL，校验即失效），两个端点都必须带 `Cache-Control: no-store`，且要写在 `captcha_off` 提前 return **之前**。
- ⚠️ `archives` 表**没有 `status` 列也没有 `content` 列**：正文在 `archive_data` 表，草稿在 `archive_drafts`。`content_article action=save` 传 `draft=false` 即发布，草稿 link 带 `?preview=true`。
- ⚠️ `parseArticle` 优先取外层字段：`POST /archive/detail` 的 `data.data` 是窄对象，`link`/`title` 在外层。
- ⚠️ **skill 端点早就齐全**（`route/manage.go` 五个 `/anqi/skill/*`），别重造内存 cap。判断端点是否存在要 `grep -rn "func Skill"` 追 handler，**只搜路由字符串会漏**。
- `AutoMigrateDB(db, force=false)` 只在 `lastVersion != config.Version` 时才真迁移 → 新模型在老库**不建表**，需后台「修复数据库」。
- 模板包用 `package`（字符串）寻址；改模板走 `fs_write`/`fs_edit` + `system_config` 的 `template_reload`。
- `api` 意图 = 端点通用发现/调用（list/schema/invoke）。判断筛选是否生效看 **`matched` 不是 `total`**；`schema` 不传 `method` 默认 GET。
- `endpointFailure` 靠纯文本前缀判定失败（`错误：`/`未找到匹配的文件`/`精确匹配失败`/`⚠ 警告：`）。

## 10. Agent 子系统

- 表 `ai_agents`；调度器**不在 crond**，是 `provider/aiChat.go` 里 `(*AiChatService).StartAgentScheduler()` 起的 goroutine（在 `aiChat.go:292` 被调用），每 30 秒轮询一次。
- ⚠️ 三道门槛任一不满足则**静默不执行**：① `eino.GlobalConfig()` 非 nil；② `Enabled==1`；③ `NextRunAt>0 && <=now`。
- ⚠️ cron 周期必须 > 单次耗时（`max_rounds=8` ≈ 30 秒）；被跳过的 tick **不顺延，错过永久丢失**。
- ⚠️ 标题配图未配置时 Agent 会卡死（`GenerateAllTitleImages` 全站扫描，只支持 .ttf）。

## 11. 硬教训（通用，任何改动都适用）

1. **发现 bug 时改 desc 掩盖实现 = 埋雷。** 先问"是 desc 写错了还是实现写错了"。
2. **循环里用 `return`/`continue` 跳过尾部公共代码 = 同类 bug 高发区。** 公共代码放循环体最前面；写完用测试断言关键语句数量而非日志文本。
3. **异步任务失败必须留"为什么没跑"的可观测线索。**
4. **否定判断必须直调验证。** 只 grep 路由字符串会漏；应追 handler、查真实库、真实调 MCP。
   同理：`InvalidAccessKeyId` 在 Ceph RGW 上**不能按字面理解**，必须做"不存在的 bucket"对照（404 vs 403）才能区分"key 无效"与"key 无权"。
5. **map 迭代不确定 → 排序构造。** `FormFile` 返回 3 值；`GetContentType()` 恒空，用 `GetHeader`。
6. **估算"省 token / 省开销"前必须先算。** 实测多次推翻直觉（拆参数组反向 +2.5 倍；压 enum 净省 3 tok）。
7. **注释与文档里的数字必须实测核准。** 凭印象写的"白名单 15 列"实为 14 列。降级性结论要写进代码，防后人当样板套。

## 12. Git 与发布

- 提交按**功能分批**，每批要能独立编译 + 测试通过。前端与后端同一仓，配套改动可与后端同批提交。
- ⚠️ 跨批次文件必须手工拆：先 `cp` 备份到 /tmp → `git checkout HEAD --` 还原 → 只改本批部分提交 → 从 /tmp 拷回完整版。**备份在批次之间必须一直留着**。配套测试文件也要跟着拆。
- 修订已提交内容用 `GIT_EDITOR=true git commit --fixup=amend:<sha>` + `git rebase --autosquash`（未推送时安全）。注意该 git 版本**不支持 `-m` 与 `--fixup` 同用**。
- 发布清单**只有 4 个文件**：`CHANGELOG.md` / `README.md` / `README-en_US.md` / `config/constant.go`。
  ⚠️ `release/` 被 `.gitignore` 忽略且从未入库，**不要 `git add release/...`**（曾导致 commit 没带文件、tag 打到上一个提交）。
- CHANGELOG 分节沿用既有格式：新增功能 / 功能优化 / 问题修复（有安全项再加 🔒 段）。
- ⚠️ **打 tag 后必须 `git show --stat <tag>` 核对指向的提交与文件清单**，别只看 tag 打成功。

## 13. 与用户的协作方式

- 用户习惯**自然语言下指令**，期望 AI 自己选对工具，不需要他指定技术细节。
- **测试全程不动源码**：临时验证靠另起端口的临时服务 + 临时脚本，验证完恢复现场（mode 恢复 read、删临时数据、停临时服务）。
- **先汇总问题再动手修**，不要边查边改。
- 关注 **GEO（生成式引擎优化）**，内容类改动优先考虑 AI 检索/引用场景。
- 站点现状参考：约 268 篇文章、9 个分类、2 个页面、58 个附件。

## 14. XERJ 参考编码（reference coding）

本机跑着一个本地检索节点（`http://localhost:9200`，单二进制、ES 兼容协议）。
**写非平凡代码前先查它，不要凭记忆猜上游 API。**

节点不是常驻服务，重启机器后需要重新拉起：

```bash
xerj --insecure --data-dir ~/xerj-data &      # 起节点
curl -s localhost:9200 | head -c 60           # 判活
```

### 已建好的两个语料

| 语料 | 内容 | 检索命令 |
|---|---|---|
| `anqicms` | 本仓 905 个源码/文档文件（8419 条记录） | `xerj search --prefix anqicms "<关键词>"` / `xerj def --prefix anqicms <符号>` |
| `anqicms-refs` | 上游参考仓：modelcontextprotocol/go-sdk、cloudwego/eino、cloudwego/eino-ext | `xerj code anqicms-refs "<要解决的问题>" --lang go` |

用法约定：

- `--prefix` 必须带上，否则查到的是默认空前缀 `ax`（等于查不到）。
- `xerj code` 命中会给出 `仓库 文件:行号`，**引用时要带上这个坐标**；回答与代码不符就回去读那一行，别继续往下写。
- **当前节点是 lexical 模式**（默认 feature-hash，索引里没有向量字段）。所有检索都是词法匹配，
  匹配的是你输入的词，**不是语义理解**。`--mode hybrid` / `--mode semantic` 在 lexical 节点上
  仍然用 feature hashing，不会变成真语义 —— 别把它当"换个更聪明的模式"用。
- 词法检索的实际用法：**换关键词再试**，或先 `xerj def <符号>` 定位再读文件。
  要真正的语义召回得重起节点 `xerj --embed-mode neural`（~90MB 模型，且两个语料都要重建）。
- 语料超过 30 天会被 `xerj code` 拒绝（防止拿旧上游骗人）。刷新方式：
  `xerj corpus index anqicms-refs`（重新 clone + 建索引）。
- 本仓索引是快照，改完代码要重建：
  `xerj autoindex . --prefix anqicms --state-dir ~/xerj-state/anqicms --plan code-and-docs`
- 上游用 --depth 1 clone 在 `~/.xerj-code/corpora/anqicms-refs`，**那是只读参考，不要在里面改代码**。

参考编码的判断流程：`xerj code` 看上游怎么写 → 打开命中的文件确认签名与契约 → 再落到本仓的实现。
本仓自己的实现位置用 `xerj search --prefix anqicms` 找，比逐目录 grep 快。

> 已设 `XERJ_DISABLE_FEEDBACK=true`：不要向 xerj 提交 field report（它会在公开仓库开 PR）。
