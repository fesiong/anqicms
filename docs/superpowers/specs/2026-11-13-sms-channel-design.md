# 安企CMS 短信渠道设计：阿里云 / 腾讯云 / Twilio

状态：设计已获用户批准（2026-11-13），待编写实现计划。
日期：2026-11-13。
范围：前台用户登录、注册、找回密码、绑定／换绑手机号。通知用途仅预留抽象，本期不实现。

> 本规格是实现的唯一契约。§1 是用户逐轮批准的决策台账，与下文任何推导冲突时以 §1 为准。
> §12 列出本环境无法验证的项，**不得**在实现汇报中写成「已通过」。

## 0. 目标与非目标

### 0.1 目标

为安企CMS 增加短信验证码能力，覆盖四个前台场景：手机号验证码登录（自动注册）、注册时校验手机号、短信找回密码、绑定／换绑手机号。在 `anqicms-admin` 提供完整的配置界面与发送记录，在后端提供配置存储、发送适配与前台 API，并纳入 MCP 意图面。

支持三家服务商：阿里云短信、腾讯云短信、Twilio。同一时刻只有一家生效。

### 0.2 非目标（本期明确不做）

| 不做的事 | 原因 |
|---|---|
| 改 pongo2 前台模板（`template/*/login.html`、`register.html`） | 用户选定「只做后台 + API」。默认模板没有手机号输入框，站长需自行改模板才能用上本功能 |
| 短信通知（订单、评论、审核提醒等） | 用户选定「只留抽象与场景码」。仅预留 `Sender` 接口与场景码命名空间 |
| 厂商故障转移／多厂商并行 | 用户选定「单选一家生效」 |
| 复用 IM 渠道子系统（`MessageChannel` / `PlatformAdapter`） | `Run` 是长连接语义，短信无入站；租约／绑定／收件箱机械对短信是死重。详见 §2.3 |
| 复用存储插件的 AK/SK | 用户选定「独立配置」。存储凭据与短信凭据的授权范围不同，混用会让一处泄露波及两处 |
| 短信上行、状态回调（Delivery Report） webhook | 本期只记录同步返回结果。回调需要公网可达端点与签名校验，属独立需求 |
| 修改 `VerifyCellphoneFormat` 的正则 | 该函数无 `^$` 锚点，收紧会误伤存量带前缀账号。新校验另写，详见 §6.2 |

### 0.3 成功标准

1. 后台能配置三家服务商中任意一家，保存后凭据不回显明文，且不影响其它字段。
2. 前台四个场景的 API 可用，验证码一次性、有有效期、有失败次数上限。
3. 同一 `(区号, 手机号)` 二元组在并发注册下只产生一个用户。
4. 三家签名器的输出与官方 SDK **逐字节一致**，并有仓内向量测试锁定。
5. 每次发送（含失败）在后台发送记录里可查到原因。
6. 复测三板斧全绿：`apilive -mode all` → `apilive -wire` → `apicatalog -check`。

## 1. 决策台账（用户已批准）

[KNOWN] 下表每一行都是用户在头脑风暴中明确选择的，不是推导。实现不得自行改动。

| # | 议题 | 决定 |
|---|---|---|
| 1 | 登录语义 | 手机号 + 验证码登录，号码未注册则**自动注册并登录** |
| 2 | 验证码存储 | **新建两张 DB 表**，不复用 `library.CodeCache` |
| 3 | 前台范围 | **只做后台 + API**，pongo2 模板不动 |
| 4 | 通知预留 | **只留抽象与场景码**，不建投递规则／模板表 |
| 5 | 凭据归属 | **独立配置**，不复用存储插件 AK/SK |
| 6 | 渠道启用 | **单选一家生效**，不做故障转移 |
| 7 | 场景范围 | 登录、注册校验、找回密码、绑定／换绑，**四个全做** |
| 8 | 签名模板现状 | 三家**都还没申请**，无真实凭据可用 |
| 9 | 手机号唯一 | **事务内应用层查重**，不加生成列 + 唯一索引 |
| 10 | 号码范围 | **支持国际号码**，增加区号列，默认中国 |
| 11 | 注册校验 | **有手机号才强制**校验验证码；不传手机号行为不变 |
| 12 | 换绑安全 | **旧号新号都验** |
| 13 | 国际模板 | 国内／国际**完全独立两套** |
| 14 | 号码列语义 | **区号列 + 本地号码**，`phone_country` 默认 `86`，唯一键 = 二元组 |
| 15 | 验证码参数 | **全部后台可配**（长度、有效期、重发间隔、失败上限、各类限额） |
| 16 | MCP 接入 | **纳入 MCP** |
| 17 | Twilio 定位 | **只用于国际**，`+86` 号码拒发 |
| 18 | Twilio 凭据 | **两种都支持**：Account SID + Auth Token，或 API Key SID + Secret |
| 19 | Twilio 发件方 | **两者都支持**：`From` 号码与 Messaging Service SID |
| 20 | 文案契约 | **本地文案 + 平台模板映射**（Twilio 用本地文案，阿里云／腾讯云用平台模板 ID） |
| 21 | 实现方案 | **方案 A：手写 HTTP + 官方 SDK 对照向量**，0 个新依赖 |
| 22 | 槽位语义 | **单厂商 + 厂商内分国内外**，不是两个独立槽位 |

### 1.1 本轮补充批准的两处前提

[INFERRED] 以下两条由组合决策推导，已在设计评审中向用户明示并获批准：

**（a）凭据继承口径。** 决策 13 的原文含「各自 AK/SK、SmsSdkAppId」，决策 22 的原文只说「两套签名模板」。合并口径为：

- 结构上国内／国际各有**独立的凭据字段**。
- 国际凭据**留空则沿用国内凭据**（同一账号跑国内外是常态，强制重复填写属于让用户填拿不到第二份的值）。
- **签名与模板永不沿用**，必须各自填写。阿里云国际站 endpoint 本就不同，签名与模板审核也确实是两套。

**（b）查重依赖 REPEATABLE READ。** 决策 9 的应用层查重只在 RR 隔离级别下成立，详见 §4.4 的实测证据与前提声明。

## 2. 架构与边界

### 2.1 分层与新增文件

[INFERRED] 分层原则：`provider/sms/` 只放**平台无关契约 + 适配器 + 签名器**，不碰 DB、不碰站点配置。这样签名器能被纯函数级向量测试覆盖，不需要起服务、不需要连库。业务逻辑一律在 `provider/smsService.go`。

| 层 | 文件 | 职责 |
|---|---|---|
| 配置 | `config/sms.go` | `PluginSmsConfig` 与三家 region profile |
| 平台契约 | `provider/sms/sender.go` | `Sender` 接口、`SendRequest`／`SendResult`、场景码常量 |
| 平台适配 | `provider/sms/aliyun.go`、`tencent.go`、`twilio.go` | 请求构造、响应解析、错误映射 |
| 签名 | `provider/sms/sign_aliyun.go`、`sign_tencent.go` | ACS3-HMAC-SHA256 / TC3-HMAC-SHA256（Twilio 用 HTTP Basic，无签名器） |
| 错误分类 | `provider/sms/errors.go` | permanent／transient 二分 |
| 业务 | `provider/smsService.go` | 发码、校验、限流、落库、号码查重 |
| 前台 | `controller/apiSms.go` | 发码、短信登录、短信找回、换绑 |
| 后台 | `controller/manageController/pluginSms.go` | 配置读写、测试发送、发送记录 |
| 模型 | `model/sms.go` | `SmsCode`、`SmsLog` 两张表 |
| 意图 | `pkg/mcp/intent/catalog_domains.go` 等 | `channel_sms` 意图，详见 §9 |

改动的既有文件：`config/constant.go`（平台常量）、`provider/website.go`（挂载配置）、`provider/setting.go`（setting key 与加载）、`provider/dao.go`（迁移注册）、`provider/user.go`（区号感知查询）、`model/user.go`（区号列）、`request/user.go`（区号与验证码字段）、`controller/apiUser.go`（登录／注册分支）、`route/base.go`、`route/manage.go`、`config/menu.go`、`pkg/mcp/intent/domain.go`、`pkg/mcp/intent/mask.go`，以及 §8 列出的前端文件。

### 2.2 依赖预算

[COMPUTED] 方案 A 引入 **0 个新模块**。对照实测（隔离临时模块 + `go mod tidy` + `go list -deps`，不含仅测试依赖）：

| 方案 | 新增构建模块 |
|---|---|
| A：手写 HTTP + 对照向量 | **0** |
| B：腾讯云官方 SDK | +2 |
| B：阿里云官方 SDK | +9（含 `tjfoc/gmsm` 国密、`clbanning/mxj/v2` XML、`aliyun/credentials-go`） |

选手写不是因为「更轻」，而是因为仓库已有手写 AWS SigV4 的成熟范式（`provider/storage/aws.go:89-171`：`hmacSHA256` → `signingKey` 四级派生 → `sign` 组装 Authorization），签名自研在本仓不是新风险。真正的风险是「签错了没人知道」，由 §11.1 的对照向量闭环消除。

### 2.3 为什么不复用 IM 渠道子系统

[KNOWN] 现有 `provider/channelPlatform.go` 的接口是：

```go
type PlatformAdapter interface {
    Name() string
    Run(ctx context.Context, creds PlatformCredentials, sink InboundSink) error
    Send(ctx context.Context, target TargetRef, msg OutboundMessage) (SendResult, error)
}
```

`Run` 是长连接语义（飞书／钉钉的事件订阅），短信没有入站通道，实现它只能写一个空转的阻塞函数。`model.MessageChannel` 还携带 `AppIdentityHash`（全局唯一）、`ChannelLease`（`FencingVersion`）、`ChannelBinding`、`ChannelInbox`、`ChannelSession` 整套机械，对短信全是死重；且它的凭据模型是 AppId／AppSecret，与短信的 AK/SK 不同构。

**只借用一样东西**：`ClassifyPlatformSendError` 的 permanent／transient 错误二分契约（401/403 + 4xx → permanent；429/408/425 + 5xx → transient；HTTP 200 但业务码非 0 → 交平台分类器；net/url 错误 → transient；未知 → transient）。短信侧镜像同一套判据，见 §10.1。

