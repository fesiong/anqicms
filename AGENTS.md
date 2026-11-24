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

## 14. 站点库表迁移（多站点）

> 2026-11-16 客户 106 站点事故后立的规矩。事故现象：升级 3.8.4 后「前十几个站点正常、
> 后面全部初始化失败」，日志里 **0 条报错**、6489 条慢 SQL，启动 2.5 分钟只走到第 75 站。

- ⚠️ **迁移禁止 `go` 出去**：`InitWebsite` 的顺序固定为
  **迁移 → InitSetting → InitModelData → 注册**。106 个站点并发迁移会把 MySQL 的
  DDL 打满（实测平均 33 倍、峰值 75 倍并行），而迁移与 `initArchiveSchema` 抢同一张
  `archives` 表；站点先注册则请求会读到半迁移的表结构。
- 统一入口 `provider.MigrateSiteSchema`（并发闸门默认 2，`ANQI_MIGRATE_CONCURRENCY`
  可调；错误必进 slog；有进度日志）。新增调用点不要绕过它。
- ⚠️ 并发初始化让**注册顺序不再等于 id 顺序**（先迁完的先注册）。`OrderedWebsites.Values()`
  因此改为按 id 排序 —— 域名匹配是"第一个命中就返回"，同域名下主站 + 二级目录站点时，
  先注册的那个会赢；排回 id 升序才恢复串行时代的确定性（护栏 `TestWebsitesValuesSortedById`）。
- 启动期**站点初始化本身也按同一容量并行**（`initSitesConcurrently`，站点 1 先串行跑完，
  父子站点等父站点完成后再派发）。三条不能退：① 站点 1 先跑（默认库/后台/主站）；
  ② 子站点等父站点；③ worker 里的 panic 记栈后重新抛，交给 `InitWebsitesAsync` 标 failed ——
  就地吞掉会让阶段被误标为 ready。护栏 `siteInitRunner_test.go`。
- ⚠️ **空转 ALTER 是真凶之一**：全仓 752 列写成 `gorm:"type:varchar(32) not null"`，
  非空被塞进类型字符串 → gorm `field.NotNull == false` 但 DDL 仍输出 NOT NULL →
  每轮迁移都发一批"改完不变"的 `MODIFY COLUMN`（实测 660 条 / 193 秒，且不收敛）。
  迁移入口用 `normalizeModelFieldNullability` 兜住（768 字段，DDL 一字不差）。
  根治是把 tag 改成 `type:varchar(32);not null` —— 改完记得删 shim，
  `TestInlineNotNullDebtIsTracked` 会变红提醒。
- 离线迁移（把迁移从启动里剥离，可分批/可预演）：
  ```bash
  ./anqicms -migrate-only -migrate-dry-run          # 只看哪些站点要迁，不发 DDL
  ./anqicms -migrate-only -migrate-sites 1,3,9-20   # 分批
  ./anqicms -migrate-only -migrate-concurrency 1    # 库扛不住时降并发
  ```
  该分支**不走** ScanPort / checkProcesses（迁移不占端口，客户常在服务开着时分批跑）。
- ⚠️ 具名返回值才能让 `defer` 改到返回值；`res := X{}; defer func(){res.Y=...}(); return res`
  改的是副本（离线迁移汇总耗时一度恒为 0s）。

### 14.1 ⚠️ 启动顺序：端口先开，迁移在后台

> 2026-11-16 事故的第二幕：把迁移改成同步（14 的改动）之后，它卡在
> `go bootstrap.Start()` 之前 —— 106 个站点升级期间「进程活着、日志不动、
> 端口几十分钟不开」，运维无法区分"在跑"和"卡死"，客户反复重启。

**顺序固定为**（`bootstrap.Serve`）：

```
bootstrap.Init()          // 路由 + i18n + 模板引擎，毫秒级
provider.PrepareStartupGate()   // 有默认库时先关闸门（必须在监听之前）
go bootstrap.Listen()     // 端口立刻监听
provider.InitWebsitesAsync()    // 后台跑站点初始化（含迁移），闸门限流
crond.Crond()
```

- ⚠️ **不允许**在 `Serve` 里直接调 `provider.InitWebsites()`（同步）。
  护栏 `TestServeListensBeforeSiteInit` 会变红。
- ⚠️ **放行粒度是站点，不是进程**。106 个站点同时只有 2 个在迁移/初始化，**迁完的那个立刻能服务**，
  不该陪着跑完最后一个（否则升级窗口 = 所有站点耗时之和）。判定在
  `provider/siteAccess.go`：该请求归属的站点**已注册 + `Initialed` + 打过可服务标记** → 放行，
  否则 503 + `Retry-After: 10`（HTML 页自动刷新；`Accept: application/json` 或 `/api/*` 返回 JSON 进度）。
  闸门必须是 `loadGlobalMiddleware` 里**第一个**中间件。
