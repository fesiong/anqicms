import { Card, PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Empty } from 'antd';
import React, { useState } from 'react';
import ChannelConnect from './components/connect';

const PluginChannels: React.FC = () => {
  const intl = useIntl();
  const [activeTabKey, setActiveTabKey] = useState<string>('connect');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['connect']);

  // 尚未实现的 tab 只渲染占位，不请求任何接口，也不报错
  const renderPlaceholder = (key: string) => {
    if (!visitedTabs.includes(key)) {
      return null;
    }
    return (
      <div style={{ display: activeTabKey === key ? 'block' : 'none' }}>
        <Empty
          description={intl.formatMessage({ id: 'plugin.channels.placeholder' })}
        />
      </div>
    );
  };

  return (
    <PageContainer>
      <Card
        activeTabKey={activeTabKey}
        onTabChange={(tabKey) => {
          setActiveTabKey(tabKey);
          setVisitedTabs((prev) =>
            prev.includes(tabKey) ? prev : [...prev, tabKey],
          );
        }}
        tabList={[
          {
            key: 'connect',
            label: intl.formatMessage({ id: 'plugin.channels.tab.connect' }),
          },
          {
            key: 'binding',
            label: intl.formatMessage({ id: 'plugin.channels.tab.binding' }),
          },
          {
            key: 'targets',
            label: intl.formatMessage({ id: 'plugin.channels.tab.targets' }),
          },
          {
            key: 'notifications',
            label: intl.formatMessage({
              id: 'plugin.channels.tab.notifications',
            }),
          },
          {
            key: 'records',
            label: intl.formatMessage({ id: 'plugin.channels.tab.records' }),
          },
        ]}
      >
        {visitedTabs.includes('connect') && (
          <div
            style={{ display: activeTabKey === 'connect' ? 'block' : 'none' }}
          >
            <ChannelConnect />
          </div>
        )}
        {renderPlaceholder('binding')}
        {renderPlaceholder('targets')}
        {renderPlaceholder('notifications')}
        {renderPlaceholder('records')}
      </Card>
    </PageContainer>
  );
};

export default PluginChannels;