### 2.4 通知预留的边界

[INFERRED] 只预留两样，不多建一张表：

1. **`Sender` 接口**：`Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error)`（签名以 §5.1 为准），平台无关。未来 `NotificationDelivery` 的执行器直接调它，**绕开 OTP 状态机**（通知不需要验证码、不需要一次性消费、不需要失败次数上限）。
2. **场景码命名空间**：本期占用 `login`／`register`／`reset`／`bind_old`／`bind_new` 五个；通知场景统一用 `notify.` 前缀，常量表里留注释说明命名规则，不预先定义具体通知场景。

不预留投递规则表、通知模板表、重试队列、租约。这些在真正做通知时按当时的需求建，提前建只会留下没人用的空表。

## 3. 配置模型

### 3.1 挂载与存储

[INFERRED] 完全沿用仓库既有配置范式，不发明新机制：

- `config/sms.go` 定义 `PluginSmsConfig`。
- `provider/website.go` 挂载 `PluginSms *config.PluginSmsConfig`（**指针**，与 `PluginSendmail` 一致）。
- `provider/setting.go` 增加 `SmsSettingKey = "sms"`，并在设置加载处调用 `w.LoadSmsSetting(settingMap[SmsSettingKey])`。
- `LoadSmsSetting` 先 `w.PluginSms = &config.PluginSmsConfig{}` 再 `json.Unmarshal`，与 `LoadSendmailSetting`（`provider/setting.go:439`）同形。
- 落库走 `w.SaveSettingValue(provider.SmsSettingKey, w.PluginSms)`，存 `settings` 表（主键列是 `key`，不是 `name`）。

### 3.2 结构定义

```go
// config/sms.go

const (
    SmsProviderAliyun  = "aliyun"
    SmsProviderTencent = "tencent"
    SmsProviderTwilio  = "twilio"

    SmsRegionDomestic      = "domestic"
    SmsRegionInternational = "international"
)

type PluginSmsConfig struct {
    Provider string `json:"provider"` // "" = 关闭；aliyun / tencent / twilio

    Aliyun  SmsAliyunConfig  `json:"aliyun"`
    Tencent SmsTencentConfig `json:"tencent"`
    Twilio  SmsTwilioConfig  `json:"twilio"`

    // 验证码参数（决策 15：全部后台可配）
    CodeLength         int `json:"code_length"`           // 4..8，默认 6
    CodeTTL            int `json:"code_ttl"`              // 秒，默认 300
    ResendInterval     int `json:"resend_interval"`       // 秒，默认 60
    MaxVerifyAttempts  int `json:"max_verify_attempts"`   // 默认 5
    DailyLimitPerPhone int `json:"daily_limit_per_phone"` // 默认 10
    HourlyLimitPerIp   int `json:"hourly_limit_per_ip"`   // 默认 10
    DailyLimitPerIp    int `json:"daily_limit_per_ip"`    // 默认 30
    LogRetentionDays   int `json:"log_retention_days"`    // 默认 90

    Scenes []string `json:"scenes"` // 启用的场景码，空 = 全部关闭
}

type SmsAliyunConfig struct {
    Domestic      SmsAliyunRegion `json:"domestic"`
    International SmsAliyunRegion `json:"international"`
}

type SmsAliyunRegion struct {
    AccessKeyId     string            `json:"access_key_id"`
    AccessKeySecret string            `json:"access_key_secret"`
    Endpoint        string            `json:"endpoint"`  // 空则用区域默认值
    SignName        string            `json:"sign_name"`
    ParamName       string            `json:"param_name"` // 模板变量名，空则默认 "code"，见 §3.6
    Templates       map[string]string `json:"templates"`  // 场景码 -> TemplateCode
}

type SmsTencentConfig struct {
    Domestic      SmsTencentRegion `json:"domestic"`
    International SmsTencentRegion `json:"international"`
}

type SmsTencentRegion struct {
    SecretId    string            `json:"secret_id"`
    SecretKey   string            `json:"secret_key"`
    SmsSdkAppId string            `json:"sms_sdk_app_id"`
    Region      string            `json:"region"`    // 空则 ap-guangzhou
    SignName    string            `json:"sign_name"`
    Templates   map[string]string `json:"templates"` // 场景码 -> TemplateId
}

// Twilio 只有国际 profile（决策 17）。
type SmsTwilioConfig struct {
    International SmsTwilioRegion `json:"international"`
}

type SmsTwilioRegion struct {
    // AccountSid 恒必填（它在 URL 路径里）。
    // Basic 凭据对二选一（决策 18）：(AccountSid, AuthToken) 或 (ApiKeySid, ApiKeySecret)；
    // 都填时优先 API Key，但路径里仍用 AccountSid。
    AccountSid   string `json:"account_sid"`
    AuthToken    string `json:"auth_token"`
    ApiKeySid    string `json:"api_key_sid"`
    ApiKeySecret string `json:"api_key_secret"`

    // 两种发件方二选一（决策 19）：都填时优先 MessagingServiceSid。
    From                string `json:"from"`
    MessagingServiceSid string `json:"messaging_service_sid"`

    Texts map[string]string `json:"texts"` // 场景码 -> 本地文案，占位符 {{code}}
}
```

区域默认 endpoint：阿里云国内 `dysmsapi.aliyuncs.com`、国际 `dysmsapi.ap-southeast-1.aliyuncs.com`；腾讯云统一 `sms.tencentcloudapi.com`；Twilio `https://api.twilio.com/2010-04-01/Accounts/{AccountSid}/Messages.json`。

**Twilio 不做 Content SID 映射。** 决策 20 的「平台模板映射」在 Twilio 侧落为本地文案（`Texts`），因为 Twilio 不预审模板。Content Template Builder 是独立能力，本期不引入，留作后续扩展点。

### 3.3 区域解析与凭据继承

[INFERRED] 唯一入口 `sms.Resolve(cfg *config.PluginSmsConfig, region, scene string) (*sms.Profile, error)`，实现 §1.1(a) 的口径。**必须带 `scene`**：模板是按场景配的，不带场景就无法判定「该场景模板为空」并提前拒发。

1. `Provider == ""` → `ErrSmsServiceNotEnabled`。
2. 按 `phone_country == "86"` 判定 region；`86` 为国内，其余（含 `852`／`853`／`886`）为国际。
3. `Provider == twilio && region == domestic` → `ErrSmsRegionUnsupported`，**明确拒发**，不静默降级。
4. 国际 profile 的**凭据字段**逐个判空，空则取国内同名字段；`SignName` 与 `Templates`／`Texts` **永不继承**。
5. 解析后仍缺必填凭据、缺签名、或该场景模板为空 → 对应错误，见 §10.2。

`sms.Profile` 是值类型快照，含生效凭据、签名、该场景模板 ID／文案、endpoint、region 名。适配器只吃它，不回头读站点配置——这样适配器可脱离 DB 测试。

**import 方向是硬约束**：`provider/sms` → `config`（单向），`provider` → 两者。因此解析函数**不能**写成 `config.PluginSmsConfig` 的方法——那会让 `config` 反向依赖 `provider/sms` 形成环。`config/sms.go` 只放结构与常量，不放行为。

### 3.4 掩码与回写保护

[KNOWN] 三件套缺一不可（仓库 2026-10-03 因指针浅拷贝踩过坑）：

**读时掩码。** `PluginSmsSetting` 必须先 `setting := *currentSite.PluginSms` **值拷贝**再脱敏。`PluginSms` 是指针，`setting := currentSite.PluginSms` 只复制指针，一次 `setting_get` 就会把内存里的真凭据抹成哨兵。嵌套的 region 结构体是值类型，随外层值拷贝一起复制，无需额外处理；但 `Templates`／`Texts` 是 map（引用类型），**脱敏不涉及它们，只读不改**，若将来要改必须显式深拷贝。

掩码字段（参与鉴权的全部字段，含 Basic auth 的用户名侧）：

| 服务商 | 掩码字段 |
|---|---|
| 阿里云 | `access_key_id`、`access_key_secret` |
| 腾讯云 | `secret_id`、`secret_key` |
| Twilio | `account_sid`、`auth_token`、`api_key_sid`、`api_key_secret` |

不掩码：`sms_sdk_app_id`、`region`、`endpoint`、`sign_name`、`from`、`messaging_service_sid`、`templates`、`texts`——它们不是凭据，掩掉会让站长无法核对配置。

哨兵常量 `manageController.SmsSecretMask = "********"`，与 `SendmailPasswordMask`、`request.MysqlPasswordMask`、`intent.MaskedSecret` 同串。

**写端点识别哨兵。** 逐字段判断：值等于哨兵则**沿用现值**，不等于则覆盖（显式传空串才算清空）。

**desc 说明。** MCP 意图与前端表单都要写明「`********` 表示未改动，原样写回即可保持不变」。

### 3.5 部分更新

[KNOWN] 配置 POST 走仓库标准范式，顺序不能变：

1. 保留具名 `ctx.ReadJSON(&req)`——`cmd/apicatalog` 的 AST 靠它生成参数 schema，换成第二次 `ReadJSON` 会让 params 归零。
2. `setting := *currentSite.PluginSms` 值拷贝作基底。
3. `json.Unmarshal(ctx.GetBody(), &payload)` 取 `map[string]json.RawMessage` 字段清单。
4. `library.MergeJSONPayload(payload, &setting)`（`library/merge.go:20`）按 json tag 只覆盖出现过的字段。
5. 逐字段掩码回写保护后赋值给 `currentSite.PluginSms`。
6. `SaveSettingValue` + `AddAdminLog`。

⚠️ `MergeJSONPayload` 只处理**顶层字段**。`aliyun`／`tencent`／`twilio` 是嵌套对象，整体出现在 payload 里就会整体替换。因此前端提交这三个对象时必须带**完整子树**（含未改动的掩码哨兵），不能只传改动的叶子字段。这一条要写进前端表单约定与 MCP desc。

⚠️ **`null` 的两种行为不同，必须区分**（`library/merge.go:40` 直接调 `json.Unmarshal`）：

