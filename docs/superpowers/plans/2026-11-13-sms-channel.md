# 短信渠道（阿里云 / 腾讯云 / Twilio）实现计划

> **面向 AI 代理的工作者：** 必需子技能：使用 subagent-driven-development（推荐）或 executing-plans 逐任务实现此计划。步骤使用复选框（`- [ ]`）语法来跟踪进度。

**目标：** 为安企CMS 增加短信验证码能力，覆盖手机号登录（自动注册）、注册校验、短信找回密码、绑定／换绑四个前台场景，并在 `anqicms-admin` 提供配置界面与发送记录。

**架构：** 单厂商生效 + 厂商内分国内／国际两套签名模板。`provider/sms/` 是纯函数平台层（配置解析、号码归一化、签名器、三家适配器），不碰 DB；`provider/smsSend.go`／`smsVerify.go` 是业务层（OTP 状态机、限额、落库）；前台 `controller/apiSms.go`、后台 `controller/manageController/pluginSms.go`。新建 `sms_codes`／`sms_logs` 两张表，`users` 加 `phone_country` 列。

**技术栈：** Go 1.26.10 + iris v12.2.11 + GORM + MariaDB/MySQL；前端 UmiJS + antd/ProForm；MCP 意图内核 `pkg/mcp/intent`。**0 个新依赖**，签名手写，用官方 SDK 在隔离环境生成对照向量。

**规格：** `docs/superpowers/specs/2026-11-13-sms-channel-design.md`（commit `63286c62` 及后续订正）。**执行者必须两份都读**，规格的 §1 决策台账是绑定权威。

## 全局约束

每个任务的要求都隐含包含本节。数值从规格逐字照抄。

- **0 个新依赖。** `go.mod`／`go.sum` 全程不得改动。官方 SDK 只允许装在 `tmp/smsvec/` 的隔离模块里（`tmp/` 已被 `.gitignore:10` 忽略）。
- 验证码默认值：`CodeLength=6`（范围 4..8）、`CodeTTL=300`、`ResendInterval=60`、`MaxVerifyAttempts=5`、`DailyLimitPerPhone=10`、`HourlyLimitPerIp=10`、`DailyLimitPerIp=30`、`LogRetentionDays=90`。
- 区号默认 `86`；`86` → `domestic`，其余（含 `852`／`853`／`886`）→ `international`。
- 掩码哨兵 `********`（与 `SendmailPasswordMask`、`intent.MaskedSecret` 同串）。
- 响应码 `config.StatusOK`（0）／`config.StatusFailed`（-1）。仓库**没有** `StatusSuccess`。
- 平台成功判据：阿里云 `Code == "OK"`；腾讯云 `Response.SendStatusSet[0].Code == "Ok"`（大小写不同）；Twilio `status` 不为 `failed`／`undelivered` 且 `error_code` 为空。
- API 版本：阿里云 `2017-05-25`，腾讯云 `2021-01-11`。
- Endpoint：阿里云国内 `dysmsapi.aliyuncs.com`、国际默认 `dysmsapi.ap-southeast-1.aliyuncs.com`（可配）；腾讯云 `sms.tencentcloudapi.com`；Twilio `https://api.twilio.com/2010-04-01/Accounts/{AccountSid}/Messages.json`。
- 发送超时 **10s**，**不自动重试**（按条计费）。
- 场景码：`login`／`register`／`reset`／`bind_old`／`bind_new`，后台专用 `test`。通知预留 `notify.` 前缀，本期不定义具体值。
- **不改** `provider/user.go:963` 的 `VerifyCellphoneFormat`；**不复用** `library.CodeCache`。
- 后台端点用 `middleware.AdminPermissionRequired`（不是 `AdminPermission`）。
- `channel_sms` 意图 `DefaultOff: true`，四处平行名单同步（规格 §9.2）。
- 改 `route/` 后必跑 `go run ./cmd/apicatalog`。
- `go test ./provider/` 整包会被沙箱 SIGTERM（exit 137），**必须按 `-run` 分批**。`library`／`controller/manageController`／`pkg/mcp/intent` 可整跑。
- 提交**只按路径暂存**，禁止 `git add -A`／`git add .`。`docs/` 整体被忽略，规格与计划需 `git add -f`。工作区存在用户并行改动（`provider/sitemap.go`、`provider/sitemap_alternate_test.go`），**不得暂存**。
- 向量与测试一律用**假凭据**，不读取、不写入、不提交真实 AK/SK。
- 新增文件必须 `gofmt`。仓库存量 gofmt 不干净的文件不要顺手格式化。
- 每条护栏测试必须带**反向对照**；声明式清单用 go/ast 穷举，**不写死总数**。

## 审查重点（Review Focus）

规格隐含、但最可能伤到使用者的五类输入／失败模式。每条都已落到负责任务的测试步骤里。

1. **号码归一化不彻底** —— 全角数字 `１３９００００１１１１`、零宽空格、`+86`／`0086`／`86` 三种区号写法。正常人期待：同一号码归一化成同一个键，否则限额被绕过、平台侧重复计费。→ 任务 1 的 `TestNormalizePhone` 表驱动必须含这些用例。
2. **号码日限按场景各算** —— 攻击者同时打 `login`／`register`／`reset`，每场景各 10 条 = 30 条轰炸。正常人期待：号码日限**跨场景合计**。→ 任务 8 的 `TestSmsDailyLimitAcrossScenes`。
3. **并发校验绕过失败上限／同码双消费** —— `Attempts++` 与 `ConsumedAt` 写回不在同一事务。正常人期待：一个码只能被消费一次，失败次数不可重置。→ 任务 9 的 `TestSmsVerifyConcurrent`。
4. **配置提交 `templates: null` 静默清空** —— 顶层厂商对象传 null 是 no-op，但嵌套 map 传 null 会被设成 nil。正常人期待：模板不被静默清掉。→ 任务 12 的 `TestSmsSettingNullNestedMap`。
5. **平台返回 HTTP 200 但 body 不是 JSON** —— 网关插 HTML 错误页、CDN 拦截。正常人期待：判为失败并把原始 body 片段留进日志，绝不伪装成功。→ 任务 3／4／5 各自的 `Test*SenderNonJSONBody`。

## 文件结构

| 文件 | 职责 | 任务 |
|---|---|---|
| `config/sms.go` | 配置结构与常量，**纯结构无行为** | 1 |
| `provider/sms/sender.go` | `Profile`／`SendRequest`／`SendResult`／`Sender`／场景码 | 1 |
| `provider/sms/errors.go` | `ErrSms*` 哨兵、`SendError`、`ErrorClass`、通用分类 | 1 |
| `provider/sms/phone.go` | 号码归一化、E164、区域判定（纯函数） | 1 |
| `locales/{zh-CN,zh-TW,en-US}/default.yml` | 15 个错误键的三语文案 | 1 |
| `provider/sms/resolve.go` | `Resolve(cfg, region, scene)`、凭据继承、默认值 | 2 |
| `provider/sms/sign_aliyun.go` | ACS3-HMAC-SHA256（纯函数，时间与 nonce 可注入） | 3 |
| `provider/sms/aliyun.go` | 阿里云适配器 + 错误码映射 | 3 |
| `provider/sms/sign_tencent.go` | TC3-HMAC-SHA256 | 4 |
| `provider/sms/tencent.go` | 腾讯云适配器 + 错误码映射 | 4 |
| `provider/sms/twilio.go` | Twilio Basic 适配器 + 错误码映射 | 5 |
| `provider/sms/factory.go` | `NewSender(provider)` | 5 |
| `provider/sms/testdata/*.json` | 冻结的签名对照向量 | 3／4／5 |
| `model/sms.go` | `SmsCode`／`SmsLog` | 6 |
| `provider/sms_model_test.go` | 迁移注册、索引列序、敏感字段护栏 | 6 |
| `model/user.go` | 加 `PhoneCountry` | 7 |
| `provider/user.go` | 区号感知查询、`createUserWithPhoneGuard` | 7 |
| `provider/smsService.go` | 共享 helper（HMAC、日志回写） | 8 |
| `provider/smsSend.go` | 发码流程、限额、落库、惰性清理 | 8 |
| `provider/smsVerify.go` | 校验流程、行锁、一次性消费 | 9 |
| `provider/smsLogin.go` | `LoginViaSms`：查号／自动注册／审核门 | 10 |
| `request/sms.go` | 前台与后台请求结构 | 10／11／12 |
| `controller/apiSms.go` | 前台发码、短信找回、换绑 | 10／11 |
| `controller/apiUser.go` | 登录 `sms` 分支、注册手机号校验 | 10／11 |
| `controller/manageController/pluginSms.go` | 后台配置、测试发送、发送记录 | 12／13 |
| `route/base.go`、`route/manage.go` | 路由注册 | 10／11／12／13 |
| `anqicms-admin/src/services/plugin/sms.ts` | 前端 API 封装 | 14 |
| `anqicms-admin/src/pages/plugin/sms/index.tsx` | 六 tab 配置页 | 14 |
| `anqicms-admin/config/routes.ts`、`config/menu.go` | 路由与菜单权限 KEY | 14 |
| `anqicms-admin/src/locales/{zh-CN,en-US,zh-TW}/plugin.ts` | 三语文案 | 14 |
| `pkg/mcp/intent/catalog_domains.go`、`domain.go`、`mask.go` | `channel_sms` 意图与掩码名单 | 15 |
| `provider/api_intent_routes_test.go` | 门禁名单护栏 | 15 |

---

## 任务

### 任务 1：配置结构 + 平台契约 + 号码归一化

**文件：**

- 创建：`config/sms.go`、`provider/sms/sender.go`、`provider/sms/errors.go`、`provider/sms/phone.go`
- 测试：`provider/sms/phone_test.go`

**Interfaces（后续任务依赖）：** `config.PluginSmsConfig` 及其子结构、`sms.Profile`／`SendRequest`／`SendResult`／`Sender`、`sms.NormalizePhone`／`Phone`、15 个 `ErrSms*` 哨兵、`sms.SendError`／`ErrorClass`。

- [x] **步骤 1：编写失败的测试 `TestNormalizePhone`**

表驱动，字段 `{name, country, local, wantCountry, wantLocal, wantRegion, wantErr bool}`。**必须包含审查重点 1 的全部用例**：

```go
{"国内基本", "86", "13900001111", "86", "13900001111", "domestic", false},
{"加号区号", "+86", "139 0000 1111", "86", "13900001111", "domestic", false},
{"00 前缀区号", "0086", "139-0000-1111", "86", "13900001111", "domestic", false},
{"空区号默认 86", "", "13900001111", "86", "13900001111", "domestic", false},
{"全角数字", "86", "１３９００００１１１１", "86", "13900001111", "domestic", false},
{"零宽与不换行空格", "86", "139\u200b0000\u00a01111", "86", "13900001111", "domestic", false},
{"括号与点", "1", "(415) 555.0100", "1", "4155550100", "international", false},
{"意大利前导 0 保留", "39", "0612345678", "39", "0612345678", "international", false},
{"港澳台算国际", "852", "61234567", "852", "61234567", "international", false},
{"E164 恰好 15 位", "1", "12345678901234", "1", "12345678901234", "international", false},
{"国内少一位", "86", "1390000111", "", "", "", true},
{"国内非 1[3-9] 开头", "86", "23900001111", "", "", "", true},
{"区号为 0", "0", "1234567", "", "", "", true},
{"区号 5 位", "12345", "1234567", "", "", "", true},
{"本地号码 3 位", "1", "123", "", "", "", true},
{"E164 超 15 位", "1234", "123456789012", "", "", "", true},
{"本地号码为空", "86", "", "", "", "", true},
```

