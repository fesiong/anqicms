import { pluginGetCommissions, pluginSetWithdrawApply } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef } from 'react';

const PluginFinanceCommission: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const handleWithdraw = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.finance.withdraw.confirm' }),
      content: intl.formatMessage({
        id: 'plugin.finance.withdraw.confirm.content',
      }),
      onOk: () => {
        pluginSetWithdrawApply(record).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.comment.user-name' }),
      dataIndex: 'user_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.amount' }),
      dataIndex: 'amount',
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.order-id' }),
      dataIndex: 'order_id',
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.finance.status.unwithdraw' }),
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.finance.status.withdraw' }),
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
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          {record.amount > 200 && record.status === 0 && (
            <a
              key="edit"
              onClick={() => {
                handleWithdraw(record);
              }}
            >
              <FormattedMessage id="plugin.finance.withdraw" />
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <ProTable<any>
      headerTitle={intl.formatMessage({ id: 'plugin.finance.commission' })}
      actionRef={actionRef}
      rowKey="id"
      search={false}
      tableAlertOptionRender={false}
      request={(params) => {
        return pluginGetCommissions(params);
      }}
      columnsState={{
        persistenceKey: 'commission-table',
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

export default PluginFinanceCommission;