| 提交形态 | 实际效果 |
|---|---|
| 顶层 `"aliyun": null` | **无效果**。Go 的 json 把 null 解进结构体值是 no-op，基底原值保留 |
| 嵌套 `"aliyun": {"templates": null}` | **清空**。null 解进 map 会把它设成 nil，该厂商所有场景模板丢失 |

所以前端**不得**提交 `templates: null`／`texts: null`：要么带完整 map，要么整个厂商对象不出现在 payload 里。这一条要有测试钉住，因为它是静默数据丢失，界面上看不出来。

### 3.6 阿里云模板变量名

[KNOWN] 阿里云 `TemplateParam` 是 JSON 字符串，键名必须与已审核模板里的变量名一致（常见为 `code`，但不保证）。腾讯云 `TemplateParamSet` 是**位置数组**，无变量名。因此阿里云 region 额外带一个字段：

```go
ParamName string `json:"param_name"` // 空则默认 "code"
```

腾讯云不需要该字段。Twilio 用本地文案的 `{{code}}` 占位符，变量名固定。

## 4. 数据模型

### 4.1 `users` 增加区号列

```go
PhoneCountry string `json:"phone_country" gorm:"column:phone_country;type:varchar(8);not null;default:'86'"`
```

唯一键语义 = `(phone_country, phone)` 二元组（决策 14）。`phone` 列本身**保持现有的非唯一索引 `idx_users_phone` 不变**——理由见 §4.4。

[COMPUTED] 开发库实测：`users` 共 339 行，其中 66 行 `phone = ''`，273 个非空号码互不重复。因此**不能**对 `phone` 加唯一索引：MySQL／MariaDB 没有部分索引，66 个空串会立刻撞唯一约束。这也是决策 9 选应用层查重的根本原因。

⚠️ 升级注意：AutoMigrate 加列时，存量 273 个非空号码一律回填 `86`。若库里已有国际号码，站长需自行订正区号，否则这些账号会走国内通道。这一条必须写进发布说明。

### 4.2 `sms_codes`：验证码状态机

```go
type SmsCode struct {
    Id           uint   `json:"id" gorm:"column:id;type:int unsigned;primaryKey;autoIncrement"`
    SiteId       uint   `json:"site_id" gorm:"column:site_id;type:int unsigned;not null;default:0;index:idx_site_phone_scene,priority:1"`
    PhoneCountry string `json:"phone_country" gorm:"column:phone_country;type:varchar(8);not null;default:'86';index:idx_site_phone_scene,priority:2"`
    Phone        string `json:"phone" gorm:"column:phone;type:varchar(32);not null;default:'';index:idx_site_phone_scene,priority:3"`
    Scene        string `json:"scene" gorm:"column:scene;type:varchar(32);not null;default:'';index:idx_site_phone_scene,priority:4"`

    CodeHash    string `json:"-" gorm:"column:code_hash;type:char(64);not null;default:''"`
    Attempts    int    `json:"attempts" gorm:"column:attempts;type:int;not null;default:0"`
    MaxAttempts int    `json:"max_attempts" gorm:"column:max_attempts;type:int;not null;default:5"`
    ExpiresAt   int64  `json:"expires_at" gorm:"column:expires_at;type:bigint;not null;default:0;index:idx_expires_at"`
    ConsumedAt  int64  `json:"consumed_at" gorm:"column:consumed_at;type:bigint;not null;default:0"`

    Provider   string `json:"provider" gorm:"column:provider;type:varchar(16);not null;default:''"`
    Region     string `json:"region" gorm:"column:region;type:varchar(16);not null;default:''"`
    BizId      string `json:"biz_id" gorm:"column:biz_id;type:varchar(64);not null;default:''"`
    SendStatus int    `json:"send_status" gorm:"column:send_status;type:tinyint;not null;default:0"` // 0 待发 1 成功 -1 失败
    LogId      uint   `json:"log_id" gorm:"column:log_id;type:int unsigned;not null;default:0"`

    CreatedTime int64 `json:"created_time" gorm:"column:created_time;type:bigint;not null;default:0"`
}
```

`CodeHash` 的 json tag 必须是 `-`：摘要不对外输出。存 `HMAC-SHA256(w.TokenSecret, code)` 的 64 位小写 hex，**不存明文**——库被读走不等于验证码被读走。

`MaxAttempts` 是发放时的**快照**，不是每次读配置：站长中途调小上限不应让已发出的码突然失效口径不一致。

**为什么不复用 `library.CodeCache`**（决策 2 的技术依据，`library/verifyCode.go` 共 112 行）：

| 缺陷 | 后果 |
|---|---|
| `Get` 从不检查 `Expire` | 过期码照样能通过校验 |
| `Generate` 在无锁状态下读写 map | 并发下数据竞争，可能 panic |
| `Verify` 在 key 不存在且 code 为空时返回 `true` | 空验证码绕过 |
| `GetByCode` 无锁遍历 map | 同上 |
| `Delete(code)` 收的是码不是 key，且 `size` 减错 | 清理逻辑失效 |
| 进程内存储 | 重启即丢，多实例不共享 |
| `math/rand` 生成 | 可预测 |

这套实现用于邮箱验证尚可容忍，用于**按条计费、可被刷的短信凭据**是明确的安全缺陷。新表用 `crypto/rand` + HMAC + DB 持久化 + 事务行锁，逐条对上。

### 4.3 `sms_logs`：发送流水

```go
type SmsLog struct {
    Id           uint   `json:"id" gorm:"column:id;type:int unsigned;primaryKey;autoIncrement"`
    SiteId       uint   `json:"site_id" gorm:"column:site_id;type:int unsigned;not null;default:0;index:idx_site_created,priority:1"`
    PhoneCountry string `json:"phone_country" gorm:"column:phone_country;type:varchar(8);not null;default:'86'"`
    Phone        string `json:"phone" gorm:"column:phone;type:varchar(32);not null;default:''"`
    Scene        string `json:"scene" gorm:"column:scene;type:varchar(32);not null;default:''"`
    Provider     string `json:"provider" gorm:"column:provider;type:varchar(16);not null;default:''"`
    Region       string `json:"region" gorm:"column:region;type:varchar(16);not null;default:''"`
    TemplateId   string `json:"template_id" gorm:"column:template_id;type:varchar(64);not null;default:''"`
    SignName     string `json:"sign_name" gorm:"column:sign_name;type:varchar(64);not null;default:''"`
    BizId        string `json:"biz_id" gorm:"column:biz_id;type:varchar(64);not null;default:''"`

    Status    int    `json:"status" gorm:"column:status;type:tinyint;not null;default:0"` // 0 pending 1 success -1 failed
    ErrCode   string `json:"err_code" gorm:"column:err_code;type:varchar(64);not null;default:''"`
    ErrMsg    string `json:"err_msg" gorm:"column:err_msg;type:varchar(500);not null;default:''"`
    Retryable int    `json:"retryable" gorm:"column:retryable;type:tinyint(1);not null;default:0"`
    CostMs    int    `json:"cost_ms" gorm:"column:cost_ms;type:int;not null;default:0"`

    Ip          string `json:"ip" gorm:"column:ip;type:varchar(64);not null;default:''"`
    UserId      uint   `json:"user_id" gorm:"column:user_id;type:int unsigned;not null;default:0"`
    CreatedTime int64  `json:"created_time" gorm:"column:created_time;type:bigint;not null;default:0;index:idx_site_created,priority:2"`
}
```

**不存验证码明文，也不存渲染后的正文。** 只存 `template_id` + `sign_name` + `scene`，足以反查发了什么。把含验证码的正文写进日志等于把凭据抄进一张查询权限更宽的表。

`idx_site_created` 列序必须是 `(site_id, created_time)`：后台查询恒为 `WHERE site_id=? ORDER BY created_time DESC`，列序反了站点过滤就用不上索引前缀。这一条要有 go/ast 护栏（复用 `channel_model_test.go` 的 `gormIndexPriority`）。

### 4.4 手机号唯一性：事务内应用层查重

[KNOWN] 现状是**裸的读后写**，没有任何事务：`provider/user.go:576` 查重、`:616` 才 `w.DB.Save(&user)`，中间隔着 40 行。并发下必然双写。

[COMPUTED] 开发库实测隔离级别与版本：

```
@@transaction_isolation = REPEATABLE-READ
@@version               = 11.7.2-MariaDB
```

**做法**：把「查重 + 插入用户」这两步收进一个事务，其余步骤（extra 字段、`ProcessNewUser`、发验证邮件）**留在事务外**——不能把 SMTP 发送圈进持锁事务。

```go
func (w *Website) createUserWithPhoneGuard(user *model.User) error {
    return w.DB.Transaction(func(tx *gorm.DB) error {
        if user.Phone != "" {
            var existing []model.User
            if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
                Where("phone = ?", user.Phone).Find(&existing).Error; err != nil {
                return err
            }
            for _, u := range existing {
                if u.PhoneCountry == user.PhoneCountry {
                    return errors.New(w.Tr("TheMobilePhoneNumberHasBeenRegistered"))
                }
            }
        }
        return tx.Save(user).Error
    })
}
```

`RegisterUser` 的 `w.DB.Save(&user)` 换成调它。

**三条必须遵守的约束：**

1. **WHERE 只用 `phone` 单列。** 写成 `WHERE phone_country=? AND phone=?` 会用不上 `idx_users_phone`，退化成全表扫描并锁全表。区号在应用层比对（同号跨国最多几行，代价可忽略）。
2. **`phone == ""` 直接跳过查重。** 66 行空串若参与 `FOR UPDATE` 会锁住一大片间隙，把无关注册全部串行化。
3. **依赖 REPEATABLE READ。** RR 下 InnoDB 对非唯一二级索引的 `FOR UPDATE` 取 next-key lock，即使没命中行也锁住间隙，能挡住并发插同号。**若生产库是 READ COMMITTED，这个保证不成立**（无间隙锁，两个事务都查不到、都插入）。上线前必须核对生产库隔离级别；若是 RC，需另行决策（改隔离级别，或引入独立唯一映射表），**不得默认已解决**。

