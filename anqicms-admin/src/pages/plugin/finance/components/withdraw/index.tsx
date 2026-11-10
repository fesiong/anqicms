import {
  pluginGetWithdraws,
  pluginSetWithdrawApproval,
  pluginSetWithdrawFinished,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormRadio,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const PluginFinanceWithdraw: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentInfo, setCurrentInfo] = useState<boolean>(false);
  const [withdrawVisible, setWithdrawVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleSetApproval = (record: any) => {
    setCurrentInfo(record);
    setWithdrawVisible(true);
  };

  const saveWithdrawApproval = async () => {
    pluginSetWithdrawApproval(currentInfo).then((res) => {
      message.info(res.msg);
      setWithdrawVisible(false);
      actionRef.current?.reload();
    });
  };

  const setWithdrawFinished = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.finance.withdraw.finish.confirm',
      }),
      content: intl.formatMessage({
        id: 'plugin.finance.withdraw.finish.content',
      }),
      onOk: () => {
        pluginSetWithdrawFinished(record).then((res) => {
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
      title: intl.formatMessage({ id: 'plugin.finance.withdraw.amount' }),
      dataIndex: 'amount',
      render: (dom: any) => {
        return dom / 100;
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({
            id: 'plugin.finance.withdraw.status.waiting',
          }),
        },
        1: {
          text: intl.formatMessage({
            id: 'plugin.finance.withdraw.status.agree',
          }),
        },
        2: {
          text: intl.formatMessage({
            id: 'plugin.finance.withdraw.status.finish',
          }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.withdraw.apply-time' }),
      dataIndex: 'created_time',
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.finance.withdraw.success-time' }),
      dataIndex: 'success_time',
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
          {record.status === 0 && (
            <a
              key="edit"
              onClick={() => {
                handleSetApproval(record);
              }}
            >
              <FormattedMessage id="plugin.finance.withdraw.agree" />
            </a>
          )}
          {record.status === 1 && (
            <a
              key="finish"
              onClick={() => {
                setWithdrawFinished(record);
              }}
            >
              <FormattedMessage id="plugin.finance.withdraw.finish" />
            </a>
          )}
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'plugin.finance.withdraw.name' })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetWithdraws(params);
        }}
        columnsState={{
          persistenceKey: 'withdraw-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      {withdrawVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.finance.withdraw.apply' })}
          open={withdrawVisible}
          onOpenChange={(flag) => {
            setWithdrawVisible(flag);
          }}
          onFinish={saveWithdrawApproval}
        >
          <ProFormRadio.Group
            name="status"
            label={intl.formatMessage({ id: 'plugin.finance.withdraw.apply' })}
            options={[
              {
                value: 1,
                label: intl.formatMessage({
                  id: 'plugin.finance.withdraw.agree',
                }),
              },
            ]}
          />
        </ModalForm>
      )}
    </>
  );
};

export default PluginFinanceWithdraw;
