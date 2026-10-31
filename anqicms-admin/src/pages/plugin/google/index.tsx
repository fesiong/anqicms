import NewContainer from '@/components/NewContainer';
import { pluginGetGoogleSetting, pluginSaveGoogleSetting } from '@/services';
import {
  ProForm,
  ProFormInstance,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card, Divider, message } from 'antd';
import React, { useEffect, useState } from 'react';

const PluginGoogle: React.FC<any> = () => {
  const formRef = React.createRef<ProFormInstance>();
  const [googleSetting, setGoogleSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetGoogleSetting();
    let setting = res.data || {};
    // 需要转换数组成字符串
    setting.white_ips = setting.white_ips?.join('\n') || '';
    setting.black_ips = setting.black_ips?.join('\n') || '';
    setting.block_agents = setting.block_agents?.join('\n') || '';
    setting.allow_prefixes = setting.allow_prefixes?.join('\n') || '';

    setGoogleSetting(setting);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    // 提交前，需要将数据转换成数组
    let setting = values;

    pluginSaveGoogleSetting(setting)
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
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        title={intl.formatMessage({ id: 'plugin.google.setting' })}
      >
        {fetched && (
          <div className="mt-normal">
            <ProForm
              onFinish={onSubmit}
              initialValues={googleSetting}
              formRef={formRef}
            >
              <ProFormText
                name="redirect_url"
                label={intl.formatMessage({ id: 'plugin.google.redirect-url' })}
                width="lg"
                readonly
              />
              <ProFormText
                name="client_id"
                label={intl.formatMessage({ id: 'plugin.google.client-id' })}
                extra={intl.formatMessage({
                  id: 'plugin.google.client-id.description',
                })}
              />
              <ProFormText
                name="client_secret"
                label={intl.formatMessage({
                  id: 'plugin.google.client-secret',
                })}
                extra={intl.formatMessage({
                  id: 'plugin.google.client-secret.description',
                })}
              />
              <Divider></Divider>
              <ProFormText
                name="places_api_key"
                label={intl.formatMessage({
                  id: 'plugin.google.places-api-key',
                })}
                extra={intl.formatMessage({
                  id: 'plugin.google.places-api-key.description',
                })}
              />
            </ProForm>
          </div>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginGoogle;