- ⚠️ **"注册"不等于"可服务"**：`InitWebsite` 先 `websites.Set`（`GetRewritePattern` 要回写注册表里同一个
  对象），之后才建 limiter/cache。可服务标记（`markSiteServable`）打在函数末尾，`RemoveWebsite` /
  重新初始化时清掉。别把两者合成一个 —— 合成就会出现"Cache 还是 nil 就被放给线上流量"。
- 后台请求带 `Site-Id` / `X-Site-Id` / `Sub-Site-Id` 时，**声明的站点优先于域名**：
  否则会拿站点 1 的就绪状态放行一个要操作未迁移站点的请求，那条请求会落回站点 1，写错库。
- `GET /__startup` 是唯一无条件放行的端点，同时当**就绪探针**（全部站点处理完才 503→200，
  响应体是进度 JSON）。别改成"永远 200 + ready 字段"——负载均衡只看状态码。
- 日志必须有心跳：初始化期间每 15 秒一条
  `站点初始化进行中：已处理 x/y（迁移 a、跳过 b、失败 c），当前 #id「名称」、#id「名称」，已耗时 …`
  （并发的站点会按 id 升序一起列出来，`SetStartupCurrent`/`ClearStartupCurrent` 成对使用）。
  没有它，单站点回填几分钟的静默和"卡死"又是一回事。
- 闸门有四态必须分清：`idle`（无默认库/全新安装，**放行**）、
  `running`（**按站点放行**）、`ready`（放行）、`failed`（初始化 panic，**全站保持关闭**直到重启 ——
  那时注册表可能是"部分就绪、部分半初始化"，按站点放行等于赌某个站点没被 panic 影响）。
- ⚠️ 站点注册表的不变式没变：**迁移成功才注册**。所以闸门不能为了"让后台能看进度"
  提前注册半初始化站点 —— 那正是 2026-11-15 那次 500 的成因。
- ⚠️ 还没轮到的站点**做不到"照常访问"**：新代码读 3.6.6 的旧表（`archives` 缺列、
  `archive_products` 不存在）会 500。它们只能是 503，页面上显示"排队中/正在升级"。

## 15. 进程级共享资源（sego 词典是第一个）

> 2026-11-17：词典从「每站点/每次一份」改成「全进程一份」。实测官方词典
> （5.3MB / 350 700 词条）：单份加载 0.7s、堆常驻 +124MB；10 份各自加载 =
> 7.0s、1236MB。106 个站点名义上 ≈ 13GB —— 这就是启动期被吃满的一端。

- 唯一入口 `provider/segmenter.Shared()`（`sync.Once`，返回 `*sego.Segmenter`，
  **可能为 nil**，调用方必须处理）。`provider.WordSplit` 与悟空引擎都走它；
  `InitWebsites` 在遍历站点前先调一次，把加载耗时归到启动期。
- ⚠️ 不许出现第二处 `LoadDictionary`。护栏 `TestOnlyOneDictionaryLoaderInRepo`
  全仓扫描（跳过 `vendor/`、`node_modules/`）。
- ⚠️ 悟空引擎必须传 `EngineInitOptions.Segmenter`，不能再传 `SegmenterDictionaries`
  —— 后者让每个站点的引擎各读一遍词典。护栏 `TestWukongUsesSharedSegmenter`。
  传外部分词器还有个好处：`engine.Close()` 只关自己加载的那个
  （`usingExternalSegmenter == false` 才 Close），站点重载全文搜索不会关掉共享词典。
- ⚠️ **运行期谁都不能 Close 词典**：`sego.Close()` 把 `dict.trie` 置 nil，
  下一次分词就是 nil 解引用 —— 而且是"别的站点"的请求在 panic。
  历史上 `Website.DeleteCache`（后台「清缓存」）里有 `DictClose()`，多站点下
  等于给全站埋雷，已删。护栏 `TestNoOneClosesTheSharedDictionary`。
- 词典缺失 / 0 字节 / 格式坏时的口径是 **记 ERROR + 降级，不 os.Exit**：
  `sego.LoadDictionary` 对打不开的文件是 `log.Fatalf`（recover 拦不住），
  所以加载前必须 `Stat` 确认存在；悟空引擎拿不到词典时返回 error，
  由 `InitFulltext` 把状态置 -1 给用户看，而不是让进程起不来。
- 路径顺序：`config.ExecPath/dictionary.txt` → 工作目录/dictionary.txt。
  发布包把词典和主程序放一起；服务多由 systemd / 计划任务拉起，cwd 不固定。
- ⚠️ wukong 自身有个数据竞争（与本改造无关，别顺手改 vendor）：
  `Close() → FlushIndex()` 非原子读 `Init` 起 worker 正在原子写的计数器
  （engine.go:439 vs :170），`-race` 下「Init 后立刻 Close」必报。
  写测试时不要在 Init 之后马上 Close。