每条成功用例额外断言 `got.E164() == "+" + wantCountry + wantLocal`。
另写 `TestNormalizePhoneIdempotent`：对每条成功用例断言 `NormalizePhone(got.Country, got.Local)` 与首次结果逐字段相等（归一化必须幂等，否则查重键会漂移）。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/sms/ -run 'TestNormalizePhone' -v`
预期：编译失败，`undefined: NormalizePhone`。

- [x] **步骤 3：创建 `config/sms.go`**

只放结构与常量，**零方法、零 import provider**（`provider/sms` → `config` 单向，反向会成环）。
常量取值：`SmsProviderAliyun="aliyun"`、`SmsProviderTencent="tencent"`、`SmsProviderTwilio="twilio"`、`SmsRegionDomestic="domestic"`、`SmsRegionInternational="international"`。
类型：`PluginSmsConfig`、`SmsAliyunConfig{Domestic, International SmsAliyunRegion}`、`SmsAliyunRegion`、`SmsTencentConfig{Domestic, International SmsTencentRegion}`、`SmsTencentRegion`、`SmsTwilioConfig{International SmsTwilioRegion}`、`SmsTwilioRegion`。**字段名、json tag、注释逐字照抄规格 §3.2**，不增不减。

- [x] **步骤 4：创建 `provider/sms/sender.go` 与 `provider/sms/errors.go`**

`sender.go`：`Profile`／`SendRequest`／`SendResult`／`Sender` 逐字照抄规格 §5.1；场景码常量 `SceneLogin="login"`、`SceneRegister="register"`、`SceneReset="reset"`、`SceneBindOld="bind_old"`、`SceneBindNew="bind_new"`、`SceneTest="test"`。常量表上方写注释说明**通知场景统一用 `notify.` 前缀，本期不定义具体值**（规格 §2.4 的预留边界：只留 `Sender` 接口与命名空间，不建投递规则表／模板表／重试队列）。

`errors.go`：
```go
type ErrorClass string
const (
    ClassPermanent ErrorClass = "permanent"
    ClassTransient ErrorClass = "transient"
)
type SendError struct { Class ErrorClass; Code, Message string }
func (e *SendError) Error() string
func ClassifyHTTPStatus(status int) ErrorClass

// 带参数的两个错误：Error() 仍返回翻译键，数值由调用方 errors.As 取出
type WaitError struct{ Seconds int }      // Unwrap() → ErrSmsResendTooSoon
type AttemptsError struct{ Remaining int } // Unwrap() → ErrSmsCodeMismatch
```
15 个哨兵 `var ErrSmsXxx = errors.New("<翻译键>")`，**Error() 字符串就是翻译键本身**（`provider/sms` 拿不到 `*Website`，翻译在 controller 层做）。键名与规格 §10.2 表格逐行对应：`SmsServiceNotEnabled`、`SmsSceneNotEnabled`、`SmsPhoneInvalid`、`SmsRegionUnsupported`、`SmsCredentialMissing`、`SmsSignMissing`、`SmsTemplateMissing`、`SmsIpLimited`、`SmsPhoneLimited`、`SmsResendTooSoon`、`SmsCodeNotFound`、`SmsCodeExpired`、`SmsCodeMismatch`、`SmsCodeLocked`、`SmsSendFailed`。
`ClassifyHTTPStatus`：429／408／425 与 ≥500 → transient；其余 4xx → permanent；2xx／3xx → transient（未知一律 transient）。

- [x] **步骤 5：实现 `provider/sms/phone.go`**

```go
type Phone struct{ Country, Local string }
func NormalizePhone(country, local string) (Phone, error)
func (p Phone) E164() string  // "+" + Country + Local
func (p Phone) Region() string // Country == "86" → domestic，否则 international
```
规则按规格 §6.2 五条，正则逐字：区号 `^[1-9]\d{0,3}$`、本地 `^\d{4,14}$`、国内附加 `^1[3-9]\d{9}$`。
去格式字符集合必须覆盖：半角空格、`\t`、`-`、`(`、`)`、`.`、不换行空格 `\u00a0`、零宽空格 `\u200b`、零宽连接符 `\u200d`、BOM `\ufeff`，以及**全角数字 `０-９`（U+FF10..U+FF19）折半角**。顺序：先折全角 → 再去格式字符 → 再校验。所有失败路径返回 `ErrSmsPhoneInvalid`。

- [x] **步骤 6：补后端多语言文案**

15 个翻译键加进 `locales/zh-CN/default.yml`、`locales/zh-TW/default.yml`、`locales/en-US/default.yml`，文案口径按规格 §10.2 表格第三列。
**只加这三个** —— 实测另外 6 个 locale（bn-BD／fa-IR／id-ID／ja-JP／pt-BR／ru-RU）各只有 95 个键，是部分翻译，缺键时 `w.tr` 回落到 `fmt.Sprintf(str, args...)` 返回键名本身，与仓库其余 770 多个键的现状一致。
两个带参数的键用 `%d` 占位：`SmsResendTooSoon` → `"请 %d 秒后再试"`，`SmsCodeMismatch` → `"验证码错误，还可尝试 %d 次"`（`Tr(str, args...)` 支持可变参数）。
校验：三个文件的键集合差集必须为空（用脚本比对 `^"Sms` 前缀的键名）。

- [x] **步骤 7：运行测试验证通过**

运行：`go test ./provider/sms/ -run 'TestNormalizePhone' -v && go build ./... && go vet ./provider/sms/ ./config/`
预期：两个测试 PASS，build 与 vet 无输出。

- [x] **步骤 8：Commit**

```bash
gofmt -w config/sms.go provider/sms/sender.go provider/sms/errors.go provider/sms/phone.go provider/sms/phone_test.go
git add config/sms.go provider/sms/sender.go provider/sms/errors.go provider/sms/phone.go provider/sms/phone_test.go locales/zh-CN/default.yml locales/zh-TW/default.yml locales/en-US/default.yml
git commit -m "feat(短信): 新增短信配置结构、平台契约与号码归一化"
```

---

### 任务 2：区域解析与凭据继承

**文件：**

- 创建：`provider/sms/resolve.go`
- 测试：`provider/sms/resolve_test.go`

**Interfaces：** `sms.Resolve(cfg *config.PluginSmsConfig, region, scene string) (*Profile, error)`、`sms.ApplyDefaults(cfg *config.PluginSmsConfig)`。

- [x] **步骤 1：编写失败的测试**

四个测试，断言用规格 §3.3 的口径：

`TestResolveCredentialInheritance`：只填国内凭据、国际凭据留空 → 国际 profile 的凭据字段等于国内值；**`SignName`／`TemplateId` 必须仍取国际槽自己的值，绝不为国内值**（签名与模板永不沿用）。国际槽填了自己的凭据 → 不继承。**反向不继承**：只填国际凭据、国内留空 → 国内 `Resolve` 报 `ErrSmsCredentialMissing`，**不得静默借用国际凭据**。

`TestResolveRegionUnsupported`：`Provider=twilio` + `region=domestic` → `errors.Is(err, ErrSmsRegionUnsupported)`，且 `profile == nil`（明确拒发，不静默降级）。

`TestResolveMissingPieces`：表驱动三条 —— 缺凭据 → `ErrSmsCredentialMissing`；国内缺 `SignName` → `ErrSmsSignMissing`；`Templates[scene]` 为空或 scene 不在 map 里 → `ErrSmsTemplateMissing`。**第三条对三家都跑一遍**（Twilio 查 `Texts[scene]`）。

