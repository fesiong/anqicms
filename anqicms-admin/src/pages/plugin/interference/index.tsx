import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  pluginGetInterferenceConfig,
  pluginSaveInterferenceConfig,
} from '@/services';
import { ProForm, ProFormRadio } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Card, message } from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

const PluginInterference: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [setting, setSetting] = useState<any>(null);
  const [fetched, setFetched] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetInterferenceConfig();
    setSetting(res.data || {});
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
    pluginSaveInterferenceConfig(Object.assign(setting, values))
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
      <Card key={newKey}>
        {fetched && (
          <ProForm
            title={intl.formatMessage({ id: 'menu.plugin.system.interference' })}
            layout="vertical"
            initialValues={setting}
            onFinish={onSubmit}
          >
            <ProFormRadio.Group
              name="open"
              label={intl.formatMessage({ id: 'plugin.interference.isopen' })}
              extra={intl.formatMessage({
                id: 'plugin.interference.isopen.description',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.interference.isopen.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.interference.isopen.yes',
                  }),
                },
              ]}
              disabled={isVip === false}
            />
            {!isVip ? (
              <div
                className="link mb-normal"
                onClick={() => {
                  checkVip(() => {});
                }}
              >
                <FormattedMessage id="plugin.interference.vip-tip" />
              </div>
            ) : null}
            <ProFormRadio.Group
              name="mode"
              label={intl.formatMessage({ id: 'plugin.interference.mode' })}
              options={[
                {
                  value: 0,
                  label: intl.formatMessage({
                    id: 'plugin.interference.mode.class',
                  }),
                },
                {
                  value: 1,
                  label: intl.formatMessage({
                    id: 'plugin.interference.mode.text',
                  }),
                },
              ]}
            />
            <ProFormRadio.Group
              name="disable_selection"
              label={intl.formatMessage({
                id: 'plugin.interference.disable-selection',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.yes',
                  }),
                },
              ]}
            />
            <ProFormRadio.Group
              name="disable_copy"
              label={intl.formatMessage({
                id: 'plugin.interference.disable-copy',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.yes',
                  }),
                },
              ]}
            />
            <ProFormRadio.Group
              name="disable_right_click"
              label={intl.formatMessage({
                id: 'plugin.interference.disable-right-click',
              })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.interference.disable-selection.yes',
                  }),
                },
              ]}
            />
          </ProForm>
        )}
      </Card>
      <VipModal />
    </NewContainer>
  );
};

export default PluginInterference;
