import { getAdminLoginLogs } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

const AdminLoginLog: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'account.time' }),
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: 'IP',
      dataIndex: 'ip',
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'account.logs.failure' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'account.logs.success' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'account.base.username' }),
      dataIndex: 'user_name',
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'account.logs.login-record' })}
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params) => {
          return getAdminLoginLogs(params);
        }}
        columnsState={{
          persistenceKey: 'login-logs-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </PageContainer>
  );
};

export default AdminLoginLog;