`TestApplyDefaults`：零值 `PluginSmsConfig` 过一遍 `ApplyDefaults` 后八个数值等于全局约束里的默认值；`CodeLength` 传 3 → 收敛到 4，传 9 → 收敛到 8；已显式设置的合法值不被改写。阿里云 `ParamName` 为空 → `"code"`；阿里云国际 `Endpoint` 为空 → `dysmsapi.ap-southeast-1.aliyuncs.com`；国内 `Endpoint` 为空 → `dysmsapi.aliyuncs.com`；腾讯云 `Region` 为空 → `ap-guangzhou`。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/sms/ -run 'TestResolve|TestApplyDefaults' -v`
预期：编译失败，`undefined: Resolve`。

- [x] **步骤 3：实现 `provider/sms/resolve.go`**

两个导出函数签名见 Interfaces。`Resolve` 内部顺序：`ApplyDefaults` 的副本（**不得改写入参 cfg**，它是站点内存里的全局对象）→ 按 provider 选槽 → Twilio+domestic 直接拒 → 凭据继承（只继承凭据字段）→ 必填校验 → 填 `Profile`。
腾讯云 `TemplateParamSet` 是位置数组，`Profile.ParamName` 对腾讯云无意义，留空。

- [x] **步骤 4：运行测试验证通过**

运行：`go test ./provider/sms/ -v`
预期：任务 1 与本任务的测试全部 PASS。

- [x] **步骤 5：反向对照 —— 证明继承护栏抓得住错**

临时把「只继承凭据」改成「整槽拷贝」，重跑 `TestResolveCredentialInheritance`，**必须变红**；恢复后重跑必须变绿。把这次对照的结论写进测试文件顶部注释（一行，说明改坏了会红）。

- [x] **步骤 6：Commit**

```bash
gofmt -w provider/sms/resolve.go provider/sms/resolve_test.go
git add provider/sms/resolve.go provider/sms/resolve_test.go
git commit -m "feat(短信): 区域解析与国内→国际凭据继承，签名模板不沿用"
```

---

### 任务 3：阿里云签名器 + 适配器 + 对照向量

**文件：**

- 创建：`provider/sms/sign_aliyun.go`、`provider/sms/aliyun.go`
- 创建：`provider/sms/testdata/aliyun_sign.json`（冻结向量）
- 测试：`provider/sms/aliyun_test.go`、`provider/sms/sign_aliyun_test.go`
- 一次性（不入库）：`tmp/smsvec/`（隔离 Go module，装官方 SDK 生成向量）

**Interfaces：** `sms.NewAliyunSender() Sender`；签名器 `sms.SignAliyun(req *http.Request, p Profile, now time.Time, nonce string) error`（时间与 nonce 可注入，便于对照）。

- [x] **步骤 1：在隔离模块里生成对照向量（规格 §11.1 九步）**

在 `tmp/smsvec/` 建独立 `go.mod`（**仓库 `go.mod`／`go.sum` 全程不动**），装阿里云官方 SDK。起 `httptest` 假服务器，用假 AK/SK 让 SDK 把 `SendSms` 打到假服务器上，抓下完整请求。
**不强行注入时间／nonce** —— 从抓到的请求头里读出 SDK 实际用的 `x-acs-date` 与 `x-acs-signature-nonce`，连同 method／path／query／全部 header／body 一起写成 `provider/sms/testdata/aliyun_sign.json`。向量里凭据一律是假值。

- [x] **步骤 2：编写失败的测试**

`TestAliyunSignMatchesOfficialVector`：读 `testdata/aliyun_sign.json`，用其中的时间与 nonce 调 `SignAliyun`，**逐字节比对** `Authorization` 与 `x-acs-content-sha256`。
反向对照子测试：① 把向量里 `Signature` 改一个字节 → 必须不等；② 把 canonical query 里的 `%20` 换成 `+` → 必须不等（阿里云 V3 用 `%20`，写成 `+` 是经典错法）。

`TestAliyunSenderSuccess`：httptest 返回 `{"Code":"OK","Message":"OK","BizId":"1234567890","RequestId":"R-1"}` → `err == nil`，`result.BizId == "1234567890"`，`result.RawCode == "OK"`。同时断言请求体里 `TemplateParam == "{\"code\":\"123456\"}"`（`ParamName` 默认 `code`）、`OutId` 等于传入值、`PhoneNumbers` 是**不带 `+`** 的号码。

`TestAliyunSenderBusinessFailure`：HTTP 200 + `{"Code":"isv.BUSINESS_LIMIT_CONTROL","Message":"..."}` → `err != nil`，`errors.As` 取到 `*SendError` 且 `Code == "isv.BUSINESS_LIMIT_CONTROL"`。**HTTP 200 不等于成功。**

`TestAliyunSenderNonJSONBody`（审查重点 5）：HTTP 200 + body `<html>502 Bad Gateway</html>` → `err != nil`，`SendError.Message` 含原始 body 片段（截断到 200 字节），`Class == ClassTransient`，**绝不返回 nil error**。

`TestAliyunSenderErrorCodeClassification`：表驱动覆盖规格 §5.6 阿里云行的**每一类** permanent 错误（签名不存在／未审核、模板不存在／未审核、AK 无效或无权限、账户余额不足、号码格式非法、平台级流控），逐条断言 `Class == ClassPermanent`；再加一条未列入表的码 → `ClassTransient` 且 **`SendError.Code` 原样保留该码**（不得被吞成「发送失败」，那是排查唯一线索）。**不写死错误码总数。**

`TestAliyunSenderBothEndpoints`：`Profile.Endpoint` 分别取 `dysmsapi.aliyuncs.com` 与 `dysmsapi.ap-southeast-1.aliyuncs.com`，断言请求确实打到对应 host，且两次签名都通过各自的向量（向量文件里存两组）。

- [x] **步骤 3：运行测试验证失败**

运行：`go test ./provider/sms/ -run 'TestAliyun' -v`
预期：编译失败，`undefined: SignAliyun`。

- [x] **步骤 4：实现签名器与适配器**

`sign_aliyun.go`：ACS3-HMAC-SHA256。请求头 `x-acs-action: SendSms`、`x-acs-version: 2017-05-25`、`x-acs-date`（UTC ISO8601 `2006-01-02T15:04:05Z`）、`x-acs-signature-nonce`、`x-acs-content-sha256`；`Authorization: ACS3-HMAC-SHA256 Credential={AccessKeyId},SignedHeaders={...},Signature={...}`。canonical query 用 `%20` 不用 `+`。手写，**不引入依赖**；写法参考仓库既有的 `provider/storage/aws.go:89-171`。
`aliyun.go`：`NewAliyunSender()`；`Send` 走 `p.Endpoint`（国内／国际由 `Resolve` 决定），超时 **10s，不重试**。成功判据 `Code == "OK"`。错误码映射按规格 §5.6，未列入的一律 `ClassTransient`。

- [x] **步骤 5：运行测试验证通过**

运行：`go test ./provider/sms/ -run 'TestAliyun' -v`
预期：全部 PASS，且**不联网**（向量已冻结，测试只读 testdata）。

- [x] **步骤 6：清理隔离模块并 Commit**

```bash
rm -rf tmp/smsvec
gofmt -w provider/sms/sign_aliyun.go provider/sms/aliyun.go provider/sms/aliyun_test.go provider/sms/sign_aliyun_test.go
git add provider/sms/sign_aliyun.go provider/sms/aliyun.go provider/sms/aliyun_test.go provider/sms/sign_aliyun_test.go provider/sms/testdata/aliyun_sign.json
git commit -m "feat(短信): 阿里云 ACS3-HMAC-SHA256 签名与适配器，附官方 SDK 对照向量"
```

---

### 任务 4：腾讯云签名器 + 适配器 + 对照向量

**文件：**

- 创建：`provider/sms/sign_tencent.go`、`provider/sms/tencent.go`、`provider/sms/testdata/tencent_sign.json`
- 测试：`provider/sms/tencent_test.go`、`provider/sms/sign_tencent_test.go`
- 一次性（不入库）：`tmp/smsvec/`

**Interfaces：** `sms.NewTencentSender() Sender`；`sms.SignTencent(req *http.Request, p Profile, payload []byte, now time.Time) error`。

- [x] **步骤 1：生成对照向量**

同任务 3 步骤 1 的九步，隔离模块装腾讯云官方 SDK，action `SendSms`、version `2021-01-11`、endpoint `sms.tencentcloudapi.com`。从抓到的请求读出 SDK 实际用的 `X-TC-Timestamp`，冻结成 `testdata/tencent_sign.json`。凭据全假。

- [x] **步骤 2：编写失败的测试**

`TestTencentSignMatchesOfficialVector`：逐字节比对 `Authorization` 与 `X-TC-Timestamp`。反向对照：① 改 `Signature` 一字节必不等；② 把派生密钥的日期串从 `X-TC-Timestamp` 的 UTC 日期换成本地日期 → 必不等（TC3 的 credential scope 日期必须按 UTC 算，跨时区是经典错法）。

`TestTencentSenderSuccess`：httptest 返回 `{"Response":{"SendStatusSet":[{"SerialNo":"S-1","PhoneNumber":"+8613900001111","IsoCode":"CN","Code":"Ok","Message":"send success","Fee":1,"SessionContext":"42"}],"RequestId":"R-1"}}` → `err == nil`，`BizId == "S-1"`，`RawCode == "Ok"`。断言请求体：`PhoneNumberSet` 是**带 `+` 的 E.164 数组**、`SmsSdkAppId`／`TemplateId`／`SignName` 就位、`TemplateParamSet == ["123456"]`（**位置数组，不是对象**）、`SessionContext == req.OutId`。

`TestTencentSenderBusinessFailure`：HTTP 200 + `SendStatusSet[0].Code == "LimitExceeded.PhoneNumberDailyLimit"` → `err != nil` 且 `SendError.Code` 等于该串。**成功判据是 `"Ok"` 不是 `"OK"`** —— 单独写一条 `Code == "OK"`（大写 K）的用例，断言**判为失败**，钉住大小写差异。

`TestTencentSenderNonJSONBody`（审查重点 5）：HTTP 200 + `<html>...</html>` → `err != nil`，`Message` 含 body 片段（≤200 字节），`Class == ClassTransient`。

`TestTencentSenderEmptyStatusSet`：HTTP 200 + `{"Response":{"SendStatusSet":[],"RequestId":"R"}}` → `err != nil`（空数组不能当成功）。

`TestTencentSenderErrorCodeClassification`：表驱动覆盖规格 §5.6 腾讯云行的每一类 permanent 错误（签名未审核、模板未审核、SecretId 无效、SmsSdkAppId 不存在、余额不足、号码格式非法）；未列入的码 → `ClassTransient` 且 `Code` 原样保留。

`TestTencentSenderSignNameVariants`：国内 `SignName` 非空 → 请求体带该值；国际 `SignName` 为空 → 请求体**不带** `SignName` 键（腾讯云国际选填）。两种各存一组向量。

- [x] **步骤 3：运行测试验证失败**

运行：`go test ./provider/sms/ -run 'TestTencent' -v`
预期：编译失败，`undefined: SignTencent`。

- [x] **步骤 4：实现签名器与适配器**

`sign_tencent.go`：TC3-HMAC-SHA256，header `X-TC-Action`／`X-TC-Version`／`X-TC-Timestamp`／`X-TC-Region`（取 `p.CloudRegion`）；credential scope `{UTC日期}/sms/tc3_request`。手写，0 依赖。
`tencent.go`：`NewTencentSender()`；超时 10s 不重试；成功判据 `Response.SendStatusSet[0].Code == "Ok"`，空数组视为失败。`SignName` 国内必填（是签名内容不是 ID）。错误码映射按规格 §5.6。

- [x] **步骤 5：运行测试验证通过**

运行：`go test ./provider/sms/ -run 'TestTencent' -v`
预期：全部 PASS，不联网。

- [x] **步骤 6：清理并 Commit**

```bash
rm -rf tmp/smsvec
gofmt -w provider/sms/sign_tencent.go provider/sms/tencent.go provider/sms/tencent_test.go provider/sms/sign_tencent_test.go
git add provider/sms/sign_tencent.go provider/sms/tencent.go provider/sms/tencent_test.go provider/sms/sign_tencent_test.go provider/sms/testdata/tencent_sign.json
git commit -m "feat(短信): 腾讯云 TC3-HMAC-SHA256 签名与适配器，附官方 SDK 对照向量"
```

---

### 任务 5：Twilio 适配器 + Sender 工厂

**文件：**

- 创建：`provider/sms/twilio.go`、`provider/sms/factory.go`
- 测试：`provider/sms/twilio_test.go`、`provider/sms/factory_test.go`

**Interfaces：** `sms.NewTwilioSender() Sender`；`sms.NewSender(provider string) (Sender, error)`。

- [x] **步骤 1：编写失败的测试**

Twilio 无签名算法（HTTP Basic），**不需要 SDK 对照向量**；用 httptest 直接钉契约。

`TestTwilioSenderAuth`：表驱动两条 —— 只填 `AuthToken` → `Authorization` 是 `Basic base64(AccountSid:AuthToken)`；同时填 `ApiKeySid`／`ApiKeySecret` → 用 `base64(ApiKeySid:ApiKeySecret)`（**API Key 优先**）。两条都断言 URL 路径里的 AccountSid 仍是 `AccountSid`（它恒必填，不随凭据对切换）。

`TestTwilioSenderFrom`：只填 `From` → 表单带 `From`、不带 `MessagingServiceSid`；两个都填 → **只带 `MessagingServiceSid`**；两个都空 → `err != nil`（`ErrSmsCredentialMissing`）。断言 `Content-Type: application/x-www-form-urlencoded`、`To` 带 `+`、`Body` 等于 `req.Text`（已渲染文案）。

`TestTwilioSenderSuccess`：返回 `{"sid":"SM1","status":"queued","error_code":null,"error_message":null}` → `err == nil`，`BizId == "SM1"`。

`TestTwilioSenderFailureStatus`：表驱动 `status` 为 `failed`／`undelivered` → 均 `err != nil`；`error_code` 非空（如 `21211`）即使 `status == "queued"` 也 `err != nil`。

`TestTwilioSenderNonJSONBody`（审查重点 5）：HTTP 200 + HTML → `err != nil`，`Message` 含 body 片段（≤200 字节），`Class == ClassTransient`。

`TestNewSender`：三个 provider 字符串各返回对应 `Name()`；`""` 与 `"unknown"` 返回 error，**不返回 nil sender**（fail closed）。

`TestTwilioSenderErrorCodeClassification`：表驱动覆盖规格 §5.6 Twilio 行的每一个码（`20003` 鉴权失败、`21211` To 非法、`21606` From 非法、`21610` 已退订、余额不足）→ `ClassPermanent`；未列入的码 → `ClassTransient` 且 `Code` 原样保留。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/sms/ -run 'TestTwilio|TestNewSender' -v`
预期：编译失败，`undefined: NewTwilioSender`。

- [x] **步骤 3：实现 `twilio.go` 与 `factory.go`**

`twilio.go`：`POST https://api.twilio.com/2010-04-01/Accounts/{AccountSid}/Messages.json`，Basic Auth，表单编码。超时 10s 不重试。**不做 Content SID 映射**（规格 §3.2 已决定 YAGNI），`Body` 直接用 `req.Text`。
`factory.go`：`NewSender` 按 `config.SmsProvider*` 常量分派，未知值返回 error。

- [x] **步骤 4：运行测试验证通过**

运行：`go test ./provider/sms/ -v && go build ./... && go vet ./provider/sms/`
预期：`provider/sms` 整包 PASS（该包无 DB 依赖，可整跑）。

- [x] **步骤 5：Commit**

```bash
gofmt -w provider/sms/twilio.go provider/sms/factory.go provider/sms/twilio_test.go provider/sms/factory_test.go
git add provider/sms/twilio.go provider/sms/factory.go provider/sms/twilio_test.go provider/sms/factory_test.go
git commit -m "feat(短信): Twilio Basic 适配器与 Sender 工厂"
```

---

### 任务 6：数据模型 + 迁移注册 + 结构护栏

**文件：**

- 创建：`model/sms.go`、`provider/sms_model_test.go`
- 修改：`provider/dao.go:134-242`（models 切片末尾追加两项）

**Interfaces：** `model.SmsCode`、`model.SmsLog`（字段见规格 §4.2／§4.3）。

- [x] **步骤 1：编写失败的护栏测试**

复用 `provider/channel_model_test.go` 已有的 `scanMigratedModels`／`parseGormIndexes`／`gormIndexPriority`／`modelTypeName`（**不重写扫描器**）。新增 `smsModels = []string{"SmsCode", "SmsLog"}`。

