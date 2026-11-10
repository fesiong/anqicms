import { PageContainer } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card, Tabs } from 'antd';
import React from 'react';
import PluginFinanceCommission from './components/comission';
import PluginFinanceFlow from './components/flow';
import PluginFinanceWithdraw from './components/withdraw';

const { TabPane } = Tabs;

const PluginFinance: React.FC<any> = () => {
  const intl = useIntl();

  return (
    <PageContainer>
      <Card>
        <Tabs tabPosition={'left'}>
          <TabPane tab={intl.formatMessage({ id: 'plugin.finance.flow' })} key="1">
            <PluginFinanceFlow />
          </TabPane>
          <TabPane tab={intl.formatMessage({ id: 'plugin.finance.commission' })} key="2">
            <PluginFinanceCommission />
          </TabPane>
          <TabPane tab={intl.formatMessage({ id: 'plugin.finance.withdraw.name' })} key="3">
            <PluginFinanceWithdraw />
          </TabPane>
        </Tabs>
      </Card>
    </PageContainer>
  );
};

export default PluginFinance;