[INFERRED] 用户名与邮箱的查重仍是事务外的裸读后写，属既有缺陷，本期**不顺手修**——它与短信无关，改它会扩大爆炸半径。规格记录此事，不假装已解决。

### 4.5 表增长与清理

[INFERRED] 两张表的增长策略不同，这也是为什么必须分两张：

- **`sms_codes` 只保留活跃码。** 发码事务内先删除同 `(site_id, phone_country, phone, scene)` 的已过期或已消费行，再插入新行。表大小天然有界 ≈ 活跃号码数 × 场景数，不需要定时任务。
- **`sms_logs` 是 append-only，所有限额计数都查它。** 不能查 `sms_codes`——那里的行会被删，日限额会随之失效。清理按 `LogRetentionDays`（默认 90 天）惰性执行：发码时顺带删除本站点超期日志，**单次限量 1000 行**，避免长事务。

### 4.6 迁移注册

两张新表必须加进 `provider/dao.go` 的 `AutoMigrateDB` models 切片。`AutoMigrateDB(db, force=false)` 只在 `lastVersion != config.Version` 时才真迁移，所以老库需后台「修复数据库」或随版本号提升触发。

护栏复用 `provider/channel_model_test.go` 的 `scanMigratedModels`（go/ast 解析，带「扫描器失效」自检与合成源码反向对照），新增 `smsModels` 清单与 `TestSmsTablesRegistered`。**不写死模型总数。**

## 5. 平台适配与签名

### 5.1 Sender 契约

```go
// provider/sms/sender.go

type Profile struct {
    Provider string
    Region   string
    Endpoint string

    // 阿里云 / 腾讯云
    AccessKeyId     string
    AccessKeySecret string
    SecretId        string
    SecretKey       string
    SmsSdkAppId     string
    CloudRegion     string // 腾讯云 X-TC-Region
    SignName        string
    TemplateId      string
    ParamName       string

    // Twilio
    AccountSid          string
    AuthToken           string
    ApiKeySid           string
    ApiKeySecret        string
    From                string
    MessagingServiceSid string
}

type SendRequest struct {
    To    string // E.164，必须带 +，如 +8613900001111
    Scene string
    Code  string // 验证码，唯一模板变量
    Text  string // Twilio 本地文案（已渲染，含验证码）
    OutId string // 关联 SmsLog.Id，透传给平台做对账
}

type SendResult struct {
    BizId   string // 阿里云 BizId / 腾讯云 SerialNo / Twilio sid
    RawCode string // 平台业务码原值
    RawMsg  string
}

type Sender interface {
    Name() string
    Send(ctx context.Context, p Profile, req SendRequest) (SendResult, error)
}
```

`Profile` 用扁平值类型而不是三个嵌套结构：适配器只读自己关心的字段，测试构造成本低，且**不含任何 DB 或站点依赖**。

`Send` 的错误一律包成 `*sms.SendError`，带 `Class`（permanent／transient）、`Code`、`Message`。业务失败**不得**返回 `nil` error——HTTP 200 但业务码非成功也是失败。

### 5.2 阿里云

[KNOWN] 已核验的接口事实（来源见 §13）：

| 项 | 值 |
|---|---|
| Action / Version | `SendSms` / `2017-05-25` |
| 国内 endpoint | `dysmsapi.aliyuncs.com` |
| 国际 endpoint | 按地域，新加坡 `dysmsapi.ap-southeast-1.aliyuncs.com`（另有 `ap-southeast-5`、`eu-central-1`） |
| 请求参数 | `PhoneNumbers`、`SignName`、`TemplateCode`、`TemplateParam`、`OutId`、`SmsUpExtendCode` |
| 返回参数 | `Code`、`Message`、`BizId`、`RequestId` |
| 成功判据 | `Code == "OK"` |

签名 ACS3-HMAC-SHA256（V3）。请求头需带 `x-acs-action`、`x-acs-version`、`x-acs-date`、`x-acs-signature-nonce`、`x-acs-content-sha256`，`Authorization` 形如 `ACS3-HMAC-SHA256 Credential={AccessKeyId},SignedHeaders={...},Signature={...}`。

`TemplateParam` 是 JSON 字符串，键名取 `Profile.ParamName`（默认 `code`）：`{"code":"123456"}`。`OutId` 填 `SmsLog.Id`。

国际 endpoint 做成**可配置**（`SmsAliyunRegion.Endpoint`），默认新加坡——站长可能开在别的地域，写死会让他配不出来。

### 5.3 腾讯云

[KNOWN] 已核验的接口事实：

| 项 | 值 |
|---|---|
| Action / Version | `SendSms` / `2021-01-11` |
| Endpoint | `sms.tencentcloudapi.com` |
| 请求体必填 | `PhoneNumberSet`（Array of String）、`SmsSdkAppId`（String）、`TemplateId`（String） |
| 请求体可选 | `SignName`（**国内必填**，是签名内容不是 ID）、`TemplateParamSet`（Array of String，位置对应）、`SessionContext`（<512 字节，原样回传）、`ExtendCode`、`SenderId`（仅国际／港澳台且已申请独立 SenderID） |
| 返回 | `Response.SendStatusSet[]`：`SerialNo`、`PhoneNumber`、`IsoCode`、`Code`、`Message`、`Fee`、`SessionContext`；外层 `Response.RequestId` |
| 成功判据 | `SendStatusSet[0].Code == "Ok"`（注意大小写与阿里云不同） |

签名 TC3-HMAC-SHA256，POST JSON。请求头 `X-TC-Action`、`X-TC-Version`、`X-TC-Region`、`X-TC-Timestamp`，`Authorization` 形如 `TC3-HMAC-SHA256 Credential={SecretId}/{Date}/sms/tc3_request, SignedHeaders=content-type;host, Signature={...}`。

**号码格式**：`PhoneNumberSet` 用 E.164 `+[国家码][号码]`。本设计一次只发一个号码，因此不会触发「单次请求不能混发国内与国际」的平台限制（该限制上限 200 个号码）。

`SignName` 校验口径：国内 profile 必填；国际 profile 允许为空（平台侧国际短信可用 SenderId 或默认签名）。`SessionContext` 填 `SmsLog.Id`，`TemplateParamSet` 为 `[]string{code}`。

### 5.4 Twilio

[KNOWN] 已核验的接口事实：

| 项 | 值 |
|---|---|
| 方法 / URL | `POST https://api.twilio.com/2010-04-01/Accounts/{AccountSid}/Messages.json` |
| 鉴权 | HTTP Basic |
| Content-Type | `application/x-www-form-urlencoded` |
| 必填参数 | `To` |
| 可选参数 | `From`、`MessagingServiceSid`、`Body`、`StatusCallback`、`ContentSid` |
| 响应字段 | `sid`、`status`、`error_code`、`error_message`、`num_segments`、`price` |
| 失败状态 | `failed`、`undelivered` |

**无签名器**——Basic auth 就是全部鉴权。

⚠️ **`AccountSid` 恒必填**，因为它在 URL 路径里。所谓「两种鉴权二选一」指的是 Basic 的凭据对：`(AccountSid, AuthToken)` 或 `(ApiKeySid, ApiKeySecret)`；用 API Key 时路径里仍然是 Account SID。两者都填时优先 API Key。

`From` 与 `MessagingServiceSid` 二选一，都填时优先 `MessagingServiceSid`；两者都空则拒绝发送（平台会报错，但提前拒能给出可读原因）。

`Body` 取 `SendRequest.Text`（本地文案渲染结果）。本期**不使用** `ContentSid` 与 `StatusCallback`。

Twilio 到中国大陆的到达率不可靠，这是它只用于国际号码的原因（决策 17）。`+86` 号码在 `sms.Resolve` 阶段就被拒，不会走到适配器。

### 5.5 签名器实现要点

[INFERRED] 参照 `provider/storage/aws.go` 的既有范式：`hmacSHA256` 纯函数 + 密钥派生链 + `sign` 组装 Authorization。三家共性约束：

1. **时间可注入。** 签名器接受 `now time.Time` 参数（或内部 `clock` 字段），否则向量测试无法固定时间。
2. **随机 nonce 可注入。** 阿里云 `x-acs-signature-nonce` 同理。
3. **纯函数，无 IO。** 签名器不发请求、不读配置、不碰 DB。HTTP 调用在适配器里。
4. **编码规则逐字对齐。** 阿里云 V3 的 canonical query 要求 RFC3986 编码（空格编 `%20` 不是 `+`，`*` 编 `%2A`，`~` 不编）；腾讯云 V3 的 canonical request 用 `content-type;host` 两个头 + body 的 SHA256。这两处是最容易签错的地方，必须由 §11.1 的对照向量锁死。
5. **不复用 AWS SigV4 代码。** 三家算法相似但细节不同（派生链层数、credential scope 字符串、header 集合），共用一份实现会把差异藏进 if 分支。各自独立文件，各自向量。

### 5.6 平台错误码映射

[INFERRED] 每个适配器维护一张**已知错误码 → 分类 + 中文提示**的表，未列入的一律按 §10.1 的通用判据兜底为 transient。已知 permanent 类至少覆盖：

| 平台 | 典型 permanent 错误 |
|---|---|
| 阿里云 | 签名不存在／未审核、模板不存在／未审核、AK 无效或无权限、账户余额不足、号码格式非法、触发平台级流控 |
| 腾讯云 | 签名未审核、模板未审核、SecretId 无效、SmsSdkAppId 不存在、余额不足、号码格式非法 |
| Twilio | 20003（鉴权失败）、21211（To 非法）、21606（From 非法）、21610（收件人已退订）、余额不足 |

⚠️ 错误码表**不写死总数断言**，但每条映射要有单测。未识别的码必须原样落进 `sms_logs.err_code`，不能被吞成「发送失败」——那是排查时唯一的线索。

## 6. 验证码状态机

### 6.1 场景码

| 场景码 | 用途 | 消费方 |
|---|---|---|
| `login` | 手机号验证码登录（未注册则自动注册） | `POST /api/login`，`platform=sms` |
| `register` | 注册时校验手机号 | `POST /api/register` |
| `reset` | 短信找回密码 | `POST /api/sms/password/reset` |
| `bind_old` | 换绑：验证旧号 | `POST /api/sms/phone/bind` |
| `bind_new` | 换绑：验证新号 | `POST /api/sms/phone/bind` |