`TestSmsTablesRegistered`：`scanMigratedModels` 的结果必须包含 `smsModels` 全部元素。**不写死模型总数。** 反向对照：用合成源码（去掉这两项的 models 切片）喂扫描器，必须报缺失。

`TestSmsLogIndexOrdersSiteFirst`：用 `parseGormIndexes` + `gormIndexPriority` 断言 `SmsLog` 的 `idx_site_created` 列序是 `site_id`(1) → `created_time`(2)；`SmsCode` 的 `idx_site_phone_scene` 列序是 `site_id`(1) → `phone_country`(2) → `phone`(3) → `scene`(4)。反向对照：把合成源码里两个 priority 对调，必须变红。

`TestSmsCodeHashNotSerialized`：go/ast 断言 `SmsCode.CodeHash` 的 tag 含 `json:"-"`。反向对照：合成一份 tag 为 `json:"code_hash"` 的源码，必须被判违规。**正向对照**：同一个检查器对 `SmsCode.Phone`（正常 `json:"phone"`）必须判为合规，证明检查不恒假。

`TestSmsLogStoresNoPlainText`：go/ast 断言 `SmsLog` **不存在**名为 `Code`／`CodeHash`／`Body`／`Text`／`Content` 的字段（日志表不得存验证码明文或渲染正文）。反向对照：合成一份加了 `Body string` 的源码，必须变红。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestSmsTablesRegistered|TestSmsLogIndexOrdersSiteFirst|TestSmsCodeHashNotSerialized|TestSmsLogStoresNoPlainText' -v`
预期：FAIL（`model.SmsCode` 未定义／未注册）。**按 `-run` 分批，不整跑 provider。**

- [x] **步骤 3：创建 `model/sms.go`**

`SmsCode` 与 `SmsLog` 的字段名、列名、类型、默认值、索引 tag **逐字照抄规格 §4.2／§4.3**。两处不可省：`CodeHash` 的 `json:"-"`；`idx_site_created` 的 `site_id` priority 1。
**不写 `TableName()`** —— 仓库 90 余个模型里只有 2 个写了它，约定是靠 GORM 复数化：`SmsCode` → `sms_codes`、`SmsLog` → `sms_logs`，正好是需要的表名。测试里用 `w.DB.Migrator().HasTable("sms_codes")` 实测确认，不靠推断。

- [x] **步骤 4：注册迁移**

在 `provider/dao.go` 的 `var models = []interface{}{...}` 末尾（`&model.NotificationDelivery{}` 之后）追加 `&model.SmsCode{}`、`&model.SmsLog{}`，并加一行分组注释说明用途。

- [x] **步骤 5：运行测试验证通过**

运行：`go test ./provider/ -run 'TestSms' -v && go build ./...`
预期：四条护栏 PASS。

- [x] **步骤 6：Commit**

```bash
gofmt -w model/sms.go provider/sms_model_test.go
git add model/sms.go provider/sms_model_test.go provider/dao.go
git commit -m "feat(短信): 新增 sms_codes/sms_logs 模型与迁移注册，附结构护栏"
```

---

### 任务 7：users 区号列 + 事务内查重

**文件：**

- 修改：`model/user.go`（加 `PhoneCountry`）、`provider/user.go:95`／`:576`／`:616`／`:1048`、`provider/order.go:1347`、`request/user.go`（资料更新请求）
- 测试：`provider/sms_user_test.go`

**Interfaces：** `model.User.PhoneCountry`；`func (w *Website) GetUserInfoByPhoneCountry(country, phone string) (*model.User, error)`；`func (w *Website) createUserWithPhoneGuard(user *model.User) error`。

- [x] **步骤 1：编写失败的测试**

`TestUserPhoneCountryColumn`：go/ast 断言 `model.User` 有 `PhoneCountry string`，tag 含 `type:varchar(8)`、`not null`、`default:'86'`；断言 `Phone` 字段**没有** `uniqueIndex`（实测 339 行里 273 个互不重复非空号 + 66 个空串，加唯一索引会迁移失败）。反向对照：合成一份把 `Phone` 标成 `uniqueIndex` 的源码，必须变红。

`TestRegisterUserUsesPhoneGuard`：go/ast 断言 `RegisterUser` 函数体内**出现** `createUserWithPhoneGuard` 调用、**不出现** `w.DB.Save(&user)`。反向对照：合成一份仍用裸 `w.DB.Save` 的源码，必须变红。

`TestCreateUserWithPhoneGuard`（真库，三条子用例，每条自带清理）：
① 同 `(country, phone)` 插两次 → 第二次返回 error，`users` 表该号仍只有 1 行；
② 同 `phone` 不同 `country`（`86` vs `852`）→ 两次都成功，2 行；
③ `phone == ""` → 连续插 3 个用户全部成功（空号不参与查重）。

`TestCreateUserWithPhoneGuardConcurrent`：10 个 goroutine 同时用同一 `(country, phone)` 调 `createUserWithPhoneGuard` → **恰好 1 个成功**，表里 1 行。这条是应用层查重的存在理由，跑在实测为 REPEATABLE-READ 的开发库上。

`TestVerifyCellphoneFormatUnchanged`：从 `provider/user.go` **真实文件文本**里取 `VerifyCellphoneFormat` 的函数体，断言正则字面量仍是 `1[3-9][0-9]{9}`（**无 `^$` 锚点**）。锚点字符串必须从文件读出后比对，不凭印象写死。反向对照：合成一份加了 `^...$` 的源码必须变红。理由：收紧会让存量带前缀账号（`"abc13900001111xyz"` 这类）无法登录，新校验只用于短信路径。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestUserPhoneCountryColumn|TestRegisterUserUsesPhoneGuard|TestCreateUserWithPhoneGuard|TestVerifyCellphoneFormatUnchanged' -v`
预期：FAIL（字段／函数未定义）。

- [x] **步骤 3：加列与改查询**

`model/user.go` 加 `PhoneCountry string`，gorm tag `column:phone_country;type:varchar(8);not null;default:'86'`，位置紧邻 `Phone`。**存量行由 MySQL 的 `ADD COLUMN ... NOT NULL DEFAULT '86'` 自动回填 86**，不需要单独的数据迁移脚本。
`GetUserInfoByPhone(phone)` 改名为 `GetUserInfoByPhoneCountry(country, phone)`，WHERE 用 `phone_country = ? AND phone = ?`；`country == ""` 时按 `86` 处理。三处调用点按规格 §6.6 改：`:576` 由步骤 4 的事务查重取代、`:1048` 传 `req.PhoneCountry`（未传沿用用户现值）、`provider/order.go:1347` 传 `user.PhoneCountry`。
资料更新请求的 `PhoneCountry` 用 **`*string`**（与既有 `Phone *string` 一致，未传不改，否则编辑资料会把区号静默重置成 86）。

- [x] **步骤 4：实现 `createUserWithPhoneGuard` 并接进 `RegisterUser`**

函数体照抄规格 §4.4 的代码块（三条约束不可改：WHERE 只用 `phone` 单列、`phone == ""` 跳过、依赖 RR）。`RegisterUser` 里 `w.DB.Save(&user)`（`:616`）换成 `w.createUserWithPhoneGuard(&user)`。**extra 字段、`ProcessNewUser`、发验证邮件全部留在事务外** —— 不能把 SMTP 圈进持锁事务。
在函数上方写注释记录「依赖 REPEATABLE READ；若生产库是 READ COMMITTED 则保证不成立，上线前必须核对」。

- [x] **步骤 5：运行测试验证通过**

运行：`go test ./provider/ -run 'TestUserPhoneCountryColumn|TestRegisterUserUsesPhoneGuard|TestCreateUserWithPhoneGuard|TestVerifyCellphoneFormatUnchanged' -v && go build ./... && go vet ./provider/ ./model/ ./request/`
预期：全部 PASS。

- [x] **步骤 6：Commit**

```bash
gofmt -w provider/sms_user_test.go
git add model/user.go provider/user.go provider/order.go request/user.go provider/sms_user_test.go
git commit -m "feat(短信): users 增加 phone_country 列，手机号查重收进事务"
```

---

### 任务 8：OTP 发放（限额 + 事务落库 + 平台调用 + 回写）

**这是最大的一个任务，分两轮 TDD 子循环执行。**

**文件：**

- 创建：`provider/smsService.go`（共享 helper）、`provider/smsSend.go`（发放流程）
- 测试：`provider/smsSend_test.go`

**Interfaces（任务 9／10／11／13 依赖）：**
```go
type SmsSendInput struct {
    PhoneCountry, Phone string
    Scene               string
    Ip                  string
    UserId              uint
}
type SmsSendResult struct {
    ExpiresIn   int `json:"expires_in"`
    ResendAfter int `json:"resend_after"`
}
func (w *Website) SendSmsCode(in SmsSendInput) (*SmsSendResult, error)
func (w *Website) sendSmsCodeWithSender(in SmsSendInput, sender sms.Sender) (*SmsSendResult, error)
// provider/smsService.go
func (w *Website) smsCodeHash(code string) string // HMAC-SHA256(TokenSecret, code)，hex
func smsIpBudgetFor(siteId uint, hourLimit, dayLimit int) *smsIpBudget
```

**两条放置约束（不可改）：**

- **图形验证码校验放 controller 层，不在 provider。** `Store` 在 `controller` 包，`provider` 反向 import 会成环。规格 §6.3 第 4 步由任务 10 的 handler 承担。
- **`sender.Send` 必须在 `w.DB.Transaction` 回调之外。** 持锁事务里做网络 IO 会把行锁按住数秒，平台一慢就雪崩。`sendSmsCodeWithSender` 是测试接缝：生产入口 `SendSmsCode` 用 `sms.NewSender` 构造后转调它，测试传假 sender，**全程不触网、不发真实短信**。

- [x] **步骤 1：写测试基建 + 第一轮失败测试（限额与状态机）**

基建（`provider/smsSend_test.go` 顶部）：
```go
func setupSmsTest(t *testing.T) *Website // initTestSite(t) + 幂等 AutoMigrate(&model.SmsCode{}, &model.SmsLog{})
func smsTestPhone(t *testing.T, w *Website) (country, local string)
```
`smsTestPhone` 用保留测试号段（国际 `("1","5550100001")`，国内 `("86","19900000001")`）。**前置断言该号未被真实用户占用，占用则 `t.Skip` 而不是删数据**；`t.Cleanup` 只删本用例号段且 `created_time >= 用例开始时刻` 的 `sms_codes`／`sms_logs`／`users` 行。库里是站点 1 的真实数据，**不 DROP 任何表**。

假 sender：`type fakeSmsSender struct{ res sms.SendResult; err error; calls int; beforeSend func() }`，实现 `Name()`／`Send()`。

第一轮测试：

