---
title: 业务组件
sidemenu: false
---

> 此功能由[dumi](https://d.umijs.org/zh-CN/guide/advanced#umi-%E9%A1%B9%E7%9B%AE%E9%9B%86%E6%88%90%E6%A8%A1%E5%BC%8F)提供，dumi 是一个 📖 为组件开发场景而生的文档工具，用过的都说好。

# 业务组件

这里列举了 Pro 中所有用到的组件，这些组件不适合作为组件库，但是在业务中却真实需要。所以我们准备了这个文档，来指导大家是否需要使用这个组件。

## Footer 页脚组件

这个组件自带了一些 Pro 的配置，你一般都需要改掉它的信息。

```tsx
/**
 * background: '#f0f2f5'
 */
import React from 'react';
import Footer from '@/components/Footer';

export default () => <Footer />;
```

## HeaderDropdown 头部下拉列表

HeaderDropdown 是 antd Dropdown 的封装，但是增加了移动端的特殊处理，用法也是相同的。

```tsx
/**
 * background: '#f0f2f5'
 */
import { Button, Menu } from 'antd';
import React from 'react';
import HeaderDropdown from '@/components/HeaderDropdown';

export default () => {
  const menuHeaderDropdown = (
    <Menu selectedKeys={[]}>
      <Menu.Item key="center">个人中心</Menu.Item>
      <Menu.Item key="settings">个人设置</Menu.Item>
      <Menu.Divider />
      <Menu.Item key="logout">退出登录</Menu.Item>
    </Menu>
  );
  return (
    <HeaderDropdown overlay={menuHeaderDropdown}>
      <Button>hover 展示菜单</Button>
    </HeaderDropdown>
  );
};
```

## HeaderSearch 头部搜索框

一个带补全数据的输入框，支持收起和展开 Input

```tsx
/**
 * background: '#f0f2f5'
 */
import { Button, Menu } from 'antd';
import React from 'react';
import HeaderSearch from '@/components/HeaderSearch';

export default () => {
  return (
    <HeaderSearch
      placeholder="站内搜索"
      defaultValue="umi ui"
      options={[
        { label: 'Ant Design Pro', value: 'Ant Design Pro' },
        {
          label: 'Ant Design',
          value: 'Ant Design',
        },
        {
          label: 'Pro Table',
          value: 'Pro Table',
        },
        {
          label: 'Pro Layout',
          value: 'Pro Layout',
        },
      ]}
      onSearch={(value) => {
        console.log('input', value);
      }}
    />
  );
};
```

### API

| 参数            | 说明                               | 类型                         | 默认值 |
| --------------- | ---------------------------------- | ---------------------------- | ------ |
| value           | 输入框的值                         | `string`                     | -      |
| onChange        | 值修改后触发                       | `(value?: string) => void`   | -      |
| onSearch        | 查询后触发                         | `(value?: string) => void`   | -      |
| options         | 选项菜单的的列表                   | `{label,value}[]`            | -      |
| defaultVisible  | 输入框默认是否显示，只有第一次生效 | `boolean`                    | -      |
| visible         | 输入框是否显示                     | `boolean`                    | -      |
| onVisibleChange | 输入框显示隐藏的回调函数           | `(visible: boolean) => void` | -      |

## RightContent

RightContent 是以上几个组件的组合，同时新增了 plugins 的 `SelectLang` 插件。

```tsx | pure
<Space>
  <HeaderSearch
    placeholder="站内搜索"
    defaultValue="umi ui"
    options={[
      { label: <a href="https://umijs.org/zh/guide/umi-ui.html">umi ui</a>, value: 'umi ui' },
      {
        label: <a href="next.ant.design">Ant Design</a>,
        value: 'Ant Design',
      },
      {
        label: <a href="https://protable.ant.design/">Pro Table</a>,
        value: 'Pro Table',
      },
      {
        label: <a href="https://prolayout.ant.design/">Pro Layout</a>,
        value: 'Pro Layout',
      },
    ]}
  />
  <Tooltip title="使用文档">
    <span
      className={styles.action}
      onClick={() => {
        window.location.href = 'https://pro.ant.design/docs/getting-started';
      }}
    >
      <QuestionCircleOutlined />
    </span>
  </Tooltip>
  <Avatar />
  {REACT_APP_ENV && (
    <span>
      <Tag color={ENVTagColor[REACT_APP_ENV]}>{REACT_APP_ENV}</Tag>
    </span>
  )}
</Space>
```
