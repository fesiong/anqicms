import { pluginGetOrderSetting, pluginSaveOrderSetting } from '@/services';
import {
  ModalForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormRadio,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useState } from 'react';

export type OrderSettingProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const OrderSetting: React.FC<OrderSettingProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleSaveSetting = async (values: any) => {
    const res = await pluginSaveOrderSetting(values);

    if (res.code === 0) {
      message.success(res.msg);
      setVisible(false);
    } else {
      message.error(res.msg);
    }
  };

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <ModalForm
        width={600}
        title={intl.formatMessage({ id: 'plugin.order.setting' })}
        open={visible}
        // modalProps={{
        //   onCancel: () => {
        //     setVisible(false);
        //   },
        // }}
        onOpenChange={(flag) => {
          setVisible(flag);
        }}
        request={async () => {
          const res = await pluginGetOrderSetting();
          return res.data;
        }}
        layout="horizontal"
        onFinish={async (values) => {
          handleSaveSetting(values);
        }}
      >
        <ProFormCheckbox
          name="no_need_login"
          label={intl.formatMessage({
            id: 'plugin.order.setting.no-need-login',
          })}
          extra={intl.formatMessage({
            id: 'plugin.order.setting.no-need-login.description',
          })}
        >
          {intl.formatMessage({
            id: 'plugin.order.setting.no-need-login',
          })}
        </ProFormCheckbox>
        <ProFormRadio.Group
          name="no_process"
          label={intl.formatMessage({ id: 'plugin.order.setting.progress' })}
          options={[
            {
              label: intl.formatMessage({
                id: 'plugin.order.setting.progress.yes',
              }),
              value: false,
            },
            {
              label: intl.formatMessage({
                id: 'plugin.order.setting.progress.no',
              }),
              value: true,
            },
          ]}
          extra={intl.formatMessage({
            id: 'plugin.order.setting.progress.description',
          })}
        />
        <ProFormDigit
          label={intl.formatMessage({ id: 'plugin.order.setting.auto-finish' })}
          name="auto_finish_day"
          extra={intl.formatMessage({
            id: 'plugin.order.setting.auto-finish.placeholder',
          })}
          fieldProps={{
            precision: 0,
            addonAfter: intl.formatMessage({
              id: 'plugin.order.setting.auto-finish.suffix',
            }),
          }}
        />
        <ProFormDigit
          label={intl.formatMessage({ id: 'plugin.order.setting.auto-close' })}
          name="auto_close_minute"
          extra={intl.formatMessage({
            id: 'plugin.order.setting.auto-close.description',
          })}
          fieldProps={{
            precision: 0,
            addonAfter: intl.formatMessage({
              id: 'plugin.order.setting.auto-close.suffix',
            }),
          }}
        />
        <ProFormDigit
          label={intl.formatMessage({
            id: 'plugin.order.setting.seller-percent',
          })}
          name="seller_percent"
          extra={intl.formatMessage({
            id: 'plugin.order.setting.seller-percent.description',
          })}
          fieldProps={{ precision: 0, addonAfter: '%' }}
        />
      </ModalForm>
    </>
  );
};

export default OrderSetting;
