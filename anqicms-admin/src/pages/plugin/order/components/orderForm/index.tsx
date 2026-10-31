import {
  pluginGetOrderInfo,
  pluginOrderApplyRefund,
  pluginSetOrderDelivery,
  pluginSetOrderFinished,
  pluginSetOrderPay,
  pluginSetOrderRefund,
} from '@/services';
import {
  ModalForm,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { AutoComplete, Divider, Image, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';
import './index.less';

export type OrderFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  order: any;
};

const OrderForm: React.FC<OrderFormProps> = (props) => {
  const [order, setOrder] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);

  const [deliverVisible, setDeliverVisible] = useState<boolean>(false);
  const [refundVisible, setRefundVisible] = useState<boolean>(false);
  const [payVisible, setPayVisible] = useState<boolean>(false);
  const intl = useIntl();

  const getOrderDetail = async () => {
    const res = await pluginGetOrderInfo({
      order_id: props.order.order_id,
    });
    setOrder(res.data || null);
    setFetched(true);
  };

  useEffect(() => {
    getOrderDetail();
  }, []);

  const handleSetDelivery = () => {
    setDeliverVisible(true);
  };

  const saveOrderDeliver = async (values: any) => {
    values.order_id = order.order_id;
    pluginSetOrderDelivery(values).then((res) => {
      message.info(res.msg);
      getOrderDetail();
      setDeliverVisible(false);
    });
  };

  const saveOrderRefund = async (values: any) => {
    values.order_id = order.order_id;
    pluginSetOrderRefund(values).then((res) => {
      message.info(res.msg);
      setDeliverVisible(false);
      getOrderDetail();
    });
  };

  const handleSetFinished = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.order.finish.confirm' }),
      content: intl.formatMessage({ id: 'plugin.order.finish.content' }),
      onOk: () => {
        pluginSetOrderFinished(order).then((res) => {
          message.info(res.msg);
          getOrderDetail();
        });
      },
    });
  };

  const handleSetRefund = () => {
    setRefundVisible(true);
  };

  const handleApplyRefund = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.order.apply-refund.confirm' }),
      content: intl.formatMessage({ id: 'plugin.order.apply-refund.content' }),
      onOk: () => {
        pluginOrderApplyRefund(order).then((res) => {
          message.info(res.msg);
          getOrderDetail();
        });
      },
    });
  };

  const handleSetPay = () => {
    setPayVisible(true);
  };

  const saveOrderPaid = async (values: any) => {
    values.order_id = order.order_id;
    pluginSetOrderPay(values).then((res) => {
      setPayVisible(false);
      message.info(res.msg);
      getOrderDetail();
    });
  };

  const getStatusText = (status: number) => {
    let text = '';
    switch (status) {
      case 0:
        text = intl.formatMessage({ id: 'plugin.order.status.wait' });
        break;
      case 1:
        text = intl.formatMessage({ id: 'plugin.order.status.paid' });
        break;
      case 2:
        text = intl.formatMessage({ id: 'plugin.order.status.delivery' });
        break;
      case 3:
        text = intl.formatMessage({ id: 'plugin.order.status.finished' });
        break;
      case 8:
        text = intl.formatMessage({ id: 'plugin.order.status.refunding' });
        break;
      case 9:
        text = intl.formatMessage({ id: 'plugin.order.status.refunded' });
        break;
      case -1:
        text = intl.formatMessage({ id: 'plugin.order.status.closed' });
        break;
    }

    return text;
  };

  return (
    <>
      {fetched ? (
        <ModalForm
          width={900}
          title={intl.formatMessage({ id: 'plugin.order.detail' })}
          open={props.open}
          layout="horizontal"
          onOpenChange={(flag) => {
            if (!flag) {
              props.onCancel(flag);
            }
          }}
          submitter={false}
        >
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.type' })}
            readonly
          >
            {order.type === 'vip'
              ? intl.formatMessage({ id: 'plugin.order.type.vip' })
              : order.type === 'communication'
              ? intl.formatMessage({ id: 'plugin.order.type.communication' })
              : intl.formatMessage({ id: 'plugin.order.type.goods' })}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.order-id' })}
            readonly
          >
            {order.order_id}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.status' })}
            readonly
          >
            <p>{getStatusText(order.status)}</p>
            <Space size={20}>
              {order.status === 1 && (
                <a
                  onClick={() => {
                    handleSetDelivery();
                  }}
                >
                  <FormattedMessage id="plugin.order.delivery" />
                </a>
              )}
              {order.status === 2 && (
                <a
                  onClick={() => {
                    handleSetFinished();
                  }}
                >
                  <FormattedMessage id="plugin.order.finish-order" />
                </a>
              )}
              {order.status === 8 && (
                <a
                  onClick={() => {
                    handleSetRefund();
                  }}
                >
                  <FormattedMessage id="plugin.order.refund-process" />
                </a>
              )}
              {(order.status === 1 ||
                order.status === 2 ||
                order.status === 3) && (
                <a
                  onClick={() => {
                    handleApplyRefund();
                  }}
                >
                  <FormattedMessage id="plugin.order.apply-refund" />
                </a>
              )}
              {order.status === 0 && (
                <a
                  onClick={() => {
                    handleSetPay();
                  }}
                >
                  <FormattedMessage id="plugin.order.pay" />
                </a>
              )}
            </Space>
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.create-time' })}
            readonly
          >
            {dayjs(order.created_time * 1000).format('YYYY-MM-DD HH:mm:ss')}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.pay-time' })}
            readonly
          >
            {order.paid_time > 0
              ? dayjs(order.paid_time * 1000).format('YYYY-MM-DD HH:mm:ss')
              : '-'}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.deliver-time' })}
            readonly
          >
            {order.deliver_time > 0
              ? dayjs(order.deliver_time * 1000).format('YYYY-MM-DD HH:mm:ss')
              : '-'}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.finished-time' })}
            readonly
          >
            {order.finished_time > 0
              ? dayjs(order.finished_time * 1000).format('YYYY-MM-DD HH:mm:ss')
              : '-'}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.payment-id' })}
            readonly
          >
            {order.payment_id}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.terrace-id' })}
            readonly
          >
            {order.terrace_id}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.origin-amount' })}
            readonly
          >
            {order.origin_amount / 100}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.order.detail.discount-amount',
            })}
            readonly
          >
            <Space size={10} direction="vertical">
              <div className="text-muted">
                {order.discount_amount > 0 ? order.discount_amount / 100 : 0}
              </div>
              {order.user_discount > 0 && (
                <div className="text-muted ml-normal">
                  {intl.formatMessage({
                    id: 'plugin.order.detail.user-discount',
                  })}
                  {': '}
                  {order.user_discount / 100}
                </div>
              )}
              {order.coupon_discount > 0 && (
                <div className="text-muted ml-normal">
                  {intl.formatMessage({
                    id: 'plugin.order.detail.coupon-discount',
                  })}
                  {': '}
                  {order.coupon_discount / 100}
                </div>
              )}
              {order.shipping_discount > 0 && (
                <div className="text-muted ml-normal">
                  {intl.formatMessage({
                    id: 'plugin.order.detail.shipping-discount',
                  })}
                  {': '}
                  {order.shipping_discount / 100}
                </div>
              )}
            </Space>
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.order.detail.shipping-fee',
            })}
            readonly
          >
            {order.shipping_fee > 0 ? order.shipping_fee / 100 : 0}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.pay-amount' })}
            readonly
          >
            {order.amount / 100}
          </ProFormText>
          {order.service_fee > 0 && (
            <>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.order.service-fee',
                })}
                readonly
              >
                {order.service_fee / 100}
              </ProFormText>
              {order.services?.map((service: any) => (
                <ProFormText
                  key={service.id}
                  label={
                    service.order_service?.service_name || service.service_code
                  }
                  extra={service.order_service?.description}
                  readonly
                >
                  {service.service_fee / 100}
                </ProFormText>
              ))}
            </>
          )}
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.buy.user-name' })}
            readonly
          >
            {order.user?.user_name} ({order.user_id})
          </ProFormText>
          {order.share_user_id > 0 && (
            <>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.order.share.user-name',
                })}
                readonly
              >
                {order.share_user?.user_name}
              </ProFormText>
              <ProFormText
                label={intl.formatMessage({ id: 'plugin.order.share.amount' })}
                readonly
              >
                {order.share_amount / 100}
              </ProFormText>
            </>
          )}
          {order.share_parent_user_id > 0 && (
            <>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.order.share.parent.user-name',
                })}
                readonly
              >
                {order.parent_user?.user_name}
              </ProFormText>
              <ProFormText
                label={intl.formatMessage({
                  id: 'plugin.order.share.parent.amount',
                })}
                readonly
              >
                {order.share_parent_amount / 100}
              </ProFormText>
            </>
          )}
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.remark' })}
            readonly
          >
            {order.remark}
          </ProFormText>
          <Divider />
          {order.type === 'vip' ? (
            <div className="order-details">
              <div className="order-details-title">
                <FormattedMessage id="plugin.order.vip" />
              </div>
              {order.details?.map((item: any, index: number) => (
                <div className="order-details-item" key={index}>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.title',
                    })}
                    readonly
                  >
                    {item.group?.title}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.price',
                    })}
                    readonly
                  >
                    {item.price / 100}
                  </ProFormText>
                </div>
              ))}
            </div>
          ) : (
            <div className="order-details">
              <div className="order-details-title">
                <FormattedMessage id="plugin.order.goods" />
              </div>
              {order.details?.map((item: any, index: number) => (
                <div className="order-details-item" key={index}>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.thumb',
                    })}
                    readonly
                  >
                    <Image
                      className="img"
                      height={100}
                      preview={{
                        src: item.communication_order
                          ? item.communication_order.logo
                          : item.goods?.logo,
                      }}
                      src={
                        item.communication_order
                          ? item.communication_order.thumb
                          : item.goods?.thumb
                      }
                    />
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.title',
                    })}
                    readonly
                  >
                    {item.communication_order
                      ? item.communication_order.title
                      : item.goods?.title}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.sku',
                    })}
                    readonly
                  >
                    {item.sku_values?.map((sv: any) => sv.title).join(' / ') ||
                      item.goods_sku?.title}
                  </ProFormText>
                  {item.order_fields && (
                    <div
                      className="order-fields"
                      style={{ border: '1px solid #eee', padding: '10px' }}
                    >
                      <h3>Custom Fields</h3>
                      {item.order_fields?.map((field: any, index: number) => (
                        <ProFormText key={index} label={field.name} readonly>
                          {field.type === 'texts' ? (
                            <div className="text-groups">
                              {item.order_extra?.[field.field_name].map(
                                (text: any, index: number) => (
                                  <div className="text-group" key={index}>
                                    <div className="text-key">{text.key}: </div>
                                    <div className="text-value">
                                      {text.value}
                                    </div>
                                  </div>
                                ),
                              )}
                            </div>
                          ) : field.type === 'image' ? (
                            <Image
                              height={100}
                              preview={{
                                src: item.order_extra?.[field.field_name],
                              }}
                              src={item.order_extra?.[field.field_name]}
                            />
                          ) : field.type === 'file' ? (
                            <a href={item.order_extra?.[field.field_name]}>
                              {item.order_extra?.[field.field_name]}
                            </a>
                          ) : field.type === 'images' ? (
                            <div>
                              {item.order_extra?.[field.field_name].map(
                                (text: any, index: number) => (
                                  <Image
                                    key={index}
                                    height={100}
                                    preview={{ src: text }}
                                    src={text}
                                  />
                                ),
                              )}
                            </div>
                          ) : (
                            <div>{item.order_extra?.[field.field_name]}</div>
                          )}
                        </ProFormText>
                      ))}
                    </div>
                  )}
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.price',
                    })}
                    readonly
                  >
                    {item.goods?.price / 100}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.quantity',
                    })}
                    readonly
                  >
                    {item.quantity}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.origin-amount',
                    })}
                    readonly
                  >
                    {item.origin_amount / 100}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.discount-amount',
                    })}
                    readonly
                  >
                    {(item.origin_amount - item.amount) / 100}
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.order.detail.amount',
                    })}
                    readonly
                  >
                    {item.amount / 100}
                  </ProFormText>
                </div>
              ))}
            </div>
          )}
          <Divider />
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.recipient.name' })}
            readonly
          >
            {order.order_address?.name}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.recipient.company' })}
            readonly
          >
            {order.order_address?.company}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.recipient.contact' })}
            readonly
          >
            {order.order_address?.phone}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.recipient.email' })}
            readonly
          >
            {order.order_address?.email}
          </ProFormText>
          <ProFormText
            label={intl.formatMessage({ id: 'plugin.order.recipient.address' })}
            readonly
          >
            {order.order_address?.country}
            {order.order_address?.province}
            {order.order_address?.city}
            {order.order_address?.town}
            {order.order_address?.address_info}
            {', '}
            {order.order_address?.postcode}
          </ProFormText>
        </ModalForm>
      ) : (
        <div></div>
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
    </>
  );
};

export default OrderForm;
