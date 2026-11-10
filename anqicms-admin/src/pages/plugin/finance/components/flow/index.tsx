import { pluginGetFinances } from '@/services/plugin/finance';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

const PluginFinanceFlow: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.comment.user-name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.direction' }),
      dataIndex: 'direction',
      valueEnum: {
        1: {
          text: intl.formatMessage({ id: 'plugin.finance.direction.in' }),
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.finance.direction.out' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.amount' }),
      dataIndex: 'amount',
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.after-amount' }),
      dataIndex: 'after_amount',
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.type' }),
      dataIndex: 'action',
      valueEnum: {
        1: {
          text: intl.formatMessage({ id: 'plugin.finance.type.sale' }),
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.finance.type.buy' }),
        },
        3: {
          text: intl.formatMessage({ id: 'plugin.finance.type.refund' }),
        },
        4: {
          text: intl.formatMessage({ id: 'plugin.finance.type.charge' }),
        },
        5: {
          text: intl.formatMessage({ id: 'plugin.finance.type.withdraw' }),
        },
        6: {
          text: intl.formatMessage({ id: 'plugin.finance.type.spread' }),
        },
        7: {
          text: intl.formatMessage({ id: 'plugin.finance.type.cashback' }),
        },
        8: {
          text: intl.formatMessage({ id: 'plugin.finance.type.commission' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.time' }),
      dataIndex: 'created_time',
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
  ];

  return (
    <ProTable<any>
      headerTitle={intl.formatMessage({ id: 'plugin.finance.flow' })}
      actionRef={actionRef}
      rowKey="id"
      search={false}
      tableAlertOptionRender={false}
      request={(params) => {
        return pluginGetFinances(params);
      }}
      columnsState={{
        persistenceKey: 'finance-table',
        persistenceType: 'localStorage',
      }}
      columns={columns}
      rowSelection={false}
      pagination={{
        showSizeChanger: true,
        showQuickJumper: true,
      }}
    />
  );
};

export default PluginFinanceFlow;