通知场景统一用 `notify.` 前缀，本期不定义具体值（§2.4）。

另有后台专用的 `test` 场景（§7.2）：它**不在** `Scenes` 白名单里、前台不可请求，只由 `POST /plugin/sms/test` 内部使用，因此不受「场景未开启」限制，但仍受 IP 与号码限额约束。

**场景码严格隔离**：`login` 的码不能用于 `reset`。校验查询带 `scene` 条件，跨场景重放直接落「验证码不存在」。

### 6.2 号码归一化与校验

[INFERRED] 纯函数放 `provider/sms/phone.go`，不碰 DB：

1. 去格式字符：空格、横线、括号、点。
2. 区号去前导 `+` 与 `00`，校验 `^[1-9]\d{0,3}$`。空则默认 `86`。
3. 本地号码校验 `^\d{4,14}$`。**不去前导 0**——意大利等国的号码前导 0 是有效组成部分，去掉会发错人。
4. 组合后 E.164 数字总长 ≤ 15。
5. 区号为 `86` 时额外校验 `^1[3-9]\d{9}$`（**带锚点的新函数**）。
6. 区号必须属于 `provider/sms/country_codes.go` 的 ITU 已分配地理区号表（206 条）。该表**前缀无关**——没有任何一个码是另一个码的真前缀，因此同一串 E.164 的切分唯一，`(phone_country, phone)` 键不会被切分放大。这是实现期追加的第 6 道闸门，上面五条规则与三条正则不变；前缀无关不变量由 `TestCountryCodesPrefixFree` 穷举锁住，**不要当多余校验删掉**：删掉它，换一种区号切分就能乘出多份号码日限与重发冷却预算，`Region()` 还会把大陆号误判成 international 从而绕过规则 3 的 Twilio 拒发。

⚠️ **不改 `VerifyCellphoneFormat`**（`provider/user.go:963`）。它的正则 `1[3-9][0-9]{9}` 无 `^$`，`"abc13900001111xyz"` 也能通过；收紧会让存量带前缀账号无法登录。新校验只用于短信路径。

存储：`phone_country` 存归一化区号（纯数字），`phone` 存归一化本地号码。发送时拼 `+{country}{phone}`。

### 6.3 发码流程

```
 1. Provider 非空？                        否 → ErrSmsServiceNotEnabled
 2. scene 在 Scenes 白名单？               否 → ErrSmsSceneNotEnabled（后台 test 场景跳过此步）
 3. 号码归一化 + 格式校验                  失败 → ErrSmsPhoneInvalid
 4. Safe.Captcha == 1 时校验图形验证码      失败 → 图形验证码错误
 5. IP 滑窗（小时 / 日）                   超限 → ErrSmsIpLimited
 6. sms.Resolve(cfg, region, scene)        Twilio+86 / 缺凭据 / 缺签名 / 场景模板空 → 对应错误
 7. DB 事务（不做任何网络 IO）：
      a. sms_logs 当日同号码计数 ≥ DailyLimitPerPhone → ErrSmsPhoneLimited（**跨场景合计**，不带 scene 条件）
      b. sms_codes 最新活跃行 CreatedTime + ResendInterval > now → ErrSmsResendTooSoon（带剩余秒数，**按场景**）
      c. 删除同 (site, country, phone, scene) 的已过期 / 已消费行
      d. crypto/rand 生成码 → HMAC → 插入 sms_codes(send_status=0)
      e. 插入 sms_logs(status=0 pending)
      f. 回写 sms_codes.log_id
 8. 事务外调 Sender.Send（超时 10s）
 9. 回写 sms_logs（status / biz_id / err_code / err_msg / retryable / cost_ms）
    回写 sms_codes（send_status / biz_id）
10. 平台失败 → send_status = -1，该码作废
11. 惰性清理超期日志（单次 ≤ 1000 行）
```

**第 7 步与第 8 步必须分开**：持锁事务里做网络 IO 会把行锁按住数秒，同一号码的并发请求全部串行化，平台一慢就雪崩。

**第 10 步的理由**：发送失败的码若仍可用，用户会拿着一个永远收不到的验证码干等，还会反复点重发。作废后重发间隔仍生效（防刷），但提示语明确告知发送失败。

返回 `{expires_in, resend_after}`，**绝不返回验证码本身**。

### 6.4 校验流程

```
1. 号码归一化
2. 事务 + 行锁取最新可用行：
   WHERE site_id=? AND phone_country=? AND phone=? AND scene=? AND consumed_at=0
   ORDER BY id DESC LIMIT 1 FOR UPDATE
3. 无行            → ErrSmsCodeNotFound
4. ExpiresAt < now → ErrSmsCodeExpired
5. SendStatus != 1 → ErrSmsSendFailed
6. Attempts >= MaxAttempts → ErrSmsCodeLocked
7. subtle.ConstantTimeCompare(HMAC(输入码), CodeHash)
   不符 → Attempts++ 写回 → ErrSmsCodeMismatch（带剩余次数）
8. 符合 → ConsumedAt = now 写回 → 成功
```

一次性由第 2 步的 `consumed_at=0` 保证：消费过的行查不出来，重放落 `ErrSmsCodeNotFound`。行锁保证同一码不会被两个并发请求同时消费。

`Attempts++` 必须**写回数据库**，不能只在内存里加——否则攻击者每次新开请求就重置计数。

### 6.5 自动注册语义（login 场景）

[INFERRED] 决策 1 要求「自动注册并登录」。实现复用 `RegisterUser`，不另写一套建号逻辑：

1. `GetUserInfoByPhoneCountry(country, phone)` 查号。
2. **已存在** → 检查 `Status`（1 正常 / 0 待审核 / -1 禁止）→ 正常则发 token。
3. **不存在** → 合成 `request.ApiRegisterRequest{UserName: 随机 12 位, Password: 随机 32 位, Phone, PhoneCountry}` → 调 `w.RegisterUser(req)`，从而复用 extra 字段、`ProcessNewUser`、默认用户组与默认状态。

两个合成字段的口径与理由：

- **UserName 用随机串，不用手机号。** 用户名在部分模板里会公开显示，用手机号等于把号码挂到页面上。撞名重试至多 3 次。代价是这类用户不能用用户名登录，只能用手机号登录，或到资料页补全用户名——这个代价可接受，且必须在发布说明里写明。
- **Password 用随机 32 位，用户不可知。** 后续通过「短信找回密码」设置真实密码。

⚠️ **不得绕过站点审核策略**：若 `PluginUser.GetDefaultStatus()` 返回 0（待审核），自动注册后必须拒绝登录并提示待审核，与邮箱注册路径一致。

### 6.6 既有查询函数的区号改造

`GetUserInfoByPhone(phone string)`（`provider/user.go:95`）改为带区号参数，3 处调用点同步改：

| 位置 | 改法 |
|---|---|
| `provider/user.go:576`（RegisterUser 查重） | 由 §4.4 的事务查重取代 |
| `provider/user.go:1048`（资料更新查重） | 传 `req.PhoneCountry`，未传则沿用用户现值 |
| `provider/order.go:1347`（订单关联用户） | 传 `user.PhoneCountry` |

`request` 侧资料更新请求补 `PhoneCountry *string`（**指针**，未传不改），与既有 `Phone *string` 一致。否则用户编辑资料会把区号静默重置成 `86`。

## 7. API 契约

### 7.1 前台端点

[INFERRED] 全部挂在 `route/base.go` 的 `/api` party（已有 `middleware.ParseUserToken`）。响应统一 `{code, msg, data}`，`code` 用 `config.StatusOK`（值为 0）／`config.StatusFailed`（值为 -1）。仓库没有 `StatusSuccess` 常量，前后台都用 `StatusOK`。

#### `POST /api/sms/send`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `scene` | string | 是 | `login`／`register`／`reset`／`bind_old`／`bind_new` |
| `phone_country` | string | 否 | 区号，纯数字，默认 `86` |
| `phone` | string | 是 | 本地号码 |
| `captcha_id` | string | 条件 | `Safe.Captcha == 1` 时必填 |
| `captcha` | string | 条件 | 同上 |

成功 `data`：`{expires_in: 300, resend_after: 60}`。

`bind_old` 场景要求登录态，且号码必须等于当前用户绑定的号码——否则可以给任意号码发码，变成免费的短信轰炸跳板。

#### `POST /api/login`（扩展，不新增端点）

沿用既有 `platform` 分派范式（weapp／wechat／google 之后加 `sms` 分支）：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `platform` | string | 是 | 固定 `sms` |
| `phone_country` | string | 否 | 默认 `86` |
| `phone` | string | 是 | 本地号码 |
| `code` | string | 是 | 短信验证码 |

成功 `data` 与既有密码登录同形（含 `token`）。新增平台常量 `config.PlatformSms = "sms"`。

⚠️ `request.ApiLoginRequest.Code` 的注释是「微信临时凭证或验证码」，`ApiRegisterRequest.Code` 的注释是 `//phone verify code` 但**实际只被微信登录消费**。这两处注释是误导性遗留，实现时一并订正。

#### `POST /api/register`（扩展）

新增可选 `phone_country`；`code` 字段已存在，本期真正启用。规则（决策 11）：

- 传了 `phone` → **强制**校验 `code`（scene = `register`），失败则整个注册失败。
- 没传 `phone` → 行为与现在**完全一致**，不校验 `code`。

#### `POST /api/sms/password/reset`

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `phone_country` | string | 否 | 默认 `86` |
| `phone` | string | 是 | 本地号码 |
| `code` | string | 是 | 短信验证码（scene = `reset`） |
| `password` | string | 是 | 新密码，≥ 6 位 |

独立端点，**不去搅**既有 `POST /api/password/reset` 的 email-token 逻辑（它用 `ResetPasswordToken` + `subtle.ConstantTimeCompare`，与短信码是两套凭据）。

未注册号码：发码阶段返回统一文案「若该号码已注册，验证码已发送」，不泄露注册状态；重置阶段因查不到用户而失败，提示语与验证码错误一致，不做区分。

