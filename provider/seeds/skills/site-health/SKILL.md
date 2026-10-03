---
name: site-health
description: 站点健康检查：检查站点配置、缓存状态、SEO设置、安全设置，发现潜在问题和优化空间。
category: Maintenance
version: 1.1
tags: [health, check, maintenance, security]
---

# 站点健康检查技能

## 概述
本技能指导如何对站点进行全面健康检查，包括系统配置、SEO 设置、安全防护、性能优化等方面。

> 工具说明：数据与运维操作都走精选意图（intent）。内容用 `content_article`、分类用 `content_manage`、系统配置用 `system_config`、缓存与索引用 `siteops_maintain`、SEO 收录用 `seo`、统计用 `traffic_statistics`。不要写成后台能力（cap）名。

## 使用步骤

### 第一步：检查系统配置
- 用 `system_config(action=setting, section=system)` 读取并确认站点名称、关键词、描述等基本信息是否正确填写
- 用 `system_config(action=setting, section=rewrite)` 检查 URL 配置（伪静态、URL 模式）
- 用 `system_config(action=setting, section=content)` 确认缓存等设置是否合理

### 第二步：检查内容健康度
1. 使用 `content_article(action=list, status=draft)` 检查是否有大量草稿未发布
2. 使用 `content_article(action=list)` 抽查是否有内容为空或过短的文章、是否有过期内容（updated_time 很久以前）
3. 使用 `content_manage(action=category_list)` 检查是否有空分类（没有文章的分类）

### 第三步：检查 SEO 基础设置
- 用 `seo(action=robots_get)` 查看 robots.txt 是否误屏蔽
- 用 `seo(action=sitemap)` 确认站点地图可正常重建
- 用 `content_article(action=list)` 排查重复标题的内容

### 第四步：检查安全设置
- 确认后台登录限制是否开启
- 确认上传文件类型限制
- 确认系统已更新到最新版本

### 第五步：清理缓存与重建索引
- 用 `siteops_maintain(action=cache_clean)` 清理缓存
- 用 `siteops_maintain(action=cache_build_index)` 重建索引

### 第六步：生成健康报告
按以下格式汇总：

```
站点健康报告 - {site_name}
生成时间：{time}

✅ 正常项：
- ...

⚠️ 警告项：
- ...

❌ 问题项：
- ...

建议优先级：
1. ...
2. ...
```

## 最佳实践
- 建议每周执行一次快速检查，每月一次全面检查
- 发现问题后立即记录，分优先级处理
- 保持系统版本更新