- `TestSmsSendDisabled`：`Provider=""` → `errors.Is(err, sms.ErrSmsServiceNotEnabled)`，且 `sms_logs` 无新行。
- `TestSmsSendSceneWhitelist`：`Scenes=["login"]` → `register` 落 `ErrSmsSceneNotEnabled`；`test` **允许**（后台场景不受白名单限制）；`notify.order` 落 `ErrSmsSceneNotEnabled`。
- `TestSmsSendDailyLimitAcrossScenes`（**审查重点 2**）：`DailyLimitPerPhone=3`，依次发 `login`／`register`／`reset` 各一次全部成功，第四次（任意场景）落 `ErrSmsPhoneLimited`，`sms_logs` 该号当日行数 == 3。**反向对照**：把计数查询加上 `scene` 条件后重跑必须变红。
- `TestSmsSendResendIntervalPerScene`：`ResendInterval=60`，`login` 连发两次 → 第二次 `ErrSmsResendTooSoon` 且 `errors.As` 取到 `*sms.WaitError`、`Seconds` 在 1..60；紧接着发 `register` → **成功**（重发间隔按场景）。
- `TestSmsSendRegionUnsupported`：`Provider=twilio` + `("86", ...)` → `ErrSmsRegionUnsupported`，假 sender `calls == 0`。
- `TestSmsSendPrunesStaleCodes`：先手工插入同键的已过期行与已消费行 → 发码后这两行被删、新行存在。
- `TestSmsIpBudgetIndependent`：把 `HourlyLimitPerIp=2`，同一 IP 连发三次（换号码绕开号码限额）→ 第三次 `ErrSmsIpLimited`；日限同理跑一遍。**独立性用源码级断言**：`provider/smsSend.go`／`smsService.go` **不 import `middleware` 包**、不出现 `accountProbeLimiter`（`provider` → `middleware` 本来就会成环，这条钉住「不许为了共用预算而绕道」）。共用会让短信请求把邮箱枚举探测的预算吃掉，两个功能互相污染。
- `TestSmsIpLimitRejectedDoesNotExtendWindow`：被拒的请求**不延长窗口** —— 超限后再调一次仍被拒，且窗口到期时刻不因这次调用而后移。
- `TestSmsDoesNotReuseCodeCache`：go/ast 断言 `provider/smsService.go`／`smsSend.go`／`smsVerify.go` 三个文件里**不出现** `library.CodeCache`。反向对照：合成一份含它的源码必须变红。理由：`CodeCache` 有七项已知缺陷（Get 不检查过期、Generate 无锁读写 map、key 不存在且 code 为空时 Verify 返回 true、进程内存储、math/rand 可预测等），复用它等于把这些缺陷带进短信路径。

- [x] **步骤 2：运行验证失败**

运行：`go test ./provider/ -run 'TestSmsSend|TestSmsIp|TestSmsDoesNotReuseCodeCache' -v`
预期：编译失败，`undefined: SendSmsCode`。

- [x] **步骤 3：实现第一轮**

`provider/smsService.go`：`smsCodeHash` 用 `crypto/hmac` + `crypto/sha256`，密钥取站点 `TokenSecret`；`smsIpBudgetFor` 用 `sync.Map`，key 为 `fmt.Sprintf("%d:%d:%d", siteId, hourLimit, dayLimit)`，值是 `&smsIpBudget{hour: library.NewRateLimiter(hourLimit, time.Hour), day: library.NewRateLimiter(dayLimit, 24*time.Hour)}`。**必须是独立单例，不复用 `middleware.accountProbeLimiter`**（共用会让两个功能的限额互相污染）。注释写明「限额进 key，改配置会重置窗口」。
`provider/smsSend.go`：按规格 §6.3 的第 1／2／3／5／6／7 步实现。第 7 步事务内顺序 a→f 不可换：日限计数查 `sms_logs`（**不带 scene**）、重发间隔查 `sms_codes`（**带 scene**）、删同键过期/已消费行、`crypto/rand` 生成码（**不用 math/rand**）、插 `sms_codes(send_status=0)`、插 `sms_logs(status=0)`、回写 `log_id`。

- [x] **步骤 4：运行验证通过**

运行：`go test ./provider/ -run 'TestSmsSend|TestSmsIp|TestSmsDoesNotReuseCodeCache' -v`
预期：第一轮九条 PASS。

- [x] **步骤 5：第二轮失败测试（平台调用、可观测、清理）**

- `TestSmsSendPendingLogBeforeCall`：假 sender 的 `beforeSend` 里查库，断言此刻已存在 `status=0` 的 `sms_logs` 行（**先落 pending 再调平台**，进程崩了也有痕迹）。
- `TestSmsSendWritesBackSuccess`：假 sender 返回 `BizId="B-1"` → `sms_logs.status==1`、`biz_id=="B-1"`、`cost_ms>0`；`sms_codes.send_status==1`、`biz_id=="B-1"`；返回 `SmsSendResult{ExpiresIn: CodeTTL, ResendAfter: ResendInterval}`。
- `TestSmsSendPlatformFailureInvalidatesCode`：假 sender 返回 `&sms.SendError{Class: ClassPermanent, Code:"isv.X", Message:"m"}` → 返回 error；`sms_codes.send_status == -1`（该码作废）；`sms_logs.status==-1`、`err_code=="isv.X"`、`err_msg` 非空、`retryable==0`。transient 错误则 `retryable==1`。
- `TestSmsSendNoNetworkInTransaction`：go/ast 断言 `sendSmsCodeWithSender` 内 `sender.Send` 调用**不在**任何 `w.DB.Transaction(...)` 的函数字面量里。反向对照：合成一份把 Send 挪进事务的源码，必须变红。
- `TestSmsSendResultHasNoCode`：go/ast 断言 `SmsSendResult` 只有 `ExpiresIn`／`ResendAfter` 两个字段（**绝不返回验证码**）。反向对照：合成加 `Code string` 的源码必须变红。
- `TestSmsCodeHashIsKeyedHMAC`：发码后读库，断言 `code_hash` 是 64 位 hex，且**不等于** `sha256(明文码)`（证明用了 TokenSecret 而非裸哈希）。
- `TestSmsLogRetentionPrune`：插入 `created_time` 早于 `now - LogRetentionDays*86400` 的日志 → 发码后被删；go/ast 断言清理语句带 `Limit(1000)`（单次限量，避免长事务）。

- [x] **步骤 6：运行验证失败**

运行：`go test ./provider/ -run 'TestSmsSendPendingLogBeforeCall|TestSmsSendWritesBack|TestSmsSendPlatformFailure|TestSmsSendNoNetworkInTransaction|TestSmsSendResultHasNoCode|TestSmsCodeHashIsKeyedHMAC|TestSmsLogRetentionPrune' -v`
预期：新增七条 FAIL。

- [x] **步骤 7：实现第二轮**

补规格 §6.3 的第 8～11 步：事务外 `sender.Send`（`context.WithTimeout` **10s，不重试**）→ 回写两张表 → 失败置 `send_status=-1` → 惰性清理超期日志（`Limit(1000)`）。`OutId` 填 `SmsLog.Id`。`SendRequest.Text` 由 Twilio 的 `Texts[scene]` 把 `{{code}}` 替换成验证码得到；阿里云／腾讯云走 `Code` 字段。

- [x] **步骤 8：运行验证通过**

运行：`go test ./provider/ -run 'TestSmsSend|TestSmsIp|TestSmsDoesNotReuseCodeCache|TestSmsCodeHashIsKeyedHMAC|TestSmsLogRetentionPrune' -v && go build ./...`
预期：全部 PASS。

- [x] **步骤 9：Commit**

```bash
gofmt -w provider/smsService.go provider/smsSend.go provider/smsSend_test.go
git add provider/smsService.go provider/smsSend.go provider/smsSend_test.go
git commit -m "feat(短信): OTP 发放流程，五层防刷与发送留痕"
```

---

### 任务 9：OTP 校验（行锁 + 一次性消费）

**文件：**

- 创建：`provider/smsVerify.go`
- 测试：`provider/smsVerify_test.go`

**Interfaces：** `func (w *Website) VerifySmsCode(country, phone, scene, code string) error`。

- [x] **步骤 1：编写失败的测试**

复用任务 8 的 `setupSmsTest`／`smsTestPhone`／假 sender。

- `TestSmsVerifyHappyPath`：发码 → 用正确码校验 → `err == nil`，库里 `consumed_at > 0`。
- `TestSmsVerifyRejectsWrongCode`：错码 → `ErrSmsCodeMismatch`，`errors.As` 取到 `*sms.AttemptsError` 且 `Remaining == MaxAttempts-1`；**重开一次连接再查库，`attempts` 必须是 1**（写回 DB，不是只在内存加）。
- `TestSmsVerifyLocksAfterMaxAttempts`：连错 `MaxVerifyAttempts` 次 → 最后一次之后返回 `ErrSmsCodeLocked`；此时**用正确码也返回 `ErrSmsCodeLocked`**。
- `TestSmsVerifySceneIsolation`：`login` 的码拿去校验 `reset` → `ErrSmsCodeNotFound`。
- `TestSmsVerifyExpired`：手工把 `expires_at` 改到过去 → `ErrSmsCodeExpired`。
- `TestSmsVerifySendFailed`：手工把 `send_status` 改成 `-1` → `ErrSmsSendFailed`。
- `TestSmsVerifyReplay`：正确码校验成功后再校验一次 → `ErrSmsCodeNotFound`（`consumed_at=0` 过滤掉了）。
- `TestSmsVerifyConcurrent`（**审查重点 3**）：两条子用例 —— ① 20 个 goroutine 同时用**正确码**校验 → **恰好 1 个成功**，其余全 `ErrSmsCodeNotFound`；② 20 个 goroutine 同时用**错码**校验、`MaxVerifyAttempts=5` → 库里 `attempts` 最终 == 20（每次失败都落库，不被并发覆盖），且成功次数为 0。**反向对照**：把行锁 `FOR UPDATE` 去掉后重跑 ① 必须变红。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestSmsVerify' -v`
预期：编译失败，`undefined: VerifySmsCode`。

- [x] **步骤 3：实现 `provider/smsVerify.go`**

按规格 §6.4 八步。取行语句逐字：
```
WHERE site_id=? AND phone_country=? AND phone=? AND scene=? AND consumed_at=0
ORDER BY id DESC LIMIT 1 FOR UPDATE
```
用 `clause.Locking{Strength: "UPDATE"}`，整段包在 `w.DB.Transaction` 里。比对用 `crypto/subtle.ConstantTimeCompare`。判定顺序不可换：无行 → 过期 → 发送失败 → 已锁定 → 比对。`Attempts++` 与 `ConsumedAt` 都在同一事务内写回。

- [x] **步骤 4：运行测试验证通过**

运行：`go test ./provider/ -run 'TestSmsVerify' -v && go build ./... && go vet ./provider/`
预期：八条 PASS。

- [x] **步骤 5：Commit**

```bash
gofmt -w provider/smsVerify.go provider/smsVerify_test.go
git add provider/smsVerify.go provider/smsVerify_test.go
git commit -m "feat(短信): OTP 校验，行锁保证一次性消费与失败计数落库"
```

---

### 任务 10：前台发码端点 + 短信登录（自动注册）

**文件：**

- 创建：`request/sms.go`、`controller/apiSms.go`、`provider/smsLogin.go`
- 修改：`config/constant.go:162-164`（加 `PlatformSms`）、`request/user.go:125-148`、`controller/apiUser.go:146`、`route/base.go:71`
- 测试：`controller/apiSms_test.go`、`provider/smsLogin_test.go`

**Interfaces（任务 11／12／13 复用）：**
```go
// controller/apiSms.go
func respondSmsError(ctx iris.Context, site *provider.Website, err error)
// 统一出口：msg = site.TplTr(err.Error())；errors.As 命中 *sms.WaitError / *sms.AttemptsError
// 时把 Seconds / Remaining 作为 TplTr 的可变参数传入（Tr 签名支持 args）。
// code 恒为 config.StatusFailed。
func ApiSmsSend(ctx iris.Context)
// provider/smsLogin.go
func (w *Website) LoginViaSms(req *request.ApiLoginRequest) (*model.User, error)
```

- [x] **步骤 1：编写失败的测试**

`controller/apiSms_test.go`（复用本包已有的 `readFuncSource`／`mustContain`／`mustNotContain`）：

- `TestApiSmsSendVerifiesCaptcha`：源码级断言 `ApiSmsSend` 内出现 `currentSite.Safe.Captcha == 1` 与 `Store.Verify(req.CaptchaId, req.Captcha, true)`，且校验在 `SendSmsCode` 调用**之前**。反向对照：合成一份删掉 captcha 分支的源码必须变红。
- `TestApiSmsSendBindOldRequiresLogin`：断言 `scene == bind_old` 分支同时要求「已登录」与「号码等于当前用户绑定号码」两个条件；缺任一即拒。反向对照：合成只判登录不判号码的源码必须变红（否则是免费短信轰炸跳板）。
- `TestApiSmsSendRejectsTestScene`：断言前台拒绝 `scene == "test"` 与 `notify.` 前缀。
- `TestApiLoginDispatchesSms`：断言 `ApiLogin` 出现 `req.Platform == config.PlatformSms` 分支并调 `LoginViaSms`。