#### `POST /api/sms/phone/bind`

要求登录态。

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `old_code` | string | 是 | 旧号验证码（scene = `bind_old`） |
| `new_phone_country` | string | 否 | 默认 `86` |
| `new_phone` | string | 是 | 新号码 |
| `new_code` | string | 是 | 新号验证码（scene = `bind_new`） |

**旧号新号都验**（决策 12），两个码在**同一事务**内消费，任一失败则整体回滚。新号唯一性走 §4.4 的查重（排除自己）。

当前用户未绑定手机号时（`phone == ''`），跳过 `bind_old` 校验，只验新号——否则永远绑不上第一个号码。这是唯一的例外，必须在测试里显式覆盖。

### 7.2 后台端点

[INFERRED] 挂在 `route/manage.go` 的 `plugin` party，用 **`AdminPermissionRequired`** 而非 `AdminPermission`。理由与 `/channels` 一致（`route/manage.go:724-749` 的注释）：后台历史行为对「不在菜单里的路径」是放行的，而这些端点能改平台凭据，必须显式授权。

| 端点 | 说明 |
|---|---|
| `GET /plugin/sms/setting` | 读配置，凭据掩码 |
| `POST /plugin/sms/setting` | 部分更新 + 掩码回写保护 + `AddAdminLog` |
| `POST /plugin/sms/test` | 向指定号码发测试短信，走完整发码链路但 scene 固定为 `test` |
| `GET /plugin/sms/logs` | 分页发送记录，支持按 `scene`／`provider`／`status`／号码／时间区间筛选 |

`test` 场景**不占用**五个业务场景码，也**不受** `Scenes` 白名单限制（否则站长关掉全部场景就没法测试了）。但它**仍受** IP 与号码限额约束，且需要后台管理员权限——它是一条真实的付费发送通道。

`test` 的模板取该 region 下 `login` 场景的模板（站长通常先申请验证码模板）；若 `login` 模板为空则明确报错，不静默套用别的场景。

改完 `route/manage.go` **必须**跑 `go run ./cmd/apicatalog`，否则 `api_invoke` 报「未找到端点」。

## 8. 后台前端

### 8.1 文件清单

| 文件 | 动作 |
|---|---|
| `anqicms-admin/config/routes.ts` | 在 `/plugin/channel` 的 routes 里新增一项 |
| `anqicms-admin/src/services/plugin/sms.ts` | 新建，仿 `sendmail.ts` 的 `get`／`post` 封装 |
| `anqicms-admin/src/pages/plugin/sms/index.tsx` | 新建 |
| `anqicms-admin/src/locales/zh-CN/plugin.ts` | 新增 `plugin.sms.*` |
| `anqicms-admin/src/locales/en-US/plugin.ts` | 同上 |
| `anqicms-admin/src/locales/zh-TW/plugin.ts` | 同上 |
| `config/menu.go` | 新增菜单项（后端文件，但与前端路由强耦合，同批改） |

路由项：

```ts
{
  // path 必须与后端 config/menu.go 的 Menu.Path 逐字一致，
  // 它同时是管理员分组权限的 KEY。
  path: '/plugin/channel/sms',
  name: 'sms',
  type: 'system',
  component: '@/pages/plugin/sms/index',
  icon: 'sendmail',
},
```

菜单项（插在 `/plugin/channel/sendmail` 之后）：

```go
{
    Path:     "/plugin/channel/sms",
    GroupKey: "plugin",
    Name:     "短信服务",
    Backend:  "/plugin/sms",
},
```

⚠️ `DefaultMenuGroups` 只在首次安装／重建菜单时写库，**已运行站点需触发菜单重新同步**，否则新页面进不去。这一条要写进发布说明。

### 8.2 页面结构

[INFERRED] 沿用 `pages/plugin/channels/index.tsx` 的 **visitedTabs 常驻挂载 + `display:none` 切换**范式，不用条件渲染——条件渲染会在切 tab 时卸载表单、丢掉未保存输入。

```tsx
const [visitedTabs, setVisitedTabs] = useState<string[]>(['basic']);
const renderTab = (key: string, node: React.ReactNode) => {
  if (!visitedTabs.includes(key)) return null;
  return <div style={{ display: activeTabKey === key ? 'block' : 'none' }}>{node}</div>;
};
```

| Tab | 内容 |
|---|---|
| 基础设置 | 生效厂商单选（关闭／阿里云／腾讯云／Twilio）、验证码参数、场景开关 |
| 阿里云 | 国内／国际两组：凭据、endpoint、签名、各场景模板 ID、模板变量名 |
| 腾讯云 | 国内／国际两组：凭据、SmsSdkAppId、地域、签名、各场景模板 ID |
| Twilio | 仅国际一组：AccountSid、两种凭据对、两种发件方、各场景本地文案 |
| 发送测试 | 选区号 + 输号码 + 发送，实时显示结果与失败原因 |
| 发送记录 | 分页表格 + 筛选，失败行展开显示 `err_code`／`err_msg` |

厂商 tab **始终可见**（不因未选中而隐藏）：站长常需要先把三家的配置都填好再切换生效厂商，隐藏会让他每次切换都丢上下文。未生效厂商的 tab 顶部标注「当前未生效」。

Twilio tab 顶部固定提示：**仅支持国际号码，`+86` 号码将被拒发**。

### 8.3 表单约定

1. **配置类 tab 共用一个 ProForm 实例**，提交用 `formRef.current?.getFieldsValue(true)` 取全量。虽然后端支持部分更新，但仓库既有 tab 式设置页的约定是提交全量（`setting/system|content|safe|contact` 是整包覆盖），保持一致避免后人误判。
2. **三个厂商对象必须带完整子树提交**（§3.5）：`MergeJSONPayload` 只处理顶层字段，只传改动的叶子字段会让整个厂商对象被替换成残缺失效配置。
3. **凭据字段 placeholder 写明掩码语义**：「`********` 表示未改动，原样保存即可保持不变」。读回掩码后不要清空输入框。
4. **场景模板留空的语义要在界面上说清**：留空 = 该场景不可用，发码时会明确报错。不要让人以为留空会走默认模板。
5. 区号输入用带默认值 `86` 的文本框 + 常用区号下拉，**不做成必填的纯手填**——绝大多数站长只用国内。
6. 保存成功后 `message.success` 并重新拉取配置，让掩码状态与后端一致。

### 8.4 前端验证约束

[KNOWN] 本沙箱的既有约束，实现时必须遵守：

- `npx tsc --noEmit` 全量会被 SIGTERM（umi 类型体系过重）。小改动用 `./node_modules/.bin/esbuild` 做语法校验。
- 仓库已有 **38 个存量类型错误**（PlaceCascader／bizcharts／CopyToClipboard 等）。判断「是否引入新错误」要看是否命中改动文件，**不能看总数**。
- **不要跑 `npm run format`**——它会动全仓，为几个文件跑它是纯噪声。

## 9. MCP 意图接入

### 9.1 新增 `channel_sms` 意图

[INFERRED] 仿 `channel_sendmail`（`pkg/mcp/intent/catalog_domains.go:756-788`）：

```go
{
    Name: "channel_sms", Title: "短信服务", Domain: DomainChannel, Risk: RiskWrite,
    Desc: "短信渠道配置、测试发送与发送记录。action: setting_get/setting_save/test/logs。" +
        "返回统一为标准信封 {code,msg,ok,status,data}。" +
        "⚠️ setting_save 的顶层字段是部分更新，但 aliyun/tencent/twilio 三个对象是**整体替换**：" +
        "传它们时必须带完整子树，只传改动的叶子字段会清掉其余配置。" +
        "⚠️ setting_get 返回的凭据是掩码 ********，表示「未改动」；原样写回即可保持不变。" +
        "⚠️ 同一时刻只有一家厂商生效；Twilio 只支持国际号码，+86 会被拒发。",
    DefaultOff: true,
    Params: map[string]ParamSpec{
        "action": {Type: "string", Required: true, Enum: []string{
            "setting_get", "setting_save", "test", "logs"}},
        "values":     {Type: "object", Desc: "setting_save 传完整配置；test 传 {phone_country, phone}"},
        "page":       {Type: "integer", Default: 1},
        "page_size":  {Type: "integer", Default: 20},
        "scene":      {Type: "string", Desc: "logs 筛选：场景码"},
        "provider":   {Type: "string", Desc: "logs 筛选：厂商"},
        "status":     {Type: "integer", Desc: "logs 筛选：0 待发 1 成功 -1 失败"},
        "phone":      {Type: "string", Desc: "logs 筛选：号码"},
        "start_time": {Type: "integer", Desc: "logs 筛选：起始时间戳"},
        "end_time":   {Type: "integer", Desc: "logs 筛选：结束时间戳"},
    },
    Required: []string{"action"},
    Caps:     []string{"api_invoke"},
    Compose: invokeRoutes("channel_sms", map[string]string{
        "setting_get":  "GET /plugin/sms/setting",
        "setting_save": "POST /plugin/sms/setting",
        "test":         "POST /plugin/sms/test",
        "logs":         "GET /plugin/sms/logs",
    }),
}
```

`ParamSpec` 的类型必须与端点请求结构体的 json tag **严格一致**。加 cap 时逐个核对「handler 有、schema 无」的参数——漏了不报错，只是值被静默清空。

### 9.2 DefaultOff 的四处平行名单

[KNOWN] 这是仓库明确记录的地雷：改 `DefaultOff` 会牵动 4 处独立维护的名单，漏一处就红。`channel_sms` 设为 `DefaultOff: true`，必须同步：

1. `intent.gatedDomainIntents` —— 加入 `channel_sms`。
2. 测试内的 `intent.defaultOpenDomainIntents` —— **不得**包含它。
3. `RecommendedExposed()` —— **不得**包含 DefaultOff 意图。
4. `provider/api_intent_routes_test.go` 的 `mustStayGated`（`:519`）—— 加入并写明理由；`dailyOpen`（`:573`）**不得**包含它。

