import NewContainer from '@/components/NewContainer';
import { getStatisticSearchDetail } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const SearchDetail: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();
  const [newKey, setNewKey] = useState<string>('');

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  useEffect(() => {}, []);

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'account.time' }),
      dataIndex: 'created_time',
      width: 160,
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'statistic.search.keyword' }),
      dataIndex: 'keyword',
    },
    {
      title: intl.formatMessage({ id: 'statistic.search.ip' }),
      dataIndex: 'ip',
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <ProTable<any>
          headerTitle={intl.formatMessage({
            id: 'menu.statistic.searchdetail',
          })}
          actionRef={actionRef}
          rowKey="id"
          search={false}
          request={(params) => {
            return getStatisticSearchDetail(params);
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

export default SearchDetail;
