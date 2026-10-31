import { pluginGetPayAccountStatistic } from '@/services';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef } from 'react';

export type PayStatisticProps = {
  onCancel: (flag?: boolean) => void;
  open: boolean;
  account: any;
};

const PluginPayStatistic: React.FC<PayStatisticProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  useEffect(() => {}, []);

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.pay.stat-time' }),
      hideInSearch: true,
      dataIndex: 'stat_time',
      render: (dom: any, entity) => {
        return dayjs(entity.stat_time * 1000).format('YYYY-MM-DD');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.daily-count' }),
      dataIndex: 'daily_count',
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.daily-amount' }),
      dataIndex: 'daily_amount',
      hideInSearch: true,
      render: (_: any, entity) => {
        return (entity.daily_amount / 100).toFixed(2);
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.monthly-amount' }),
      dataIndex: 'monthly_amount',
      hideInSearch: true,
      render: (_: any, entity) => {
        return (entity.monthly_amount / 100).toFixed(2);
      },
    },
  ];

  return (
    <Modal
      width={1000}
      title={intl.formatMessage({ id: 'plugin.pay.view.log' })}
      open={props.open}
      footer={null}
      onCancel={() => {
        props.onCancel();
      }}
      onClose={() => {
        props.onCancel();
      }}
    >
      <ProTable<any>
        actionRef={actionRef}
        rowKey="id"
        tableAlertOptionRender={false}
        request={(params) => {
          params.account_id = props.account.id;
          return pluginGetPayAccountStatistic(params);
        }}
        search={false}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </Modal>
  );
};

export default PluginPayStatistic;