`provider/smsLogin_test.go`（真库，复用 `setupSmsTest`／`smsTestPhone`／假 sender）：

- `TestLoginViaSmsExistingUser`：先建号 → 发码 → 登录成功，返回的 user.Id 等于既有号。
- `TestLoginViaSmsAutoRegisters`：未注册号 → 发码 → 登录成功且库里多了一个用户；断言 `UserName` **不等于手机号**、长度为 12；`PhoneCountry`／`Phone` 与入参一致。
- `TestLoginViaSmsRespectsAuditPolicy`：把 `PluginUser` 默认状态设为待审核 → 自动注册后**拒绝登录**并提示待审核（不得绕过站点审核）。
- `TestLoginViaSmsRejectsWrongCode`：错码 → 失败，且**不创建用户**（校验在建号之前）。
- `TestLoginViaSmsRejectsBannedUser`：既有号 `Status=-1` → 拒绝登录。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./controller/ -run 'TestApiSmsSend|TestApiLoginDispatchesSms' -v` 与 `go test ./provider/ -run 'TestLoginViaSms' -v`
预期：均 FAIL（符号未定义）。

- [x] **步骤 3：实现请求结构与常量**

`config/constant.go` 在既有平台常量组里加 `PlatformSms = "sms"`。
`request/sms.go` 新增 `ApiSmsSendRequest{Scene, PhoneCountry, Phone, CaptchaId, Captcha string}`，json tag 逐字按规格 §7.1 表格。
`request/user.go`：`ApiLoginRequest` 加 `PhoneCountry string \`json:"phone_country"\``；`ApiRegisterRequest` 加同名字段。**一并订正两处误导注释**：`ApiLoginRequest.Code` 与 `ApiRegisterRequest.Code` 现在确实承载短信验证码。

- [x] **步骤 4：实现 `provider/smsLogin.go`**

`LoginViaSms` 按规格 §6.5：归一化 → `VerifySmsCode(country, phone, SceneLogin, req.Code)` → `GetUserInfoByPhoneCountry` → 存在则查 `Status`（1 正常／0 待审核／-1 禁止）→ 不存在则合成 `request.ApiRegisterRequest{UserName: 随机12位, Password: 随机32位, Phone, PhoneCountry}` 调 `w.RegisterUser`，撞名重试 ≤3 次。
随机串用 `crypto/rand`。**UserName 不用手机号**（部分模板会公开显示用户名）。注册后仍要过 `GetDefaultStatus()` 审核门。成功后走既有 `GetUserAuthToken` + `LogLogin`，与密码登录同形。

- [x] **步骤 5：实现 `controller/apiSms.go` 与路由**

`respondSmsError` 见 Interfaces。`ApiSmsSend`：`ctx.ReadJSON` → `provider.CurrentSite(ctx)` → scene 白名单（拒 `test` 与 `notify.` 前缀）→ `Safe.Captcha == 1` 时 `Store.Verify` → `bind_old` 要求登录态且号码等于当前用户绑定号码 → 调 `SendSmsCode`（`Ip` 取 `ctx.RemoteAddr()`，**不自己解析 X-Forwarded-For**）→ 成功回 `{code: config.StatusOK, data: {expires_in, resend_after}}`。
`route/base.go` 在 `api.Post("/login", ...)` 附近加 `api.Post("/sms/send", controller.ApiSmsSend)`。
`controller/apiUser.go` 在 `config.PlatformGoogle` 分支之后加 `else if req.Platform == config.PlatformSms` 分支，调 `LoginViaSms`，错误处理与相邻分支同形。

- [x] **步骤 6：运行测试验证通过 + 重生成端点清单**

运行：`go test ./controller/ -run 'TestApiSmsSend|TestApiLoginDispatchesSms' -v && go test ./provider/ -run 'TestLoginViaSms' -v && go build ./... && go run ./cmd/apicatalog && go run ./cmd/apicatalog -check`
预期：测试 PASS，catalog 校验一致。

- [x] **步骤 7：Commit**

```bash
gofmt -w request/sms.go controller/apiSms.go provider/smsLogin.go controller/apiSms_test.go provider/smsLogin_test.go
git add request/sms.go request/user.go config/constant.go controller/apiSms.go controller/apiUser.go provider/smsLogin.go route/base.go controller/apiSms_test.go provider/smsLogin_test.go api_catalog.json
git commit -m "feat(短信): 前台发码端点与手机号验证码登录（未注册自动注册）"
```

---

### 任务 11：注册手机号校验 + 短信找回 + 换绑

**文件：**

- 修改：`request/sms.go`、`controller/apiSms.go`、`controller/apiUser.go:17`（`ApiRegister`）、`provider/smsVerify.go`（抽出 `verifySmsCodeTx`）、`route/base.go`
- 创建：`provider/smsBind.go`
- 测试：`controller/apiSmsBind_test.go`、`provider/smsBind_test.go`

**Interfaces：**
```go
func ApiSmsPasswordReset(ctx iris.Context)
func ApiSmsPhoneBind(ctx iris.Context)
func (w *Website) BindUserPhone(userId uint, oldCode, newCountry, newPhone, newCode string) error
```
请求结构 `ApiSmsPasswordResetRequest{PhoneCountry, Phone, Code, Password}`、`ApiSmsPhoneBindRequest{OldCode, NewPhoneCountry, NewPhone, NewCode}`，json tag 按规格 §7.1。

- [x] **步骤 1：编写失败的测试**

`provider/smsBind_test.go`（真库）：

- `TestBindUserPhoneBothCodes`：用户已绑旧号 → 旧号新号各发一码 → 换绑成功，库里 `phone`／`phone_country` 变成新号，**两个码的 `consumed_at` 都 > 0**。
- `TestBindUserPhoneRollsBackOnNewCodeFailure`：旧号码正确、新号码错误 → 返回 error，**旧号的码未被消费**（`consumed_at == 0`），用户手机号不变。这条钉住「同一事务」。
- `TestBindUserPhoneFirstBindSkipsOld`：用户 `phone == ""` → 只验新号即可绑定成功（唯一例外，必须显式覆盖）。
- `TestBindUserPhoneRejectsDuplicate`：新号已被**别的**用户占用 → 失败；新号就是自己当前号 → 允许（排除自己）。

`controller/apiSmsBind_test.go`（源码级，复用 `readFuncSource`）：

