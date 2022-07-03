import React, { useRef } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { getAdminLoginLogs } from '@/services';

const PluginSendmail: React.FC = () => {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'IP',
      dataIndex: 'ip',
    },
    {
      title: '状态',
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: '失败',
          status: 'Default',
        },
        1: {
          text: '正常',
          status: 'Success',
        },
      },
    },
    {
      title: '用户名',
      dataIndex: 'user_name',
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="后台登录记录"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params, sort) => {
          return getAdminLoginLogs(params);
        }}
        columns={columns}
      />
    </PageContainer>
  );
};

export default PluginSendmail;
