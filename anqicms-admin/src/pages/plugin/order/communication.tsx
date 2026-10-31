import {
  pluginGetCommunicationOrders,
  pluginOrderApplyRefund,
  pluginSendCommunicationQuote,
} from '@/services';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import CommunicationOrderDetail from './components/communication/detail';
import OrderForm from './components/orderForm';
import CommunicationOrderSetting from './setting/communication';

const PluginCommunicationOrder: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentOrder, setcurrentOrder] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [quoteVisible, setQuoteVisible] = useState<boolean>(false);
  const [viewOrderVisible, setViewOrderVisible] = useState<boolean>(false);
  const [viewOrder, setViewOrder] = useState<any>({});
  const intl = useIntl();

  const handleEditOrder = async (record: any) => {
    setcurrentOrder(record);
    setEditVisible(true);
  };

  const handleViewPayOrder = (record: any) => {
    setViewOrder({ order_id: record.order_id });
    setViewOrderVisible(true);
  };

  const handleSetQuote = async (record: any) => {
    setcurrentOrder(record);
    setQuoteVisible(true);
  };

  const saveOrderQuote = async (values: any) => {
    const postData = {
      ...values,
      id: currentOrder.id,
      quote_price: Number(Math.floor(values.quote_price * 100)),
    };
    pluginSendCommunicationQuote(postData).then(async (res) => {
      message.info(res.msg);
      setQuoteVisible(false);
      actionRef.current?.reload();
    });
  };

  const handleSetClose = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.order.communication.close.confirm',
      }),
      onOk: () => {
        pluginOrderApplyRefund(record).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.order.order-id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({
        id: 'plugin.order.communication.title',
      }),
      dataIndex: 'title',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.buy.user-name' }),
      dataIndex: 'user.user_name',
      key: 'user_name',
      hideInSearch: true,
      render: (dom: any, entity) => {
        return entity.user?.user_name || '-';
      },
    },
    {
      title: intl.formatMessage({
        id: 'plugin.order.communication.quote-price',
      }),
      dataIndex: 'quote_price',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100 || '-';
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.create-time' }),
      dataIndex: 'created_time',
      hideInSearch: true,
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.wait',
          }),
        },
        1: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.doing',
          }),
        },
        2: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.quote',
          }),
        },
        3: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.accepted',
          }),
          status: 'Success',
        },
        4: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.finished',
          }),
        },
        5: {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.rejected',
          }),
        },
        '-1': {
          text: intl.formatMessage({
            id: 'plugin.order.communication.status.closed',
          }),
        },
      },
      renderFormItem: () => {
        return (
          <ProFormSelect
            name="status"
            request={async () => {
              return [
                {
                  label: intl.formatMessage({ id: 'plugin.order.status.all' }),
                  value: '',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.wait',
                  }),
                  value: 'waiting',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.doing',
                  }),
                  value: 'doing',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.quote',
                  }),
                  value: 'quote',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.accepted',
                  }),
                  value: 'accepted',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.closed',
                  }),
                  value: 'closed',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.communication.status.finished',
                  }),
                  value: 'finished',
                },
              ];
            }}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          {(record.status === 0 ||
            record.status === 1 ||
            record.status === 2 ||
            record.status === 5) && (
            <>
              <a
                onClick={() => {
                  handleSetQuote(record);
                }}
              >
                <FormattedMessage id="plugin.order.communication.set-quote" />
              </a>
              <a
                onClick={() => {
                  handleSetClose(record);
                }}
              >
                <FormattedMessage id="plugin.order.communication.set-close" />
              </a>
            </>
          )}
          {!!record.order_id && (
            <a
              onClick={() => {
                handleViewPayOrder(record);
              }}
            >
              <FormattedMessage id="plugin.order.communication.view-order" />
            </a>
          )}
          <a
            onClick={() => {
              handleEditOrder(record);
            }}
          >
            <FormattedMessage id="plugin.order.view" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'menu.plugin.order' })}
        actionRef={actionRef}
        rowKey="id"
        search={{
          span: { xs: 24, sm: 12, md: 8, lg: 8, xl: 6, xxl: 6 },
        }}
        toolBarRender={() => [
          <CommunicationOrderSetting key="setting" onCancel={() => {}}>
            <Button>
              <FormattedMessage id="plugin.order.setting" />
            </Button>
          </CommunicationOrderSetting>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetCommunicationOrders(params);
        }}
        columnsState={{
          persistenceKey: 'order-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      {editVisible && (
        <CommunicationOrderDetail
          open={editVisible}
          order={currentOrder}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
          }}
        />
      )}
      {quoteVisible && (
        <ModalForm
          title={intl.formatMessage({
            id: 'plugin.order.communication.set-quote',
          })}
          width={480}
          open={quoteVisible}
          onOpenChange={(flag) => {
            setQuoteVisible(flag);
          }}
          onFinish={saveOrderQuote}
        >
          <ProFormDigit
            name="quote_price"
            label={intl.formatMessage({
              id: 'plugin.order.communication.quote-price',
            })}
            fieldProps={{
              precision: 2,
            }}
          />
          <ProFormTextArea
            name="quote_content"
            label={intl.formatMessage({
              id: 'plugin.order.communication.quote-content',
            })}
          />
        </ModalForm>
      )}
      {viewOrderVisible && (
        <OrderForm
          open={viewOrderVisible}
          order={viewOrder}
          onCancel={() => {
            setViewOrderVisible(false);
          }}
          onSubmit={async () => {
            setViewOrderVisible(false);
          }}
        />
      )}
    </PageContainer>
  );
};

export default PluginCommunicationOrder;
