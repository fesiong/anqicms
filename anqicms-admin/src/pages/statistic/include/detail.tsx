import NewContainer from '@/components/NewContainer';
import { getStatisticIncludeInfo } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const StatisticIncludeDetail: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'account.time' }),
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'statistic.indexing.baidu' }),
      dataIndex: 'baidu_count',
    },
    {
      title: intl.formatMessage({ id: 'statistic.indexing.sogou' }),
      dataIndex: 'sogou_count',
    },
    {
      title: intl.formatMessage({ id: 'statistic.indexing.so' }),
      dataIndex: 'so_count',
    },
    {
      title: intl.formatMessage({ id: 'statistic.indexing.bing' }),
      dataIndex: 'bing_count',
    },
    {
      title: intl.formatMessage({ id: 'statistic.indexing.google' }),
      dataIndex: 'google_count',
      width: 80,
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <ProTable<any>
          headerTitle={intl.formatMessage({ id: 'statistic.indexing.detail' })}
          actionRef={actionRef}
          rowKey="id"
          search={false}
          request={(params) => {
            return getStatisticIncludeInfo(params);
          }}
          columnsState={{
            persistenceKey: 'statistic-include-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Card>
    </NewContainer>
  );
};

export default StatisticIncludeDetail;
