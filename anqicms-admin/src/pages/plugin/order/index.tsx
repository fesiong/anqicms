import {
  pluginExportOrder,
  pluginGetOrders,
  pluginOrderApplyRefund,
  pluginSetOrderDelivery,
  pluginSetOrderFinished,
  pluginSetOrderPay,
  pluginSetOrderRefund,
} from '@/services';
import { exportFile } from '@/utils';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDatePicker,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { AutoComplete, Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import OrderForm from './components/orderForm';
import OrderSetting from './setting';

const PluginOrder: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentOrder, setcurrentOrder] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [deliverVisible, setDeliverVisible] = useState<boolean>(false);
  const [refundVisible, setRefundVisible] = useState<boolean>(false);
  const [exportVisible, setExportVisible] = useState<boolean>(false);
  const [payVisible, setPayVisible] = useState<boolean>(false);
  const [status, setStatus] = useState<string>('');
  const intl = useIntl();

  const exportOrder = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'plugin.order.loading' }),
      0,
    );
    if (values.start_date) {
      values.start_time = dayjs(values.start_date).unix();
    }
    if (values.start_date) {
      values.end_time = dayjs(values.end_date).unix() + 86400;
    }
    pluginExportOrder(values)
      .then((res) => {
        exportFile(res.data?.header, res.data?.content, 'xlsx');
      })
      .finally(() => {
        hide();
      });
  };

  const handleEditOrder = async (record: any) => {
    setcurrentOrder(record);
    setEditVisible(true);
  };

  const handleSetDelivery = (record: any) => {
    setcurrentOrder(record);
    setDeliverVisible(true);
  };

  const saveOrderDeliver = async (values: any) => {
    values.order_id = currentOrder.order_id;
    pluginSetOrderDelivery(values).then((res) => {
      message.info(res.msg);
      actionRef.current?.reload();
      setDeliverVisible(false);
    });
  };

  const saveOrderRefund = async (values: any) => {
    values.order_id = currentOrder.order_id;
    pluginSetOrderRefund(values).then((res) => {
      message.info(res.msg);
      actionRef.current?.reload();
      setDeliverVisible(false);
    });
  };

  const handleSetFinished = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.order.finish.confirm' }),
      content: intl.formatMessage({ id: 'plugin.order.finish.content' }),
      onOk: () => {
        pluginSetOrderFinished(record).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleSetRefund = (record: any) => {
    setcurrentOrder(record);
    setRefundVisible(true);
  };

  const handleApplyRefund = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.order.apply-refund.confirm' }),
      content: intl.formatMessage({ id: 'plugin.order.apply-refund.content' }),
      onOk: () => {
        pluginOrderApplyRefund(record).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleSetPay = (record: any) => {
    setcurrentOrder(record);
    setPayVisible(true);
  };

  const saveOrderPaid = async (values: any) => {
    values.order_id = currentOrder.order_id;
    pluginSetOrderPay(values).then((res) => {
      setPayVisible(false);
      message.info(res.msg);
      actionRef.current?.reload();
    });
  };

  const statusItems = [
    {
      label: intl.formatMessage({ id: 'plugin.order.status.all' }),
      value: '',
    },
    {
      label: intl.formatMessage({ id: 'plugin.order.status.wait' }),
      value: 'waiting',
    },
    {
      label: intl.formatMessage({ id: 'plugin.order.status.paid' }),
      value: 'paid',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.order.status.delivery',
      }),
      value: 'delivery',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.order.status.finished',
      }),
      value: 'finished',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.order.status.refunding',
      }),
      value: 'refunding',
    },
    {
      label: intl.formatMessage({
        id: 'plugin.order.status.closed',
      }),
      value: 'closed',
    },
  ];

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.order.order-id' }),
      dataIndex: 'order_id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.buy.user-name' }),
      dataIndex: 'user.user_name',
      key: 'user_name',
      render: (dom: any, entity) => {
        return entity.user?.user_name || '-';
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.order-amount' }),
      dataIndex: 'amount',
      hideInSearch: true,
      render: (dom: any) => {
        return dom / 100;
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
      title: intl.formatMessage({ id: 'plugin.order.pay-time' }),
      dataIndex: 'paid_time',
      hideInSearch: true,
      render: (_, entity) => {
        return entity.paid_time > 0
          ? dayjs(entity.paid_time * 1000).format('YYYY-MM-DD HH:mm')
          : '-';
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.order.status.wait' }),
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.order.status.paid' }),
        },
        2: {
          text: intl.formatMessage({ id: 'plugin.order.status.delivery' }),
        },
        3: {
          text: intl.formatMessage({ id: 'plugin.order.status.finished' }),
          status: 'Success',
        },
        8: {
          text: intl.formatMessage({ id: 'plugin.order.status.refunding' }),
        },
        9: {
          text: intl.formatMessage({ id: 'plugin.order.status.refunded' }),
        },
        '-1': {
          text: intl.formatMessage({ id: 'plugin.order.status.closed' }),
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          {record.status === 1 && (
            <a
              onClick={() => {
                handleSetDelivery(record);
              }}
            >
              <FormattedMessage id="plugin.order.delivery" />
            </a>
          )}
          {record.status === 2 && (
            <a
              onClick={() => {
                handleSetFinished(record);
              }}
            >
              <FormattedMessage id="plugin.order.finish-order" />
            </a>
          )}
          {record.status === 8 && (
            <a
              onClick={() => {
                handleSetRefund(record);
              }}
            >
              <FormattedMessage id="plugin.order.refund-process" />
            </a>
          )}
          {(record.status === 1 ||
            record.status === 2 ||
            record.status === 3) && (
            <a
              onClick={() => {
                handleApplyRefund(record);
              }}
            >
              <FormattedMessage id="plugin.order.apply-refund" />
            </a>
          )}
          {record.status === 0 && (
            <a
              onClick={() => {
                handleSetPay(record);
              }}
            >
              <FormattedMessage id="plugin.order.pay" />
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
        headerTitle={
          <>
            <div className="module-tags">
              {statusItems.map((item: any) => (
                <div
                  className={
                    'module-tag ' + (item.value === status ? 'active' : '')
                  }
                  key={item.value || 'all'}
                  onClick={() => {
                    setStatus(item.value);
                    actionRef.current?.reload();
                  }}
                >
                  {item.label}
                </div>
              ))}
            </div>
          </>
        }
        actionRef={actionRef}
        rowKey="id"
        search={{
          span: { xs: 24, sm: 12, md: 8, lg: 8, xl: 6, xxl: 6 },
        }}
        toolBarRender={() => [
          <Button
            key="export"
            onClick={() => {
              setExportVisible(true);
            }}
          >
            <FormattedMessage id="plugin.order.export" />
          </Button>,
          <OrderSetting key="setting" onCancel={() => {}}>
            <Button>
              <FormattedMessage id="plugin.order.setting" />
            </Button>
          </OrderSetting>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          params.status = status;
          return pluginGetOrders(params);
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
        <OrderForm
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
      {deliverVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.order.delivery-process' })}
          width={480}
          open={deliverVisible}
          onOpenChange={(flag) => {
            setDeliverVisible(flag);
          }}
          onFinish={saveOrderDeliver}
        >
          <ProFormText
            name="express_company"
            label={intl.formatMessage({ id: 'plugin.order.express-company' })}
          >
            <AutoComplete
              options={[
                {
                  value: '',
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.empty',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.sf',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.sf',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.ems',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.ems',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.jd',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.jd',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.sto',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.sto',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.yto',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.yto',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.zto',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.zto',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.yunda',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.yunda',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.jitu',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.jitu',
                  }),
                },
                {
                  value: intl.formatMessage({
                    id: 'plugin.order.express-company.baishi',
                  }),
                  label: intl.formatMessage({
                    id: 'plugin.order.express-company.baishi',
                  }),
                },
              ]}
            />
          </ProFormText>
          <ProFormText
            name="tracking_number"
            label={intl.formatMessage({ id: 'plugin.order.tracking-number' })}
          />
        </ModalForm>
      )}
      {refundVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.order.refund-process' })}
          open={refundVisible}
          onOpenChange={(flag) => {
            setRefundVisible(flag);
          }}
          onFinish={saveOrderRefund}
        >
          <ProFormRadio.Group
            name="status"
            label={intl.formatMessage({ id: 'plugin.order.refund' })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({
                  id: 'plugin.order.refund.disagree',
                }),
              },
              {
                value: 1,
                label: intl.formatMessage({ id: 'plugin.order.refund.agree' }),
              },
            ]}
          />
        </ModalForm>
      )}
      {exportVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.order.export' })}
          width={480}
          open={exportVisible}
          onOpenChange={(flag) => {
            setExportVisible(flag);
          }}
          onFinish={exportOrder}
        >
          <ProFormSelect
            name="status"
            label={intl.formatMessage({ id: 'plugin.order.export.status' })}
            initialValue={'paid'}
            request={async () => {
              return [
                {
                  label: intl.formatMessage({ id: 'plugin.order.status.all' }),
                  value: '',
                },
                {
                  label: intl.formatMessage({ id: 'plugin.order.status.wait' }),
                  value: 'waiting',
                },
                {
                  label: intl.formatMessage({ id: 'plugin.order.status.paid' }),
                  value: 'paid',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.status.delivery',
                  }),
                  value: 'delivery',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.status.finished',
                  }),
                  value: 'finished',
                },
                {
                  label: intl.formatMessage({
                    id: 'plugin.order.status.refunding',
                  }),
                  value: 'refunding',
                },
              ];
            }}
          />
          <ProFormDatePicker
            name="start_date"
            label={intl.formatMessage({ id: 'plugin.order.export.start-date' })}
            width={'lg'}
          />
          <ProFormDatePicker
            name="end_date"
            label={intl.formatMessage({ id: 'plugin.order.export.end-date' })}
            width={'lg'}
            extra={intl.formatMessage({
              id: 'plugin.order.export.end-date.description',
            })}
          />
        </ModalForm>
      )}
      {payVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.order.pay-process' })}
          open={payVisible}
          onOpenChange={(flag) => {
            setPayVisible(flag);
          }}
          width={480}
          onFinish={saveOrderPaid}
        >
          <ProFormRadio.Group
            name="pay_way"
            label={intl.formatMessage({ id: 'plugin.order.pay-way' })}
            options={[
              {
                value: 'offline',
                label: intl.formatMessage({
                  id: 'plugin.order.pay-way.offline',
                }),
              },
            ]}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default PluginOrder;
