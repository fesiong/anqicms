# config/oem/ — 品牌数据源

一个子目录 = 一个 OEM 品牌。`brand.json` 是该品牌的**唯一真相源**：后端品牌表由
`go:embed` 直接编译进二进制，前端注入文件由 `cmd/oemgen` 从同一批 JSON 生成。
两侧都从不手写，因此不可能出现“后台叫甲、前台叫乙”的漂移。

放在 `config/` 下而不是仓库根，是 `go:embed` 的限制：embed 模式不能引用父目录，
而品牌数据必须编译进二进制（见下）。

```
config/oem/
  anqicms/            官方版（兜底品牌，必须始终存在）
    brand.json
    assets/           可选：logo.svg / favicon.ico / logo.png，缺省时用官方资源
  acme/               客户 A
    brand.json
    assets/
      logo.svg
```

## 为什么品牌必须编译进包

内网部署、首次启动、官网不可达时，都必须能完整显示客户品牌。任何“从远端拉品牌、
拉不到退回默认”的设计，会在最需要它的场景里退回官方品牌 —— 等于把客户后台变成
竞品广告位。所以品牌数据、Logo、文档地址全部编译期固化；官网只负责下发
**可变状态**（授权期、余额、最新版本号）。

## 加一个 OEM

```bash
mkdir -p config/oem/acme/assets
cp config/oem/anqicms/brand.json config/oem/acme/brand.json   # 改 id/name/vendor/域名
go run ./cmd/oemgen -oem acme                                 # 生成前端注入文件
make build OEM=acme                                           # 产出 acme 品牌的后台与二进制
```

`id` 必须等于目录名，且只能是 `[a-z0-9][a-z0-9_-]*`：它会进 `?oem=` 查询参数、
升级包文件名和二进制名，出现空格或中文会直接坏掉。

## 字段

| 字段 | 用途 | 前端 | 后端 |
|---|---|---|---|
| `id` | 编译期标识，进 `?oem=` 与产物命名 | `brand.id` | `config.OEMID` |
| `name` | 品牌主名（拉丁字母） | 登录页、侧边栏、`document.title` | 出站 User-Agent |
| `name_cn` | 中文名；为空回退 `name` | 中文界面文案 | 安装向导标题 |
| `vendor` | 版权持有方 | 页脚版权行 | — |
| `website` | 官网 | 页脚/关于链接 | — |
| `docs` | 帮助文档根地址 | 帮助入口 | `provider/designDoc.go` 帮助链接 |
| `auth_base` | 账号/授权/AI 网关根地址 | 账号/充值页 | 账号登录、AI 官方通道 |
| `download_base` | 升级包下载根地址 | — | 版本检查与升级下载 |
| `binary` | 主程序文件名（在升级包内） | — | 升级时识别主程序 |
| `database` | 安装向导默认库名 | — | `controller/install.go` |
| `support_email` | 支持邮箱，可空 | 关于页 | 错误提示 |

## 生成物

| 文件 | 角色 | 是否提交 |
|---|---|---|
| `config/oem/<id>/brand.json` | 真相源 | 是 |
| `anqicms-admin/src/oem/brand.ts` | 前端品牌对象 | 是（默认为官方版内容） |

`src/oem/brand.ts` 提交进仓库，是为了 umi 的 dev/build 不必先跑一遍 Go 代码生成。
为某个 OEM 构建后它会变成该品牌的版本，属于预期的脏工作区；`make build` 结束时会
提示用 `git checkout -- anqicms-admin/src/oem` 还原。

## 校验

```bash
go test ./config/ -run 'Brand|OEM'   # 字段完整性、兜底行为、官方值回归
go run ./cmd/oemgen -check           # 构建前门禁，有问题退出码 1
go test ./cmd/oemgen/                # 品牌验收工具自身的反向对照
```

## 验收：装完之后还看不看得见别家的名字

```bash
make brand-check URL=http://127.0.0.1:8001          # 官方版
make brand-check URL=http://127.0.0.1:8001 OEM=acme # 客户版
```

抓首页、后台外壳、`robots.txt`、`sitemap.xml` 的真实响应，扫两类问题：

- **不该有的**：其它已登记品牌的标识出现（禁词表来自 `config/oem/` 里除本品牌
  以外的所有品牌，所以多加一个客户就自动多一份检查）；
- **该有的没有**：后台标题里连本品牌名都没有——通常是 `-oem` 传错了，工具会顺带
  识别站点真实品牌并提示正确取值。

两个已知边界，别把它当万能：

1. **要对着 `make build` 的产物跑**。源码起的 dev 服务用的是仓库里的
   `template/default`，里面的官方反链是打包时由 `-brand-dir` 替换的，跑 dev 服务
   必然误报。
2. **不覆盖后台 SPA 里由 JS 在浏览器端替换的文案**。那部分要真实执行 JS 才能看到，
   而前端目前没有测试框架，这块是纯人工验收——改完 `src/locales` 或 `src/oem/`
   之后，请实际登录后台看一遍侧边栏、页脚与登录页。