另需在 `pkg/mcp/intent/domain.go` 的 `domainByPlugin` 补 `"sms": DomainChannel`（紧邻 `:62` 的 `"sendmail"`），以及 `catalog_domains.go:1088` 附近的风险描述映射补一条。

意图总数 53 → 54。新增意图 DefaultOff，**默认工具面常驻 token 成本不变**。

### 9.3 掩码名单补缺

[KNOWN] `pkg/mcp/intent/mask.go` 的 `IsSecretField` 现有精确名单含 `token`／`token_secret`，后缀规则含 `_password`／`_secret`／`_access_key`／`_api_key`。据此核对本次新增字段：

| 字段 | 现状 | 动作 |
|---|---|---|
| `access_key_id` / `access_key_secret` | 已覆盖 | 无 |
| `secret_id` / `secret_key` | 已覆盖 | 无 |
| `api_key_secret` | `_secret` 后缀已覆盖 | 无 |
| **`auth_token`** | **漏网**（`_token` 不是匹配后缀） | 补精确名 `auth_token` + 后缀 `_auth_token` |
| **`account_sid`** | **漏网** | 补精确名 `account_sid` |
| **`api_key_sid`** | **漏网** | 补精确名 `api_key_sid` |

⚠️ **不要**加宽泛的 `_token` 后缀——那会把 `token_expire`／`token_type` 这类普通字段也打成掩码，反而让调用方误判配置状态。只补确凿是凭据的名字。

`account_sid` 与 `api_key_sid` 纳入掩码的理由：它们是 Basic auth 的用户名侧，与密码侧成对构成完整凭据。仓库已把 `access_key_id`／`secret_id` 当凭据处理，口径一致。

### 9.4 失败语义

[KNOWN] 业务失败必须置 `isError=true`（SDK 是 `omitempty`，不置会伪装成成功）。

⚠️ 工具一旦返回 `data`，端点那句 `msg` 会被出口吞掉（`envelopeOf` 把非信封 map 的 `msg` 写死成 `""`）。出口已用 `mergeEndpointMsg` 只补空位。**新增 Compose 时别指望 envelope 里的 msg**，需要提示语就自己写进 `Data`。

## 10. 错误处理与防刷

### 10.1 错误分类

[INFERRED] 镜像 `ClassifyPlatformSendError` 的判据，落在 `provider/sms/errors.go`：

| 情形 | 分类 |
|---|---|
| HTTP 401 / 403，及其它 4xx | permanent |
| HTTP 429 / 408 / 425，及 5xx | transient |
| HTTP 200 但业务码非成功 | 交平台错误码表（§5.6）；未列入则 transient |
| net / url 错误、超时 | transient |
| 未知 | transient |

**发码接口不自动重试。** 短信按条计费，自动重试等于重复扣费。transient 只在 `sms_logs.retryable` 打标，由站长在后台看到后自行决定是否重发。

### 10.2 业务错误清单

[INFERRED] 全部走 `w.Tr(...)` 多语言，不硬编码中文。至少覆盖：

| 错误 | 触发条件 | 对外提示口径 |
|---|---|---|
| `ErrSmsServiceNotEnabled` | `Provider == ""` | 短信服务未启用 |
| `ErrSmsSceneNotEnabled` | 场景不在 `Scenes` | 该场景未开启 |
| `ErrSmsPhoneInvalid` | 归一化或格式校验失败 | 手机号格式不正确 |
| `ErrSmsRegionUnsupported` | Twilio + `+86` | 当前短信服务商不支持中国大陆号码 |
| `ErrSmsCredentialMissing` | 解析后仍缺必填凭据 | 短信服务未配置完整 |
| `ErrSmsSignMissing` | 国内 profile 缺签名 | 短信签名未配置 |
| `ErrSmsTemplateMissing` | 该场景模板 ID／文案为空 | 该场景的短信模板未配置 |
| `ErrSmsIpLimited` | IP 滑窗超限 | 请求过于频繁，请稍后再试 |
| `ErrSmsPhoneLimited` | 号码日限超限 | 该号码今日发送次数已达上限 |
| `ErrSmsResendTooSoon` | 重发间隔未到 | 请 N 秒后再试（带剩余秒数） |
| `ErrSmsCodeNotFound` | 无可用行／已消费／跨场景 | 验证码不存在或已使用 |
| `ErrSmsCodeExpired` | 超过 `ExpiresAt` | 验证码已过期，请重新获取 |
| `ErrSmsCodeMismatch` | HMAC 比对不符 | 验证码错误，还可尝试 N 次 |
| `ErrSmsCodeLocked` | `Attempts >= MaxAttempts` | 验证码错误次数过多，请重新获取 |
| `ErrSmsSendFailed` | `SendStatus != 1` | 短信发送失败，请稍后重试 |

**提示语不泄露内部细节**：平台原始错误码与消息只进 `sms_logs`，不回给前台用户。后台发送记录页展示完整原因。

### 10.3 防刷分层

[INFERRED] 五层，从便宜到贵：

| 层 | 机制 | 存储 | 作用 |
|---|---|---|---|
| 1 | 图形验证码 | 既有 `Store.Verify` | 挡脚本。`Safe.Captcha == 1` 时必带 |
| 2 | IP 滑窗 | `library.RateLimiter`（进程内） | 粗筛，便宜。小时 + 日两个预算 |
| 3 | 号码重发间隔 + 日限 | DB（`sms_codes` / `sms_logs`） | 跨进程有效，挡分布式换 IP |
| 4 | 单码失败次数上限 | DB（`sms_codes.attempts`） | 挡验证码爆破 |
| 5 | 号码格式校验 | 纯函数 | 挡无效号码消耗额度 |

**第 2 层必须用独立预算**，不与 `middleware.accountProbeLimiter` 共用。共用会让短信请求把邮箱枚举探测的预算吃掉（或反之），两个功能的限额互相污染。

IP 取值沿用 `ctx.RemoteAddr()`。iris 已配 `WithRemoteAddrHeader("X-Real-IP","X-Forwarded-For","CF-Connecting-IP")`，会跳过内网并取最右侧公网地址，客户端伪造的左侧条目不生效——**不要**自己解析 `X-Forwarded-For`。

第 2 层是进程内的，多实例部署下每台各算一份。这是既有 `RateLimiter` 的固有性质，不是本设计引入的缺陷；真正的跨进程防线是第 3、4 层（DB 计数）。规格记录此事，不假装 IP 限流是全局的。

### 10.4 计费与可观测

[KNOWN] 仓库硬教训：异步／外部调用失败必须留下「为什么没跑」的线索。

1. **先落 `sms_logs(pending)` 再调平台。** 进程崩在调用中途，库里仍有一条 pending 记录可查，而不是凭空消失。
2. **失败也留痕**：`err_code`、`err_msg`、`retryable`、`cost_ms` 全部回写。
3. **`OutId` / `SessionContext` 填 `SmsLog.Id`**，平台侧对账时能反查本地记录。
4. **超时 10s**，不让一个慢平台把请求 goroutine 挂死。
5. **不存验证码明文**（§4.3），排查靠 `template_id` + `scene` 反查文案。

### 10.5 隐私与信息泄露

| 风险 | 处置 |
|---|---|
| 库被读走 → 验证码泄露 | 只存 HMAC，不存明文 |
| 日志被读走 → 验证码泄露 | 日志不存码、不存渲染正文 |
| 后台响应泄露凭据 | 掩码三件套（§3.4） |
| 自动注册泄露手机号 | UserName 用随机串（§6.5） |
| reset 场景枚举注册状态 | 未注册号码也返回统一文案（§7.1） |
| `bind_old` 变成短信轰炸跳板 | 必须登录态 + 号码等于当前用户绑定号码（§7.1） |
| 掩码脱敏污染内存真值 | 值拷贝后再脱敏（§3.4） |

## 11. 测试与验收

### 11.1 签名对照向量闭环

[INFERRED] 这是方案 A 的核心价值：没有真实凭据，也能把「签对了」证明到字节级。

**步骤：**

1. 在 `/tmp` 建隔离 Go 模块（`go mod init smsvec`），装三家官方 SDK。**仓库 `go.mod`／`go.sum` 全程不动。**
2. 起本地假服务器（`httptest`），把 SDK 的 endpoint 指向它。
3. 用固定的假 AK/SK 调 SDK 发送。**不强行注入时间与 nonce**——SDK 未必支持；改为从假服务器抓到的请求里**读出 SDK 实际用的时间戳与 nonce**。
4. 假服务器完整记录：method、path、query、全部 header、body。
5. 把同一组输入（含抓到的时间戳与 nonce）喂给我们的手写签名器，构造请求。
6. **逐字节比对** `Authorization`、canonical request 中间量、body。
7. 把输入与期望输出冻结成仓内 `provider/sms/testdata/*.json` 向量。
8. 仓内测试只读向量，**不依赖 SDK、不联网、不起服务**。
9. **反向对照**：把向量里的签名改一个字节，测试必须变红；把 canonical query 的 `%20` 换成 `+`，必须变红。

三家各一组向量。阿里云覆盖国内与国际两个 endpoint；腾讯云覆盖国内与国际（`SignName` 空与非空）；Twilio 覆盖两种凭据对与两种发件方。

⚠️ 临时模块与 SDK 只存在于验证环境，**不进 `go.mod`**。向量文件进仓，是长期资产。

### 11.2 单元测试清单

| 面 | 用例 |
|---|---|
| 号码归一化 | 格式字符剥离、区号去 `+`／`00`、意大利前导 0 **不被去掉**、E.164 超长拒绝、`+86` 锚点校验、非法区号 |
| 区域路由 | `86` → domestic；`852`／`853`／`886` → international；Twilio + `86` 拒发 |
| 凭据继承 | 国际留空沿用国内；**签名与模板不继承**；国内缺凭据时报错而非静默用国际 |
| OTP 状态机 | 过期、错码、超次、重放、跨场景、发送失败的码不可用、并发消费只成一个 |
| 限额 | 重发间隔、号码日限、IP 小时／日限；被拒的请求**不延长窗口** |
| 查重事务 | 并发同 `(区号, 号码)` 只成一个；空号跳过查重；跨国同号允许并存 |
| 部分更新 | 只传一个顶层字段不清空其余；厂商对象整体替换语义符合 §3.5 |
| 掩码 | `setting_get` 后内存真值不变；哨兵写回沿用真值；显式空串才清空 |
| 自动注册 | `Status=0` 拒登；UserName 随机且不含手机号；复用 `RegisterUser` 的默认组与状态 |
| 换绑 | 旧新双验；任一失败整体回滚；**未绑号时跳过旧号校验** |

