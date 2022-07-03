import React, { useRef } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { getStatisticIncludeInfo } from '@/services';

const StatisticDetail: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '百度收录',
      dataIndex: 'baidu_count',
    },
    {
      title: '搜索收录',
      dataIndex: 'sogou_count',
    },
    {
      title: '搜搜收录',
      dataIndex: 'so_count',
    },
    {
      title: '必应收录',
      dataIndex: 'bing_count',
    },
    {
      title: '谷歌收录',
      dataIndex: 'google_count',
      width: 80,
    }
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="收录详细记录"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        request={(params, sort) => {
          return getStatisticIncludeInfo(params);
        }}
        columns={columns}
      />
    </PageContainer>
  );
};

export default StatisticDetail;
