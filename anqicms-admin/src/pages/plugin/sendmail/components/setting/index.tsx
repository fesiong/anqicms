import {
  pluginGetSendmailSetting,
  pluginSaveSendmailSetting,
} from '@/services/plugin/sendmail';
import {
  ModalForm,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { message } from 'antd';
import React, { useState } from 'react';

export type SendmailSettingProps = {
  open: boolean;
  onCancel: (flag?: boolean) => void;
};

const SendmailSetting: React.FC<SendmailSettingProps> = (props) => {
  const [useBrevo, setUseBrevo] = useState<boolean>(false);
  const intl = useIntl();

  const handleSubmit = async (values: any) => {
    values.port = Number(values.port);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveSendmailSetting(values)
      .then((res) => {
        message.info(res.msg);
        props.onCancel(false);
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <ModalForm
      width={800}
      title={intl.formatMessage({ id: 'plugin.sendmail.setting' })}
      request={async () => {
        let res = await pluginGetSendmailSetting();
        setUseBrevo(res.data?.use_brevo || false);
        return res.data;
      }}
      open={props.open}
      layout="horizontal"
      onOpenChange={(flag) => {
        if (!flag) {
          props.onCancel(flag);
        }
      }}
      onFinish={async (values) => {
        handleSubmit(values);
      }}
    >
      <ProFormText
        name="server"
        required
        label={intl.formatMessage({ id: 'plugin.sendmail.server' })}
        extra={intl.formatMessage({
          id: 'plugin.sendmail.server.description',
        })}
      />
      <ProFormRadio.Group
        name="use_ssl"
        label={intl.formatMessage({ id: 'plugin.sendmail.use-ssl' })}
        options={[
          {
            label: intl.formatMessage({ id: 'plugin.sendmail.use-ssl.no' }),
            value: 0,
          },
          { label: 'SSL', value: 1 },
          { label: 'TLS', value: 2 },
        ]}
      />
      <ProFormText
        name="port"
        label={intl.formatMessage({ id: 'plugin.sendmail.port' })}
        extra={intl.formatMessage({ id: 'plugin.sendmail.port.description' })}
      />
      <ProFormText
        name="account"
        label={intl.formatMessage({ id: 'plugin.sendmail.account' })}
        extra={intl.formatMessage({
          id: 'plugin.sendmail.account.description',
        })}
      />
      <ProFormText
        name="password"
        label={intl.formatMessage({ id: 'plugin.sendmail.password' })}
        extra={intl.formatMessage({
          id: 'plugin.sendmail.password.description',
        })}
      />
      <ProFormText
        name="recipient"
        label={intl.formatMessage({ id: 'plugin.sendmail.recipient' })}
        extra={intl.formatMessage({
          id: 'plugin.sendmail.recipient.description',
        })}
      />
      <ProFormRadio.Group
        name="use_brevo"
        label={intl.formatMessage({ id: 'plugin.sendmail.use-brevo' })}
        options={[
          {
            label: intl.formatMessage({
              id: 'plugin.sendmail.use-brevo.no',
            }),
            value: false,
          },
          {
            label: intl.formatMessage({
              id: 'plugin.sendmail.use-brevo.yes',
            }),
            value: true,
          },
        ]}
        extra={intl.formatMessage({
          id: 'plugin.sendmail.use-brevo.description',
        })}
        fieldProps={{
          onChange: (e) => setUseBrevo(e.target.value),
        }}
      />
      {useBrevo && (
        <ProFormText
          name="brevo_api_key"
          label={intl.formatMessage({ id: 'plugin.sendmail.brevo-api-key' })}
        />
      )}
    </ModalForm>
  );
};

export default SendmailSetting;