- `TestApiRegisterForcesSmsCodeWhenPhonePresent`：断言 `ApiRegister` 里「`req.Phone != ""` → 校验 `VerifySmsCode(..., SceneRegister, req.Code)`」，且**没有**「`code` 为空就跳过」的旁路。反向对照：合成一份 `if req.Code != ""` 才校验的源码必须变红。
- `TestApiRegisterUnchangedWithoutPhone`：断言不传 `phone` 时不进入短信校验分支。
- `TestApiSmsResetHidesRegistrationState`：断言 reset 发码路径对未注册号码返回与已注册号码**同一文案**，不出现 `UserDoesNotExist` 之类的区分提示。
- `TestApiSmsPhoneBindRequiresLogin`：断言路由挂了 `middleware.UserAuth`。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./provider/ -run 'TestBindUserPhone' -v` 与 `go test ./controller/ -run 'TestApiRegister|TestApiSms' -v`
预期：均 FAIL。

- [x] **步骤 3：实现 `provider/smsBind.go`**

`BindUserPhone` 在**一个** `w.DB.Transaction` 内：取当前用户 → `phone != ""` 时先 `VerifySmsCode(旧号, SceneBindOld, oldCode)` → `VerifySmsCode(新号, SceneBindNew, newCode)` → 新号唯一性查重（`WHERE phone = ?` 单列 + 应用层比区号，**排除自己**）→ 更新 `phone`／`phone_country`。任一步失败整体回滚。
⚠️ 校验函数内部自带事务，嵌套调用必须走同一个 `tx`：给 `smsVerify.go` 抽一个 `verifySmsCodeTx(tx *gorm.DB, ...)` 内部函数，`VerifySmsCode` 与 `BindUserPhone` 都调它。**否则外层回滚不会撤销内层已提交的 `consumed_at`。**

- [x] **步骤 4：实现两个 handler 与注册扩展**

`ApiSmsPasswordReset`：校验 scene=`reset` 的码 → 查用户 → 新密码 ≥6 位 → 更新密码。查不到用户时的提示语与验证码错误**完全一致**，不做区分。
`ApiSmsPhoneBind`：`middleware.UserAuth` 保护，转调 `BindUserPhone`。
`ApiRegister`：`req.Phone != ""` 时强制校验 `register` 场景码，失败则整个注册失败；不传 `phone` 时行为与现在完全一致。
`route/base.go` 加 `api.Post("/sms/password/reset", controller.ApiSmsPasswordReset)` 与 `api.Post("/sms/phone/bind", middleware.UserAuth, controller.ApiSmsPhoneBind)`。
**不动**既有 `POST /api/password/reset` 的 email-token 逻辑。

- [x] **步骤 5：运行测试验证通过 + 重生成清单**

运行：`go test ./provider/ -run 'TestBindUserPhone|TestSmsVerify' -v && go test ./controller/ -run 'TestApiRegister|TestApiSms' -v && go build ./... && go run ./cmd/apicatalog && go run ./cmd/apicatalog -check`
预期：全部 PASS（含任务 9 的回归）。

- [x] **步骤 6：Commit**

```bash
gofmt -w provider/smsBind.go provider/smsBind_test.go controller/apiSmsBind_test.go
git add request/sms.go controller/apiSms.go controller/apiUser.go provider/smsBind.go provider/smsVerify.go route/base.go provider/smsBind_test.go controller/apiSmsBind_test.go api_catalog.json
git commit -m "feat(短信): 注册手机号校验、短信找回密码与手机号换绑"
```

---

### 任务 12：后台配置端点（读掩码 + 部分更新 + 回写保护）

**文件：**

- 创建：`controller/manageController/pluginSms.go`
- 修改：`provider/website.go:87` 附近（加 `PluginSms`）、`provider/setting.go:46`／`:123`／`:439` 附近（key、加载分派、`LoadSmsSetting`）、`route/manage.go:362` 之后
- 测试：`controller/manageController/pluginSms_test.go`

**Interfaces（任务 13／15 依赖）：**
```go
const SmsSecretMask = "********" // manageController 包，与 SendmailPasswordMask 同串
func PluginSmsSetting(ctx iris.Context)     // GET
func PluginSmsSettingForm(ctx iris.Context) // POST
// provider
const SmsSettingKey = "sms"
func (w *Website) LoadSmsSetting(value string)
```

- [x] **步骤 1：编写失败的测试**

- `TestSmsSecretFieldsMasked`：构造带真值的 `config.PluginSmsConfig`，走读端点的脱敏函数，断言**八个字段**变成 `********`：阿里云 `access_key_id`／`access_key_secret`（国内+国际共 4 个）、腾讯云 `secret_id`／`secret_key`（4 个）、Twilio `account_sid`／`auth_token`／`api_key_sid`／`api_key_secret`（4 个）。同时断言**不掩码**的字段原样保留：`sms_sdk_app_id`／`region`／`endpoint`／`sign_name`／`from`／`messaging_service_sid`／`templates`／`texts`。空凭据保持空串（不要给空值也打掩码，否则站长分不清配没配）。**再断言脱敏调用之后 `currentSite.PluginSms` 里的真值逐字段不变** —— 这是行为侧的证据，与下一条的源码侧护栏互补。
- `TestSmsSettingMasksValueCopyNotPointer`：go/ast 断言脱敏前出现 `*currentSite.PluginSms` 解引用（**值拷贝**）。反向对照：合成 `setting := currentSite.PluginSms` 的源码必须变红。这条防的是 2026-10-03 真实事故——`PluginSms` 是指针，直接赋值后改掩码会把内存里的真凭据抹掉，之后所有发信失败且无报错。
- `TestSmsMaskWriteBackProtected`：POST 带 `********` → 保存后库里仍是原真值；POST 带新值 → 更新；POST 带空串 → **真的清空**（空串是主动清空，不是哨兵）。八个字段逐个跑。**反向对照**：删掉其中任一个字段的哨兵分支后重跑，该字段必须变红。
- `TestSmsSettingNullNestedMap`（**审查重点 4**）：两条子用例钉住 `library/merge.go:40` 的真实行为 —— ① 顶层 `{"aliyun": null}` → 配置**不变**（结构体 null 是 no-op）；② `{"aliyun":{"templates":null}}` → 该厂商 templates **被清空**。第二条是静默数据丢失，测试名与注释必须写明「前端不得提交 null map，见任务 14 的 payload 护栏」。
- `TestSmsSettingPartialUpdateKeepsUnsentFields`：只提交 `{"code_ttl":600}` → 其余字段（含三个厂商对象）全部保持原值。
- `TestSmsSettingFormKeepsNamedReadJSON`：go/ast 断言 `PluginSmsSettingForm` 内**恰好一次**具名 `ctx.ReadJSON(&req)`（`req` 是 `config.PluginSmsConfig` 而非 map），且出现 `library.MergeJSONPayload`。反向对照：合成把 `ReadJSON` 目标换成 map 的源码必须变红（会让 apicatalog 的 params 归零）。

- [x] **步骤 2：运行测试验证失败**

运行：`go test ./controller/manageController/ -run 'TestSms' -v`
预期：FAIL（符号未定义）。

- [x] **步骤 3：接进站点配置链路**

`provider/website.go` 在 `PluginSendmail` 附近加 `PluginSms *config.PluginSmsConfig`。
`provider/setting.go`：加 `SmsSettingKey = "sms"`；在 `LoadSendmailSetting` 的分派旁加 `w.LoadSmsSetting(settingMap[SmsSettingKey])`；实现 `LoadSmsSetting`，仿 `LoadSendmailSetting` 但**加载后调 `sms.ApplyDefaults`** 补齐默认值。

- [x] **步骤 4：实现两个 handler 与路由**

读端点：`setting := *currentSite.PluginSms`（**解引用值拷贝**）→ 八个凭据字段非空则置 `SmsSecretMask` → 回 `{code: config.StatusOK, data: setting}`。
写端点严格按规格 §3.5 六步顺序：具名 `ctx.ReadJSON(&req)` → `req = *currentSite.PluginSms` 作基底 → `ctx.GetBody()` + `json.Unmarshal` 取 payload → `library.MergeJSONPayload(payload, &req)` → 逐字段哨兵回写保护（值为 `********` 则取 `currentSite.PluginSms` 对应真值）→ 赋回 `currentSite.PluginSms` → `SaveSettingValue(SmsSettingKey, ...)` + `AddAdminLog`。
在写端点上方写注释记录 null 的两种行为（顶层 no-op / 嵌套 map 清空）。
`route/manage.go` 在 `sendmail` party 之后加 `sms := plugin.Party("/sms")`，注册 `sms.Get("/setting", ...)` 与 `sms.Post("/setting", ...)`，party 上挂 **`middleware.AdminPermissionRequired`**。

- [x] **步骤 5：运行测试验证通过 + 重生成清单**

运行：`go test ./controller/manageController/ -run 'TestSms' -v && go build ./... && go run ./cmd/apicatalog && go run ./cmd/apicatalog -check`
预期：六条 PASS，catalog 一致。

- [x] **步骤 6：Commit**

```bash
gofmt -w controller/manageController/pluginSms.go controller/manageController/pluginSms_test.go
git add provider/website.go provider/setting.go controller/manageController/pluginSms.go controller/manageController/pluginSms_test.go route/manage.go api_catalog.json
git commit -m "feat(短信): 后台短信配置端点，读掩码与哨兵回写保护"
```

---

### 任务 13：后台测试发送 + 发送记录

**文件：**

- 修改：`controller/manageController/pluginSms.go`、`route/manage.go`
- 测试：`controller/manageController/pluginSmsOps_test.go`

**Interfaces：**
```go
func PluginSmsTest(ctx iris.Context) // POST /plugin/sms/test
func PluginSmsLogs(ctx iris.Context) // GET  /plugin/sms/logs
func (w *Website) GetSmsLogList(filter func(*gorm.DB) *gorm.DB, page, pageSize int) ([]*model.SmsLog, int64)
```

- [ ] **步骤 1：编写失败的测试**

- `TestPluginSmsTestUsesLoginTemplate`：`test` 场景取该 region 下 **`login`** 场景的模板；`login` 模板为空时**明确报错**（`ErrSmsTemplateMissing`），不静默套用别的场景。
- `TestPluginSmsTestBypassesSceneWhitelistOnly`：`Scenes` 为空 → `test` 仍可发（否则站长关掉全部场景就没法测试）；但号码日限与 IP 限额**仍然生效**（源码级断言走的是同一条 `SendSmsCode` 链路，不是旁路）。
- `TestPluginSmsLogsFilters`：表驱动断言 `scene`／`provider`／`status`／`phone`／`start_time`／`end_time` 六个筛选各自生效，且**恒带 `site_id` 条件**（多站点隔离）。
- `TestPluginSmsLogsNeverReturnsCode`：go/ast 断言返回结构里不含验证码字段；响应 JSON 序列化后不出现 `code_hash`（`json:"-"` 已在任务 6 钉住，这里做端到端复核）。
- `TestPluginSmsOpsRequireAdminPermission`：go/ast 解析 `route/manage.go`，断言 `/plugin/sms` party 上挂了 `AdminPermissionRequired`（**不是** `AdminPermission`）。反向对照：合成换成 `AdminPermission` 的源码必须变红。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./controller/manageController/ -run 'TestPluginSms' -v`
预期：新增五条 FAIL。

- [ ] **步骤 3：实现**

`PluginSmsTest`：读 `{phone_country, phone}` → 归一化定 region → 取该 region 的 `login` 模板（空则报错）→ 调 `SendSmsCode`，`Scene` 传 `sms.SceneTest`，`Ip` 取 `ctx.RemoteAddr()`，`UserId` 传 `0`（管理员不是前台用户，`sms_logs.user_id` 只对前台发码有意义）→ 回发送结果与失败原因。
**错误文案**：`manageController` 与 `controller` 是不同包，用不了任务 10 的 `respondSmsError`。本包直接 `currentSite.Tr(err.Error(), args...)`，`errors.As` 命中 `*sms.WaitError`／`*sms.AttemptsError` 时把数值作为可变参数传入，口径与前台一致。
`PluginSmsLogs`：分页 + 六个筛选，恒带 `site_id`，按 `created_time DESC` 排序（命中 `idx_site_created`）。
`GetSmsLogList` 放 `provider/smsService.go`，签名见 Interfaces，仿仓库既有 `GetXxxList` 范式。
路由：`sms.Post("/test", ...)`、`sms.Get("/logs", ...)`，沿用任务 12 的 party 与权限中间件。

- [ ] **步骤 4：运行测试验证通过 + 重生成清单**

运行：`go test ./controller/manageController/ -run 'TestSms|TestPluginSms' -v && go build ./... && go run ./cmd/apicatalog && go run ./cmd/apicatalog -check`
预期：全部 PASS。

- [ ] **步骤 5：Commit**

```bash
gofmt -w controller/manageController/pluginSmsOps_test.go
git add controller/manageController/pluginSms.go provider/smsService.go route/manage.go controller/manageController/pluginSmsOps_test.go api_catalog.json
git commit -m "feat(短信): 后台测试发送与发送记录查询"
```

---

### 任务 14：后台前端配置页

**文件：**

- 创建：`anqicms-admin/src/services/plugin/sms.ts`、`anqicms-admin/src/pages/plugin/sms/index.tsx`、`anqicms-admin/src/pages/plugin/sms/payload.ts`、`anqicms-admin/scripts/check-sms-payload.mjs`
- 修改：`anqicms-admin/config/routes.ts:545-603`、`anqicms-admin/src/locales/{zh-CN,en-US,zh-TW}/plugin.ts`、`config/menu.go`

**Interfaces：** `buildSmsPayload(values: Record<string, any>): Record<string, any>`（纯函数，无 React 依赖，可单独跑）。

**前端验证约束（不可违反）：** 仓库**没有** jest／vitest，**不新增测试框架**。纯逻辑用 `node_modules/.bin/esbuild` 转译后交给 node 断言。**不跑 `npx tsc --noEmit` 全量**（会被 SIGTERM），**不跑 `npm run format`**（会动全仓）。仓库存量 38 个类型错误，判断是否引入新错误只看是否命中本次改动文件。

- [ ] **步骤 1：编写失败的测试 `check-sms-payload.mjs`**

脚本用 esbuild 把 `payload.ts` 转成 CJS 临时文件（写进 `tmp/`），require 后用 `node:assert` 断言：

- **null map 护栏（审查重点 4 的前端侧）**：输入 `{aliyun:{domestic:{templates:null, access_key_id:'********'}}}` → 输出里 `aliyun.domestic.templates` **不是 null**（要么省略该键，要么是 `{}`）。`texts` 同理。三家六处都断言。
- **完整子树**：输入只改了 `aliyun.domestic.sign_name`，输出仍带 `aliyun.domestic` 的其余全部键与 `aliyun.international` 整棵子树（`MergeJSONPayload` 只处理顶层，厂商对象是整体替换）。
- **掩码原样保留**：`********` 不被清空、不被转成空串。
- **未生效厂商也提交**：`provider='aliyun'` 时 `tencent`／`twilio` 子树仍在 payload 里。
脚本退出码非 0 即失败。

- [ ] **步骤 2：运行验证失败**

运行：`node anqicms-admin/scripts/check-sms-payload.mjs`（在仓库根执行）
预期：FAIL，找不到 `payload.ts`。

- [ ] **步骤 3：实现 `payload.ts` 与 `sms.ts`**

`payload.ts` 只导出 `buildSmsPayload`，纯函数，规则见步骤 1 的四条断言。
`services/plugin/sms.ts` 仿 `services/plugin/sendmail.ts`：`import { get, post } from '../tools';`，四个函数 `pluginSmsSetting()`（GET `/plugin/sms/setting`）、`pluginSmsSettingSave(body)`（POST 同路径）、`pluginSmsTest(body)`（POST `/plugin/sms/test`）、`pluginSmsLogs(params)`（GET `/plugin/sms/logs`），签名统一 `(params?: any, options?: {[key:string]:any})`。

- [ ] **步骤 4：运行验证通过**

运行：`node anqicms-admin/scripts/check-sms-payload.mjs`
预期：全部断言通过，退出码 0。

- [ ] **步骤 5：实现页面、路由、菜单与三语文案**

`pages/plugin/sms/index.tsx`：六个 tab（基础设置／阿里云／腾讯云／Twilio／发送测试／发送记录），**visitedTabs 常驻挂载 + `display:none` 切换**（照抄 `pages/plugin/channels/index.tsx` 的范式，条件渲染会卸载表单丢输入）。配置类 tab 共用一个 ProForm，`onFinish` 用 `formRef.current?.getFieldsValue(true)` 取全量再经 `buildSmsPayload`。
厂商 tab **始终可见**，未生效的顶部标「当前未生效」；Twilio tab 顶部固定提示「仅支持国际号码，+86 号码将被拒发」。凭据字段 placeholder 写明「`********` 表示未改动，原样保存即可保持不变」，读回掩码**不清空输入框**。场景模板留空的语义在界面上说清：留空 = 该场景不可用，发码时明确报错。区号输入默认 `86` + 常用区号下拉。保存成功后 `message.success` 并重新拉取配置。
`config/routes.ts` 在 `/plugin/channel` 的 routes 里加规格 §8.1 的路由项（`path: '/plugin/channel/sms'`、`type: 'system'`、`icon: 'sendmail'`）。
`config/menu.go` 在 `/plugin/channel/sendmail` 之后加规格 §8.1 的菜单项。**`Path` 必须与前端 `path` 逐字一致** —— 它同时是管理员分组权限的 KEY。
三语 `plugin.ts` 补 `plugin.sms.*` 全部文案键，三语键集合必须一致。