`go test ./provider/` 整包会被沙箱 SIGTERM（exit 137），**必须按 `-run` 分批跑**。`library`／`controller/manageController`／`pkg/mcp/intent` 三包可整跑。

### 11.3 护栏测试（go/ast + 反射）

[KNOWN] 仓库硬要求：每条护栏都要有**反向对照**，证明「改坏了会变红」。声明式清单一律用 go/ast 穷举校验，**不写死总数**。

| 护栏 | 断言 | 反向对照 |
|---|---|---|
| `TestSmsTablesRegistered` | 两张新表都在 `AutoMigrateDB` 的 models 切片 | 合成源码缺一表必须报缺失；扫描器找不到切片必须 Fatal |
| `TestSmsLogIndexOrdersSiteFirst` | `idx_site_created` 列序为 `(site_id, created_time)` | priority 反序必须判为反序 |
| `TestSmsCodeHashNotSerialized` | `SmsCode.CodeHash` 的 json tag 为 `-` | 用一个正常字段证明检查不恒真 |
| `TestSmsSecretFieldsMasked` | §3.4 表里每个字段在读端点被掩码 | 非凭据字段（`sign_name`／`sms_sdk_app_id`）**不得**被掩码 |
| `TestSmsMaskWriteBackProtected` | 写端点每个掩码字段都有哨兵分支 | 删掉一个分支必须红 |
| `TestSmsIntentStaysGated` | §9.2 四处名单一致 | 把 `channel_sms` 挪进 `dailyOpen` 必须红 |
| `TestSmsSecretFieldNamesCovered` | `IsSecretField` 对 `auth_token`／`account_sid`／`api_key_sid` 返回 true | `token_expire`／`token_type` 必须返回 **false** |
| `TestSmsDoesNotReuseCodeCache` | `provider/smsService.go` 不出现 `library.CodeCache` | 合成源码含它必须红 |
| `TestVerifyCellphoneFormatUnchanged` | 既有正则字面量 `1[3-9][0-9]{9}` 未被改动 | 锚点从真实文件文本取，不凭印象写 |

### 11.4 集成验证

```bash
go build ./...
go vet ./...
go run ./cmd/apicatalog            # 改 route/ 后必跑
go run ./cmd/apilive -mode all -ns "" -expose "*"
go run ./cmd/apilive -wire
go run ./cmd/apicatalog -check
```

真实读写回归（**测试全程不动源码**）：

1. `go run kandaoni.com/anqicms/main -port 8011 > /tmp/anqicms_sms.log 2>&1` 起临时服务。**8001 被占用时不杀原服务**；不能编译到 `/tmp`（`config.initPath()` 用 `ExecPath`，会读不到 `config.json`）。
2. 判活看日志里的 `AI tools initialized (intent mode)`。
3. 真实调 MCP：`initialize` → 取响应头 `Mcp-Session-Id` → `tools/call`。响应是 **SSE 形态**，要抽 `data:` 行再解析。
4. 验证配置读写、掩码、部分更新、意图门禁。
5. 恢复 `api_exposure.mode = read`，删临时数据，停 8011。
6. `ai_setting` 走进程缓存（TTL 86400s），直接改库必须重启才生效。

临时脚本与中间产物进 `tmp/`，不进 `output/`。

### 11.5 前端验证

用 `./node_modules/.bin/esbuild` 对改动文件做语法校验。不跑全量 `tsc`（会 SIGTERM），不跑 `npm run format`（会动全仓）。判断是否引入类型错误看**是否命中改动文件**，不看总数（存量 38 个）。

界面验收：切 tab 不丢输入、掩码字段保存后不变空、厂商对象提交带完整子树、Twilio tab 的 `+86` 拒发提示可见。

## 12. 交付后待验项与已知限制

### 12.1 本环境无法验证的项

[KNOWN] 以下四项**不得**在实现汇报中写成「已通过」。它们需要真实凭据或生产环境，必须由用户验收：

| 待验项 | 为什么这里验不了 | 用户怎么验 |
|---|---|---|
| 真实发送成功 | 三家签名／模板都还没申请，无 AK/SK | 后台「发送测试」向自己号码发一条 |
| 模板审核通过率 | 模板内容由站长申请，平台审核 | 平台控制台看审核状态 |
| 国际到达率 | 依赖运营商链路，无法离线模拟 | 向真实国际号码发测试 |
| 生产库隔离级别 | 只实测了开发库（RR / MariaDB 11.7.2） | 生产库执行 `SELECT @@transaction_isolation` |

凭据由用户自行填写与保管，实现过程不代写、不代取、不读取真实密钥值。

### 12.2 已知限制（有意接受，不是遗漏）

1. **默认前台模板没有手机号入口。** 实测 `template/default/login.html` 只有一个 Google 登录链接（连邮箱密码都没有），`template/shop/login.html` 是邮箱 + 密码 + Google 按钮，`template/shop/register.html` 同样没有手机号字段。本期不改模板（决策 3），站长必须自行加手机号与验证码输入框才能用上本功能。**这是交付即可用性的最大缺口，必须在发布说明里显著写明。**
2. **存量号码区号一律回填 `86`。** 库里若有国际号码需站长自行订正。
3. **自动注册的用户名是随机串。** 这类用户不能用用户名登录，需到资料页补全。
4. **查重依赖 RR。** 生产库若是 RC，§4.4 的保证不成立，需另行决策。
5. **用户名与邮箱查重仍是事务外裸读后写。** 既有缺陷，本期不修（§4.4）。
6. **IP 限流是进程内的。** 多实例下每台各算一份，跨进程防线靠 DB 计数（§10.3）。
7. **无发送状态回调。** 只记录平台同步返回结果，最终到达状态未知。
8. **Twilio 不支持中国大陆号码。** 选中 Twilio 时 `+86` 用户无法登录，需在切换厂商前告知站长。
9. **通知能力未实现。** 只预留 `Sender` 接口与 `notify.` 场景码命名空间。

## 13. 参考资料

以下接口事实已在设计阶段核验（2026-11-13）：

- [调用 SendSms 接口发送短信 - 阿里云](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-sendsms) —— Version `2017-05-25`、请求参数 `PhoneNumbers`／`SignName`／`TemplateCode`／`TemplateParam`／`OutId`／`SmsUpExtendCode`、返回 `Code`／`Message`／`BizId`／`RequestId`、国内 endpoint `dysmsapi.aliyuncs.com`
- [服务接入点 - 阿里云](https://help.aliyun.com/zh/sms/developer-reference/api-dysmsapi-2017-05-25-endpoint) —— 国际 endpoint 按地域，新加坡 `dysmsapi.ap-southeast-1.aliyuncs.com`，另有 `ap-southeast-5`、`eu-central-1`
- [发送短信 SendSms - 腾讯云](https://cloud.tencent.com/document/product/382/55981) —— Version `2021-01-11`、endpoint `sms.tencentcloudapi.com`、`PhoneNumberSet`／`SmsSdkAppId`／`TemplateId`／`SignName`／`TemplateParamSet`／`SessionContext`／`SenderId`、返回 `SendStatusSet`（`SerialNo`／`PhoneNumber`／`IsoCode`／`Code`／`Message`／`Fee`）、E.164 号码格式、单次不能混发国内与国际
- [公共参数 - 腾讯云](https://cloud.tencent.com/document/product/382/52071) —— 地域取值与签名方法 v3 说明
- [签名方法 v3 - 腾讯云](https://cloud.tencent.com/document/product/213/30654) —— TC3-HMAC-SHA256 算法
- [Message Resource - Twilio](https://www.twilio.com/docs/sms/api/message-resource) —— `POST /2010-04-01/Accounts/{AccountSid}/Messages.json`、Basic Auth、`application/x-www-form-urlencoded`、`To` 必填、`From`／`MessagingServiceSid`／`Body`／`StatusCallback`／`ContentSid` 可选、响应 `sid`／`status`／`error_code`／`error_message`／`num_segments`／`price`
- [Requests to Twilio](https://www.twilio.com/docs/usage/requests-to-twilio) —— 鉴权与请求约定
- [Twilio API Errors](https://www.twilio.com/docs/api/errors) —— 错误码表
- [Twilio API Keys](https://www.twilio.com/docs/iam/api-keys) —— API Key SID + Secret 鉴权
- [阿里云短信服务产品页](https://www.aliyun.com/product/sms) —— 国内与国际短信模板需分别申请

仓内参照实现：

- `provider/storage/aws.go:89-171` —— 手写 SigV4 范式
- `controller/manageController/pluginSendmail.go:109-220` —— 配置端点、值拷贝脱敏、掩码回写保护
- `library/merge.go:20` —— `MergeJSONPayload` 部分更新
- `middleware/accountProbe.go` —— IP 滑窗防刷范式
- `provider/channelPlatform.go` —— 错误分类契约
- `provider/channel_model_test.go` —— go/ast 迁移护栏与反向对照范式
- `anqicms-admin/src/pages/plugin/channels/index.tsx` —— visitedTabs 常驻挂载范式

## 14. 交付后仍未解决的事

1. **trunk 前缀残留放大面**：规格 §6.2 规则 3 要求「本地号码不去前导 0」（意大利等国的 0 是号码有效组成部分）。因此 `(44,"07911123456")` 与 `(44,"7911123456")` 仍是两个合法键，德/法/澳等以 0 为国内长途前缀的国家同理。本期已把放大面从「区号切分（4 倍）」收窄到「trunk 前缀（约 2 倍）」，未彻底消除。彻底单射需要按国家的 trunk 前缀元数据，而「0 个新依赖」禁用 libphonenumber；凭记忆造这张表的失败模式正是规则 3 警告的「去掉会发错人」。是否另开一期由用户决定。