import { pluginSaveCoupon } from '@/services';
import {
  ModalForm,
  ProFormDateTimePicker,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

export type CouponFormProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (flag?: boolean) => Promise<void>;
  open: boolean;
  coupon: any;
};

const CouponForm: React.FC<CouponFormProps> = (props) => {
  const [loading, setLoading] = useState(false);
  const [moneyType, setMoneyType] = useState(0);
  const [hasMinimim, setHasMinimim] = useState(false);
  const [coupon, setCoupon] = useState({});
  const [fetched, setFetched] = useState(false);
  const intl = useIntl();

  useEffect(() => {
    setMoneyType(props.coupon?.money_type || 0);
    setHasMinimim(props.coupon?.need_money > 0);
    const coupon1 = Object.assign({}, props.coupon);
    coupon1.percent = coupon1.money_type === 2 ? coupon1.money : 0;
    coupon1.money = coupon1.money > 0 ? coupon1.money / 100 : 0;
    coupon1.max_money = coupon1.max_money > 0 ? coupon1.max_money / 100 : 0;
    coupon1.has_minimum = coupon1.need_money > 0 ? true : false;
    coupon1.need_money = coupon1.need_money > 0 ? coupon1.need_money / 100 : 0;
    coupon1.start_moment =
      coupon1.start_time > 0 ? dayjs(coupon1.start_time * 1000) : null;
    coupon1.end_moment =
      coupon1.end_time > 0 ? dayjs(coupon1.end_time * 1000) : null;

    setCoupon(coupon1);
    setFetched(true);
  }, []);
  const onSubmit = async (values: any) => {
    if (loading) return;
    setLoading(true);
    const hide = message.loading(
      {
        content: intl.formatMessage({ id: 'setting.system.submitting' }),
        key: 'loading',
      },
      0,
    );

    let coupon = Object.assign(props.coupon, values);
    coupon.money = Number((coupon.money * 100).toFixed(0));
    coupon.max_money = Number((coupon.max_money * 100).toFixed(0));
    if (coupon.has_minimum) {
      coupon.need_money = Number((coupon.need_money * 100).toFixed(0));
    } else {
      coupon.need_money = 0;
    }
    if (coupon.money_type === 2) {
      coupon.money = Number(coupon.percent);
    }
    await pluginSaveCoupon(coupon);
    hide();
    setLoading(false);
    props.onSubmit();
  };

  return (
    <>
      {fetched && (
        <ModalForm
          width={800}
          title={
            props.coupon?.id
              ? intl.formatMessage({ id: 'plugin.coupon.edit' })
              : intl.formatMessage({ id: 'plugin.coupon.add' })
          }
          initialValues={coupon}
          open={props.open}
          //layout="horizontal"
          onOpenChange={(flag) => {
            if (!flag) {
              props.onCancel(flag);
            }
          }}
          onFinish={async (values) => {
            onSubmit(values);
          }}
        >
          <ProFormText
            name="title"
            label={intl.formatMessage({ id: 'plugin.coupon.title' })}
          />
          <ProFormTextArea
            name="description"
            label={intl.formatMessage({ id: 'plugin.coupon.description' })}
          />
          <ProFormDigit
            name="expire"
            label={intl.formatMessage({ id: 'plugin.coupon.expire' })}
            fieldProps={{
              precision: 0,
              suffix: intl.formatMessage({ id: 'plugin.coupon.expire.suffix' }),
            }}
          />
          <ProFormDateTimePicker
            name="start_moment"
            label={intl.formatMessage({ id: 'plugin.coupon.start-time' })}
            transform={(value) => {
              return {
                start_time: value ? dayjs(value).unix() : 0,
              };
            }}
          />
          <ProFormDateTimePicker
            name="end_moment"
            label={intl.formatMessage({ id: 'plugin.coupon.end-time' })}
            transform={(value) => {
              return {
                end_time: value ? dayjs(value).unix() : 0,
              };
            }}
          />
          <ProFormSelect
            label={intl.formatMessage({ id: 'plugin.coupon.type' })}
            name="type"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.register',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.type.limited' }),
                value: 2,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.type.order' }),
                value: 3,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.discount',
                }),
                value: 4,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.unlimited',
                }),
                value: 5,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.integral',
                }),
                value: 6,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.type.activity',
                }),
                value: 7,
              },
            ]}
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.coupon.status' })}
            name="status"
            options={[
              {
                label: intl.formatMessage({ id: 'plugin.coupon.status.hide' }),
                value: 0,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.status.show' }),
                value: 1,
              },
            ]}
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.coupon.money-type' })}
            name="money_type"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.money-type.fixed',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.money-type.range',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.money-type.percent',
                }),
                value: 2,
              },
            ]}
            fieldProps={{
              onChange: (value) => {
                setMoneyType(value.target.value);
              },
            }}
          />
          {(moneyType === 0 || moneyType === 1) && (
            <ProFormDigit
              name="money"
              label={intl.formatMessage({ id: 'plugin.coupon.money' })}
              min={0}
              fieldProps={{
                precision: 0,
              }}
            />
          )}
          {moneyType === 2 && (
            <ProFormDigit
              name="percent"
              label={intl.formatMessage({ id: 'plugin.coupon.percent' })}
              min={0}
              fieldProps={{
                precision: 0,
                suffix: '%',
              }}
            />
          )}
          {(moneyType === 1 || moneyType === 2) && (
            <ProFormDigit
              name="max_money"
              label={intl.formatMessage({
                id: 'plugin.coupon.max-money',
              })}
              rules={[
                {
                  required: true,
                  message: intl.formatMessage({
                    id: 'plugin.coupon.money.required',
                  }),
                },
              ]}
            />
          )}
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.coupon.has-minimum' })}
            name="has_minimum"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.has-minimum.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.has-minimum.yes',
                }),
                value: true,
              },
            ]}
            fieldProps={{
              onChange: (e) => {
                setHasMinimim(e.target.value);
              },
            }}
          />
          {hasMinimim && (
            <ProFormDigit
              name="need_money"
              label={intl.formatMessage({
                id: 'plugin.coupon.need-money',
              })}
              fieldProps={{
                precision: 2,
              }}
            />
          )}
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.coupon.rule' })}
            name="range_type"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.rule.all-product',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({ id: 'plugin.coupon.rule.product' }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.coupon.rule.not-product',
                }),
                value: 2,
              },
            ]}
          />
        </ModalForm>
      )}
    </>
  );
};

export default CouponForm;
