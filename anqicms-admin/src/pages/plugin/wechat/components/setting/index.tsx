import { pluginGetWechatConfig, pluginSaveWechatConfig } from '@/services';
import { ModalForm, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Divider, message } from 'antd';
import React, { useState } from 'react';

const PluginWechatSetting: React.FC<any> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const intl = useIntl();

  const onSubmit = async (values: any) => {
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginSaveWechatConfig(values)
      .then((res) => {
        message.success(res.msg);
        setVisible(false);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
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
        open={visible}
        request={async () => {
          const res = await pluginGetWechatConfig();
          return res.data || null;
        }}
        onOpenChange={(flag) => {
          setVisible(flag);
        }}
        onFinish={onSubmit}
        title={intl.formatMessage({ id: 'plugin.wechat.setting' })}
      >
        <ProFormText name="server_url" label={intl.formatMessage({ id: 'plugin.weapp.server-url' })} width="lg" readonly />
        <ProFormText name="app_id" label={intl.formatMessage({ id: 'plugin.wechat.appid' })} width="lg" />
        <ProFormText name="app_secret" label={intl.formatMessage({ id: 'plugin.wechat.app-secret' })} width="lg" />
        <ProFormText name="token" label={intl.formatMessage({ id: 'plugin.weapp.token' })} width="lg" />
        <ProFormText
          name="encoding_aes_key"
          label={intl.formatMessage({ id: 'plugin.weapp.encoding-aes-key' })}
          width="lg"
          extra={intl.formatMessage({ id: 'plugin.weapp.encoding-aes-key.description' })}
        />
        <Divider><FormattedMessage id="plugin.wechat.verify-setting" /></Divider>
        <ProFormText
          name="verify_key"
          label={intl.formatMessage({ id: 'plugin.wechat.verify-key' })}
          width="lg"
          placeholder={intl.formatMessage({ id: 'plugin.wechat.verify-key.placeholder' })}
          extra={intl.formatMessage({ id: 'plugin.wechat.verify-key.description' })}
        />
        <ProFormText
          name="verify_msg"
          label={intl.formatMessage({ id: 'plugin.wechat.verify-msg' })}
          width="lg"
          placeholder={intl.formatMessage({ id: 'plugin.wechat.verify-msg.placeholder' })}
          extra={intl.formatMessage({ id: 'plugin.wechat.verify-msg.description' })}
        />
      </ModalForm>
    </>
  );
};

export default PluginWechatSetting;
