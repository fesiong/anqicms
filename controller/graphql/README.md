# GraphQL API v2 文档

## 简介

AnqiCMS GraphQL API 提供了一种灵活的方式来查询和修改数据。与传统的REST API相比，GraphQL允许客户端精确指定所需的数据，减少了网络请求和数据传输量。

## 端点

- GraphQL API: `/api/v2/graphql`
- GraphQL Playground (开发调试): `/api/v2/playground`

## 查询示例

### 获取文章

```graphql
query {
  archive(id: 1) {
    id
    title
    content
    publish_time
  }
}
```

### 获取文章列表

```graphql
query {
  archives(category_id: 1, limit: 10, offset: 0) {
    id
    title
    logo
  }
}
```

### 获取分类

```graphql
query {
  category(id: 1) {
    id
    title
    description
  }
}
```

### 获取所有分类

```graphql
query {
  categories {
    id
    title
  }
}
```

## 变更示例

### 创建文章

```graphql
mutation {
  createArchive(
    title: "新文章标题", 
    content: "文章内容", 
    category_id: 1
  ) {
    id
    title
  }
}
```

### 更新文章

```graphql
mutation {
  updateArchive(
    id: 1, 
    title: "更新后的标题"
  ) {
    id
    title
  }
}
```

### 删除文章

```graphql
mutation {
  deleteArchive(id: 1)
}
```

## 使用说明

1. 所有请求应发送到 `/api/v2/graphql` 端点
2. 请求方法必须为 POST
3. 请求体应包含 JSON 格式的 GraphQL 查询
4. 响应将以 JSON 格式返回

## 开发调试

访问 `/api/v2/playground` 可以使用交互式的 GraphQL Playground 进行查询测试和文档浏览。