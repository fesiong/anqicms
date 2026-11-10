import { pluginGetWeappConfig, pluginSaveWeappConfig } from '@/services';
import { PageContainer, ProForm, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Divider, message } from 'antd';
import React from 'react';

const PluginWeapp: React.FC<any> = () => {
  const intl = useIntl();
  const onSubmit = async (values: any) => {
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginSaveWeappConfig(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <PageContainer>
      <Card>
        <ProForm
          request={async () => {
            const res = await pluginGetWeappConfig();
            return res.data || [];
          }}
          onFinish={onSubmit}
          title={intl.formatMessage({ id: 'menu.plugin.channel.weapp' })}
        >
          <ProFormText name="app_id" label={intl.formatMessage({ id: 'plugin.weapp.appid' })} width="lg" />
          <ProFormText name="app_secret" label={intl.formatMessage({ id: 'plugin.weapp.app-secret' })} width="lg" />
          <Divider><FormattedMessage id="plugin.weapp.push.setting" /></Divider>
          <ProFormText name="server_url" label={intl.formatMessage({ id: 'plugin.weapp.server-url' })} width="lg" readonly />
          <ProFormText name="token" label={intl.formatMessage({ id: 'plugin.weapp.token' })} width="lg" />
          <ProFormText
            name="encoding_aes_key"
            label={intl.formatMessage({ id: 'plugin.weapp.encoding-aes-key' })}
            width="lg"
            extra={intl.formatMessage({ id: 'plugin.weapp.encoding-aes-key.description' })}
          />
        </ProForm>
        <Divider><FormattedMessage id="plugin.weapp.default" /></Divider>
        <div>
          <p>
            <FormattedMessage id="plugin.weapp.default.tips" />
          </p>
          <p>
            <FormattedMessage id="plugin.weapp.default.help" />
            <a href="https://www.anqicms.com/help-basic/3495.html" target="_blank" rel="noreferrer">
              https://www.anqicms.com/help-basic/3495.html
            </a>
          </p>
          <p><FormattedMessage id="plugin.weapp.default.source" /></p>
        </div>
        <Button onClick={() => window.open('https://github.com/fesiong/anqicms-app/releases')}>
          <FormattedMessage id="plugin.weapp.default.download" />
        </Button>
      </Card>
    </PageContainer>
  );
};

export default PluginWeapp;
