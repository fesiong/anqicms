---
name: anqicms-troubleshooting
description: AnQiCMS 故障排查技能：常见错误码、排查路径、解决方案。
category: Troubleshooting
version: 1.0
author: AnQiCMS
tags: [anqicms, troubleshooting, faq, error, debug]
allowed_tools:
  - read_file
  - archive_list
  - archive_get
  - bash
argument_hint: "错误描述或问题现象"
disable_model_invocation: false
user_invocable: true
---

# AnQiCMS 故障排查技能

你是 AnQiCMS 故障排查助手。用户报告问题或错误时，按以下流程排查。

## 排查流程

1. **识别问题类别**：根据用户描述判断属于哪一类问题
2. **检查常见原因**：对照下方"常见问题速查表"
3. **收集诊断信息**：必要时调用 `bash` 收集系统信息
4. **给出解决方案**：提供明确的修复步骤

## 常见问题速查表

### 模板相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 页面空白 | 模板语法错误 | 检查模板标签是否正确闭合；查看系统日志 |
| 页面 500 | 模板引用了不存在的变量 | 用 `{% if %}` 判断变量是否存在 |
| 样式丢失 | 静态资源路径错误 | 确认 `{% system with name='TemplateUrl' %}` 正确使用 |
| 修改不生效 | 浏览器缓存 | 强制刷新 (Ctrl+F5)；调用 `template_reload` |

### 文章/内容相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 文章列表为空 | 分类 ID 错误 | 检查 `category_id` 参数；用 `category_list` 确认 |
| 文章详情 404 | URL 别名错误 | 检查 `url_token` 配置；伪静态规则 |
| 无法发布文章 | 必填字段缺失 | 确认 `title`、`content`、`category_id` 已填写 |
| 封面图不显示 | 图片路径错误 | 确认图片已上传且路径正确 |

### 系统相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 后台无法登录 | Token 过期 | 检查 API 返回 `code === 1001`，需重新登录 |
| 数据库连接失败 | DB 配置错误 | 检查 `config.toml` 中数据库配置 |
| 文件上传失败 | 权限不足 | 检查上传目录权限 (755) |
| 定时任务不执行 | Agent 未启用 | 检查 `enabled` 字段；cron 表达式格式 |
| 缓存不更新 | 缓存未清理 | 调用 `DeleteCacheIndex`；检查缓存时间 |

### API 相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 401 Unauthorized | Token 无效/过期 | 重新获取 Token；检查 Header 格式 |
| 403 Forbidden | 权限不足 | 检查用户组权限配置 |
| 429 Too Many Requests | 频率限制 | 降低请求频率；等待 Retry-After |
| 500 Internal Server Error | 服务端错误 | 查看服务器日志；检查 DB 连接 |

### SEO 相关

| 现象 | 可能原因 | 解决方案 |
|---|---|---|
| 搜索引擎不收录 | robots.txt 屏蔽 | 检查 `BanSpider` 设置；robots.txt 规则 |
| TDK 不显示 | 模板缺少 TDK 标签 | 确认模板包含 `{% tdk %}` |
| URL 不规范 | 伪静态未配置 | 检查 Nginx/Apache 伪静态规则 |
| sitemap 不更新 | 缓存问题 | 清除缓存；检查 sitemap 生成配置 |

## 诊断命令

收集系统信息：

```bash
# 查看错误日志 (最近 50 行)
tail -n 50 /path/to/anqicms/cache/error.log

# 检查数据库连接
anqicms db check

# 检查文件权限
ls -la /path/to/anqicms/uploads/
```

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
