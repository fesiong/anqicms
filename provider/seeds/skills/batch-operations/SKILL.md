---
name: batch-operations
description: 批量操作技能：批量创建、修改、删除文档的流程指导，包括数据准备、分批执行和错误处理。
category: Operations
version: 1.1
tags: [batch, bulk, operations, migration]
---

# 批量操作技能

## 概述
本技能指导如何安全高效地执行批量操作，包括批量创建文章、批量更新分类/标签、批量修改属性等场景。

> 工具说明：全部通过精选意图（intent）完成。文档增删改查用 `content_article`（action=list/get/save/publish/delete），分类/标签结构用 `content_manage`。`save` 只覆盖传入字段，未传的正文/封面/分类/标签等自动沿用原值；新建时 `id` 留空。不要写成后台能力（cap）名。

## 通用原则
- **先小后大**：先用小批量（3-5条）测试，确认无误后再扩大范围
- **备份先行**：批量修改/删除前，先用 `content_article(action=list)` 记录将要操作的文档 ID
- **分批执行**：每批最多 20 条，避免超时或出错
- **每步验证**：每执行一批后，抽查验证结果

## 使用步骤

### 场景一：批量创建文章
适用：需要一次性创建多篇结构相似的文章

1. 用 `content_manage(action=category_list)` 获取目标分类 ID
2. 准备数据（标题列表、默认内容模板、标签等）
3. 逐条调用 `content_article(action=save, title=..., content=..., category_id=..., draft=false)`：
   - `id` 留空即新建；每次传入不同的标题
   - 共用内容模板（可在内容中加占位符后替换）
   - `draft=false` 直接发布，无需再调 publish
4. 每创建 5 条后，调用 `content_article(action=list, category_id=...)` 验证已创建成功

### 场景二：批量更新文章属性
适用：批量修改分类、标签、状态等

1. 使用 `content_article(action=list)` 获取目标文章列表和 ID
2. 改标签：`content_article(action=save, id=..., tags=["标签1","标签2"])`（每条一次；save 只覆盖 tags，其它字段沿用）
3. 改分类：`content_article(action=save, id=..., category_id=新分类ID)`
4. 每执行完一批（10条），抽查验证

### 场景三：批量发布草稿
适用：将一批草稿文章统一发布

1. 使用 `content_article(action=list, status=draft)` 获取所有草稿列表
2. 记录需要发布的文章 ID
3. 逐条调用 `content_article(action=publish, id=..., status=ok)`（ok=上架，draft=下架）
4. 完成后用 `content_article(action=list, status=ok)` 验证

### 场景四：批量删除
⚠️ 高危操作，需要用户二次确认

1. 明确删除范围和条件
2. 先用 `content_article(action=list)` 导出待删文章列表
3. 向用户展示要删除的文章和数量，获得确认
4. 逐条调用 `content_article(action=delete, id=...)`，每删 5 条暂停验证
5. 完成后输出删除报告
   - 注意：删除正式文档是**移入回收站**（回执 `moved_to_trash=true`，可恢复），只有删草稿才是物理删除

## 错误处理
- 某条创建失败 → 记录错误信息，跳过继续
- 中途中断 → 重新运行时跳过已成功的
- ID 不存在 → publish/delete 会直接报错（不会假成功），跳过即可

## 最佳实践
- 批量操作前给用户预估耗时："预计创建 20 篇文章约需 2-3 分钟"
- 操作完成后输出汇总报告：成功 X 条，失败 Y 条
- 大操作拆分成多个子任务，每完成一个子任务通知用户进度
