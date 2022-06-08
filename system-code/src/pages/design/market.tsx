import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { getStatisticInfo } from '@/services/statistic';
import { Alert, Card } from 'antd';
import './index.less';

const DesignMarket: React.FC = () => {


  return (
    <PageContainer>
      <Card>
        <Alert message='模板市场的模板由热心的网友提供，如果你有想共享的模板，可以联系：https://www.kandaoni.com' />
      </Card>
    </PageContainer>
  );
};

export default DesignMarket;
