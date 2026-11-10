import { pluginDeletePayAccount, pluginGetPayAccounts } from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, Tag, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import PaymentAccountForm from './components/form';
import PluginPayStatistic from './components/statistic';

const PluginPay: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentAccount, setCurrentAccount] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [statisticVisible, setStatisticVisible] = useState<boolean>(false);
  const intl = useIntl();

  // plugin pay form 也有一份
  const PayWayOptions = [
    {
      value: 'wechat',
      label: intl.formatMessage({ id: 'plugin.pay.wechat' }),
    },
    {
      value: 'weapp',
      label: intl.formatMessage({ id: 'plugin.pay.weapp' }),
    },
    {
      value: 'alipay',
      label: intl.formatMessage({ id: 'plugin.pay.alipay' }),
    },
    {
      value: 'paypal',
      label: intl.formatMessage({ id: 'plugin.pay.paypal' }),
    },
    {
      value: 'stripe',
      label: intl.formatMessage({ id: 'plugin.pay.stripe' }),
    },
    {
      value: 'balance',
      label: intl.formatMessage({ id: 'plugin.pay.balance' }),
    },
    {
      value: 'offline',
      label: intl.formatMessage({ id: 'plugin.pay.offline' }),
    },
  ];

  useEffect(() => {}, []);

  const handleEditAccount = async (record: any) => {
    setCurrentAccount(record);
    setEditVisible(true);
  };

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.pay.delete.confirm' }),
      onOk: () => {
        pluginDeletePayAccount(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleAddAccount = () => {
    setCurrentAccount({});
    setEditVisible(true);
  };

  const handleViewAccount = (row: any) => {
    setCurrentAccount(row);
    setStatisticVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.pay.account-id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.account-name' }),
      dataIndex: 'account_name',
      render: (dom: any, entity) => {
        return (
          <div>
            {dom}{' '}
            {entity.is_default && (
              <Tag>{intl.formatMessage({ id: 'plugin.pay.is-default' })}</Tag>
            )}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.pay-way' }),
      dataIndex: 'pay_way',
      valueEnum: PayWayOptions.reduce((acc: any, option) => {
        acc[option.value] = { text: option.label };
        return acc;
      }, {}),
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.weight' }),
      dataIndex: 'weight',
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.health-score' }),
      dataIndex: 'health_score',
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.amount-limit' }),
      hideInSearch: true,
      dataIndex: 'min_amount',
      render: (_: any, entity) => {
        return (
          <div>
            {(entity.min_amount > 0 || entity.max_amount > 0) && (
              <div>
                {entity.min_amount > 0 && (
                  <span>Min: {(entity.min_amount / 100).toFixed(2)} - </span>
                )}
                {entity.max_amount > 0 && (
                  <span>Max: {(entity.max_amount / 100).toFixed(2)}</span>
                )}
              </div>
            )}
            {(entity.daily_amount_limit > 0 ||
              entity.daily_count_limit > 0) && (
              <div>
                {entity.daily_amount_limit > 0 && (
                  <span>
                    <FormattedMessage id="plugin.pay.daily-amount-limit" />:{' '}
                    {(entity.daily_amount_limit / 100).toFixed(2)}
                  </span>
                )}
                {entity.daily_count_limit > 0 && (
                  <span>
                    <FormattedMessage id="plugin.pay.daily-count-limit" />:{' '}
                    {entity.daily_count_limit}
                    <FormattedMessage id="plugin.pay.daily-count-limit.suffix" />
                  </span>
                )}
              </div>
            )}
            {entity.monthly_amount_limit > 0 && (
              <div>
                <FormattedMessage id="plugin.pay.monthly-amount-limit" />:{' '}
                {(entity.monthly_amount_limit / 100).toFixed(2)}
              </div>
            )}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.last-used-time' }),
      hideInSearch: true,
      dataIndex: 'last_used_time',
      render: (_: any, entity) => {
        return entity.last_used_time > 0
          ? dayjs(entity.last_used_time * 1000).format('YYYY-MM-DD HH:mm')
          : '-';
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.used-count' }),
      dataIndex: 'used_count',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.used-amount' }),
      dataIndex: 'used_amount',
      hideInSearch: true,
      render: (_: any, entity) => {
        return (entity.used_amount / 100).toFixed(2);
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.pay.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.pay.status.stop' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.pay.status.normal' }),
          status: 'Success',
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.pay.status.pause' }),
          status: 'Warning',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="view"
            onClick={() => {
              handleViewAccount(record);
            }}
          >
            <FormattedMessage id="plugin.pay.view.log" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditAccount(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'menu.plugin.pay' })}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button key="add" onClick={handleAddAccount}>
            <FormattedMessage id="plugin.pay.add" />
          </Button>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetPayAccounts(params);
        }}
        search={false}
        columnsState={{
          persistenceKey: 'payment-account-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={false}
      />
      {editVisible && (
        <PaymentAccountForm
          open={editVisible}
          account={currentAccount}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
      {statisticVisible && (
        <PluginPayStatistic
          open={statisticVisible}
          account={currentAccount}
          onCancel={() => {
            setStatisticVisible(false);
          }}
        />
      )}
    </PageContainer>
  );
};

export default PluginPay;