- [ ] **步骤 6：语法与文案校验**

运行：
```bash
cd anqicms-admin && ./node_modules/.bin/esbuild --loader=tsx --jsx=automatic \
  src/pages/plugin/sms/index.tsx src/pages/plugin/sms/payload.ts src/services/plugin/sms.ts \
  --outdir=../tmp/smscheck && cd ..
node anqicms-admin/scripts/check-sms-payload.mjs
go build ./... && go vet ./config/
```
预期：esbuild 无报错，payload 断言通过，Go 侧编译通过。
再人工核对三语键集合一致（用 `node -e` 比对三个 locale 文件里 `plugin.sms.` 前缀的键名集合，差集必须为空）。

- [ ] **步骤 7：Commit**

```bash
git add anqicms-admin/src/services/plugin/sms.ts anqicms-admin/src/pages/plugin/sms/index.tsx anqicms-admin/src/pages/plugin/sms/payload.ts anqicms-admin/scripts/check-sms-payload.mjs anqicms-admin/config/routes.ts anqicms-admin/src/locales/zh-CN/plugin.ts anqicms-admin/src/locales/en-US/plugin.ts anqicms-admin/src/locales/zh-TW/plugin.ts config/menu.go
git commit -m "feat(短信): 后台短信服务配置页与菜单路由"
```

---

### 任务 15：MCP `channel_sms` 意图接入

**文件：**

- 修改：`pkg/mcp/intent/catalog_domains.go`（意图定义 + `gatedDomainIntents` + `RecommendedExposed` 核对）、`pkg/mcp/intent/domain.go:62` 附近、`pkg/mcp/intent/mask.go`、`pkg/mcp/intent/catalog_domains_test.go`（`defaultOpenDomainIntents` 核对）、`provider/api_intent_routes_test.go:519`／`:573`
- 测试：`pkg/mcp/intent/catalog_sms_test.go`

**Interfaces：** 意图 `channel_sms`，action `setting_get|setting_save|test|logs`，映射到任务 12／13 的四个端点。

- [ ] **步骤 1：编写失败的测试**

- `TestSmsIntentStaysGated`：断言 `channel_sms` 存在、`DefaultOff == true`、`Domain == DomainChannel`、`Risk == RiskWrite`，且在 `gatedDomainIntents` 里有非空理由。**反向对照**：把 `channel_sms` 挪进 `provider/api_intent_routes_test.go` 的 `dailyOpen`（或从 `gatedDomainIntents` 移除）后重跑，必须变红 —— 四处名单缺一处即红。
- `TestSmsIntentParamsMatchEndpoints`：断言意图 `Params` 的每个键都能被对应端点的请求结构体 json tag 认识（复用仓库既有的 `TestIntentParamsRecognizedByEndpoints` 范式）。**类型必须与 json tag 严格一致**——漏了不报错，只是值被静默清空。
- `TestSmsSecretFieldNamesCovered`：表驱动断言 `IsSecretField` 对 `auth_token`／`account_sid`／`api_key_sid`／`api_key_secret`／`access_key_id`／`access_key_secret`／`secret_id`／`secret_key` 全部返回 true；**`token_expire`／`token_type`／`sign_name`／`templates`／`from`／`messaging_service_sid`／`sms_sdk_app_id` 全部返回 false**。反向对照：加宽泛 `_token` 后缀会让 `token_expire` 变 true，必须被这条测试抓住。
- `TestSmsDomainMapping`：断言 `domainByPlugin["sms"] == DomainChannel`。

- [ ] **步骤 2：运行测试验证失败**

运行：`go test ./pkg/mcp/intent/ -run 'TestSms' -v`
预期：FAIL（意图不存在）。

- [ ] **步骤 3：加意图与四处名单**

意图定义逐字照抄规格 §9.1（含 Desc 里的三条 ⚠️：厂商对象整体替换、掩码语义、Twilio 只支持国际）。`Compose` 用 `invokeRoutes("channel_sms", map[string]string{...})` 映射四个端点。
**四处平行名单同步**（漏一处就红）：① `intent.gatedDomainIntents` **加入**并写理由；② 测试内 `intent.defaultOpenDomainIntents` **不得包含**；③ `RecommendedExposed()` **不得包含**；④ `provider/api_intent_routes_test.go` 的 `mustStayGated` **加入**、`dailyOpen` **不得包含**。
`domain.go` 的 `domainByPlugin` 补 `"sms": DomainChannel`（紧邻 `"sendmail"`）；`catalog_domains.go:1088` 附近的风险描述映射补一条。

- [ ] **步骤 4：补掩码名单**

`mask.go` 的 `IsSecretField` 补精确名 `auth_token`／`account_sid`／`api_key_sid` 与后缀 `_auth_token`。**绝不加宽泛的 `_token` 后缀**（会把 `token_expire`／`token_type` 打成掩码，让调用方误判配置状态）。

- [ ] **步骤 5：运行测试验证通过**

运行：`go test ./pkg/mcp/intent/ -v && go test ./provider/ -run 'TestApiIntentRoutes|TestIntent' -v && go build ./... && go run ./cmd/apicatalog -check`
预期：intent 整包 PASS（该包可整跑），provider 侧门禁名单 PASS。意图总数 53 → 54，**默认工具面常驻 token 成本不变**（新意图 DefaultOff）。

- [ ] **步骤 6：Commit**

```bash
gofmt -w pkg/mcp/intent/catalog_sms_test.go
git add pkg/mcp/intent/catalog_domains.go pkg/mcp/intent/domain.go pkg/mcp/intent/mask.go pkg/mcp/intent/catalog_domains_test.go pkg/mcp/intent/catalog_sms_test.go provider/api_intent_routes_test.go
git commit -m "feat(短信): MCP channel_sms 意图接入，默认关闭并补齐凭据掩码名单"
```

---

### 任务 16：集成验证与收尾

**文件：**

- 创建：`provider/sms_integration_test.go`
- 修改：`CHANGELOG.md`、`config/constant.go`（版本号，**仅在用户要求发布时才动**）

**这一步不写新产品代码，只做端到端验证与文档。**

- [ ] **步骤 1：写端到端回归测试**

`provider/sms_integration_test.go`（真库 + 假 sender，复用任务 8 基建）：

- `TestSmsFullLoginJourney`：未注册号 → 发 `login` 码 → 登录 → 自动建号 → 拿到 token → 用 token 查用户详情，号码与区号一致。
- `TestSmsFullBindJourney`：建号并绑旧号 → 旧号新号各发码 → 换绑 → 用新号重新登录成功、旧号登录失败。
- `TestSmsFullResetJourney`：建号 → 发 `reset` 码 → 重置密码 → 用新密码走既有密码登录成功。
- `TestSmsSceneCrossReplayRejected`：`login` 码拿去 `reset`、`reset` 码拿去 `bind_new`，全部 `ErrSmsCodeNotFound`。

- [ ] **步骤 2：跑复测三板斧**

```bash
go build ./... && go vet ./...
go test ./library/ ./controller/... ./pkg/mcp/intent/
go test ./provider/ -run 'TestSms|TestLoginViaSms|TestBindUserPhone|TestCreateUserWithPhoneGuard|TestUserPhoneCountryColumn|TestRegisterUserUsesPhoneGuard|TestVerifyCellphoneFormatUnchanged'
go run ./cmd/apicatalog -check
go run ./cmd/apilive -mode all -ns "" -expose "*"
go run ./cmd/apilive -wire
```
预期：全绿。**provider 按 `-run` 分批，不整跑。** `apilive` 的写操作是可逆验证，跑完核对站点数据未被残留修改。

- [ ] **步骤 3：真机 MCP 冒烟（临时端口，不动 8001）**

```bash
go run kandaoni.com/anqicms/main -port 8011 > /tmp/anqicms_sms_verify.log 2>&1
```
判活看日志里的 `AI tools initialized (intent mode)`。**8001 被占用时不要杀原服务。**
用 Bearer token（表 `settings`、行 `key='ai_setting'` 的 `$.mcp.token`；查库用 `~/.workbuddy/binaries/python/envs/default/bin/python` + `pymysql`，`/usr/local/bin/mysql` 已坏）走 `initialize` → 取响应头 `Mcp-Session-Id` → `tools/call`。**响应是 SSE 形态**，要抽 `data:` 行再解析。
验证四件事：① `channel_sms` 默认**不在**工具面；② 打开暴露后 `setting_get` 返回掩码；③ `setting_save` 只传 `code_ttl` 不清掉厂商配置；④ 业务失败时 `isError=true` 且能看到中文拒绝原因。
**验证完恢复 `api_exposure.mode=read`、删临时数据、停 8011。**

- [ ] **步骤 4：前端真机核对**

起前端 dev，进 `/plugin/channel/sms`，逐项核对：六 tab 切换不丢输入、掩码读回不清空、只改一个字段保存后其余配置不变、发送记录筛选生效。
⚠️ **后端未部署真实凭据时，发送测试必然失败**——如实记录「界面可用、真实发送未验证」，**不得把 mock 结果写成通过**。

- [ ] **步骤 5：写 CHANGELOG 与已知限制**

`CHANGELOG.md` 按既有格式分节（新增功能／功能优化／问题修复）。必须写进发布说明的四条：
① 自动注册用户的 UserName 是随机串，**不能用用户名登录**，只能用手机号登录或到资料页补全；
② `DefaultMenuGroups` 只在首次安装／重建菜单时写库，**已运行站点需触发菜单重新同步**，否则新页面进不去；
③ 手机号查重依赖 **REPEATABLE READ**，上线前必须核对生产库隔离级别；
④ 默认前台模板 `template/default/login.html` **没有手机号登录入口**（实测只有一个 Google 登录链接），站长需自行改模板。
**四项无法验证的事如实列出**：真实发送、模板审核通过率、国际到达率、生产库隔离级别。

- [ ] **步骤 6：同步知识图谱并 Commit**

```bash
graphify update   # 增量优先；启动前确认没有其它 graphify 进程在跑
git add provider/sms_integration_test.go CHANGELOG.md
git commit -m "test(短信): 端到端回归与发布说明"
```

---

## 交付后仍未解决的事（不在本计划范围）

1. **用户名与邮箱查重仍是事务外的裸读后写**（`provider/user.go`）。既有缺陷，与短信无关，本期不顺手修——改它会扩大爆炸半径。
2. **IP 滑窗是进程内的**，多实例部署下每台各算一份。这是既有 `library.RateLimiter` 的固有性质，真正的跨进程防线是 DB 计数（号码日限、单码失败上限）。
3. **生产库若为 READ COMMITTED，手机号查重保证不成立**。需另行决策（改隔离级别，或引入独立唯一映射表），不得默认已解决。
4. **Twilio Content SID 映射未做**（YAGNI），后续需要富文本模板时再扩展。
5. **通知场景只留 `notify.` 前缀抽象**，本期不定义具体值、不接业务事件。
6. **trunk 前缀残留放大面**：规格 §6.2 规则 3 要求「本地号码不去前导 0」（意大利等国的 0 是号码有效组成部分）。因此 `(44,"07911123456")` 与 `(44,"7911123456")` 仍是两个合法键，德/法/澳等以 0 为国内长途前缀的国家同理。本期已把放大面从「区号切分（4 倍）」收窄到「trunk 前缀（约 2 倍）」，未彻底消除。彻底单射需要按国家的 trunk 前缀元数据，而「0 个新依赖」禁用 libphonenumber；凭记忆造这张表的失败模式正是规则 3 警告的「去掉会发错人」。是否另开一期由用户决定。