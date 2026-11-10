import { getAdminActionLogs } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

const AdminActionLog: React.FC = () => {
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
      title: intl.formatMessage({ id: 'account.logs.time' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'account.logs.action' }),
      dataIndex: 'log',
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'account.logs.action-record' })}
        rowKey="id"
        actionRef={actionRef}
        search={false}
        request={(params) => {
          return getAdminActionLogs(params);
        }}
        columnsState={{
          persistenceKey: 'action-logs-table',
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

export default AdminActionLog;
