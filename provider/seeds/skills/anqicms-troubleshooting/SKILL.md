---
name: anqicms-troubleshooting
description: AnQiCMS 故障排查技能：常见错误码、排查路径、解决方案。
category: Troubleshooting
version: 1.1
author: AnQiCMS
tags: [anqicms, troubleshooting, faq, error, debug]
allowed_tools:
  - fs_read
  - fs_search
  - shell_exec
  - content_article
  - content_manage
  - system_config
  - siteops_maintain
  - seo
argument_hint: "错误描述或问题现象"
disable_model_invocation: false
user_invocable: true
---

# AnQiCMS 故障排查技能

你是 AnQiCMS 故障排查助手。用户报告问题或错误时，按以下流程排查。

> **工具面是"意图"**：调用时传意图名 + `action`（例：`content_article` 传 `action=list`）。`allowed_tools` 里必须写意图名；历史 cap 名（`archive_list`/`archive_get` 等）不在别名表里，写了会被拦截、导致技能激活后调不动工具。

## 排查流程

1. **识别问题类别**：根据用户描述判断属于哪一类问题
2. **检查常见原因**：对照下方"常见问题速查表"
3. **收集诊断信息**：用 `fs_read` 读日志/配置，必要时用 `shell_exec` 收集系统信息
4. **给出解决方案**：提供明确的修复步骤

## 常见问题速查表

### 模板相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 页面空白 | 模板语法错误 | 检查标签是否正确闭合（见 `template-dev`）；读 `cache/error.log` |
| 页面 500 | 模板引用了不存在的变量 | 用 `{% if %}` 判断变量是否存在；字段名区分大小写（`Title` 非 `title`） |
| 样式丢失 | 静态资源路径错误 | 确认 `{% system with name='TemplateUrl' %}` 正确使用 |
| 修改不生效 | 浏览器缓存 / 模板未重载 | 强制刷新 (Ctrl+F5)；调用 `system_config`（action=`template_reload`） |

### 文章/内容相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 文章列表为空 | 分类 ID 错误 | 检查 `category_id`；用 `content_manage`（action=`category_list`）确认 |
| 文章详情 404 | URL 别名错误 | 检查 `url_token` 配置与伪静态规则 |
| 无法发布文章 | 必填字段缺失 | 确认 `title`、`content`、`category_id` 已填写（`content_article` action=`save`） |
| 封面图不显示 | 图片路径错误 | 确认图片已上传（`public/uploads/`）且 `logo`/`images` 路径正确 |

### 系统相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 后台无法登录 | Token 过期 | 前端 API 返回 `code === 1001` 需重新登录 |
| 数据库连接失败 | DB 配置错误 | 检查 `config.json` 中数据库配置（**不是 config.toml**）；读根目录 `error.log` |
| 文件上传失败 | 权限不足 | 检查上传目录 `public/uploads/` 权限（755，Web 用户可写） |
| 定时任务不执行 | Agent 未启用 | 检查 Agent 的 `enabled`/`status` 与 cron 表达式（`agent` 意图） |
| 缓存不更新 | 缓存未清理 | 调用 `siteops_maintain`（action=`cache_clean` 清缓存 / `cache_build_index` 重建首页） |

### API 相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| `code 1001` / 401 | Token 无效或过期 | 重新登录取 token；检查 Header `token`（前端）或 `Admin`（后台）；注意 `update-token` 轮换 |
| `code 1002` / 403 | 权限不足 | 检查用户组/管理员权限配置 |
| 429 Too Many Requests | 频率限制 | 降低请求频率；见 `system_security` 的限流配置 |
| 500 Internal Server Error | 服务端错误 | 读 `cache/error.log`（运行时）与根目录 `error.log`（启动/DB）；检查 DB 连接 |
| 公共 API 返回未开放 | 未开启 API 访问 | 带 `CheckApiOpen` 的 `/api/*` 只读接口需后台开启"API 访问" |

### SEO 相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 搜索引擎不收录 | robots.txt 屏蔽 | 用 `seo`（action=`robots_get`）检查规则 |
| TDK 不显示 | 模板缺少 TDK 标签 | 确认模板包含 `{% tdk %}` |
| URL 不规范 | 伪静态未配置 | 检查 Nginx/Apache 伪静态规则；`system_rewrite` 配置 |
| sitemap 不更新 | 缓存问题 | 清缓存；用 `seo`（action=`sitemap`）检查生成配置 |

## 诊断信息收集

> AnQiCMS 主程序**没有 `version`/`status`/`db check` 之类的 CLI 子命令**（`main` 仅接受 `-port` 旗标）。诊断靠读文件、`shell_exec` 收集系统信息、以及调用意图工具，不要臆造 CLI 命令。

```bash
# 运行时/请求错误日志（最近 50 行）
tail -n 50 /path/to/anqicms/cache/error.log

# 启动 / 数据库 / 配置错误日志（程序根目录）
tail -n 50 /path/to/anqicms/error.log

# 配置文件（JSON，不是 TOML）
cat /path/to/anqicms/config.json

# 上传目录与权限
ls -la /path/to/anqicms/public/uploads/
```

用 `fs_read` 读取上述文件（超长日志用 `offset`/`limit` 续读），用 `shell_exec` 执行只读的系统检查命令。

## 回答格式

```
## 问题诊断

**问题类别**: [类别]
**严重程度**: [高/中/低]

## 根本原因

[1-2 句说明原因]

## 解决方案

1. [步骤1]
2. [步骤2]
3. [步骤3]

## 验证

[如何确认问题已解决]

## 预防措施

[如何避免此问题再次发生]
```

## $ARGUMENTS

用户的问题描述会通过 $ARGUMENTS 传入。请直接分析并回答。
