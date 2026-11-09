import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card } from 'antd';
import React, { useState } from 'react';
import ChannelBinding from './components/binding';
import ChannelConnect from './components/connect';
import ChannelNotifications from './components/notifications';
import ChannelRecords from './components/records';
import ChannelTargets from './components/targets';

const PluginChannels: React.FC = () => {
  const intl = useIntl();
  const [activeTabKey, setActiveTabKey] = useState<string>('connect');
  // 已访问过的 tab 常驻挂载（仅切换显示），避免切换 tab 时表单被卸载丢失未保存输入
  const [visitedTabs, setVisitedTabs] = useState<string[]>(['connect']);

  // 常驻挂载 + display 切换；子组件靠 active 感知重新进入
  // （binding 离开时清空一次性 token 明文，其余 tab 重新激活时重拉下拉与列表）
  const renderTab = (key: string, node: React.ReactNode) => {
    if (!visitedTabs.includes(key)) {
      return null;
    }
    return (
      <div style={{ display: activeTabKey === key ? 'block' : 'none' }}>
        {node}
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
        {renderTab('connect', <ChannelConnect />)}
        {renderTab(
          'binding',
          <ChannelBinding active={activeTabKey === 'binding'} />,
        )}
        {renderTab(
          'targets',
          <ChannelTargets active={activeTabKey === 'targets'} />,
        )}
        {renderTab(
          'notifications',
          <ChannelNotifications active={activeTabKey === 'notifications'} />,
        )}
        {renderTab('records', <ChannelRecords />)}
      </Card>
    </PageContainer>
  );
};

export default PluginChannels;
