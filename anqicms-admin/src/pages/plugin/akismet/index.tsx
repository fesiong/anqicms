import NewContainer from '@/components/NewContainer';
import { pluginGetAkismetSetting, pluginSaveAkismetSetting } from '@/services';
import {
  ProForm,
  ProFormCheckbox,
  ProFormInstance,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Card, Col, Divider, message, Row } from 'antd';
import React, { useEffect, useState } from 'react';

const PluginAkismet: React.FC<any> = () => {
  const formRef = React.createRef<ProFormInstance>();
  const [akismetSetting, setAkismetSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetAkismetSetting();
    let setting = res.data || {};
    // 需要转换数组成字符串
    setting.white_ips = setting.white_ips?.join('\n') || '';
    setting.black_ips = setting.black_ips?.join('\n') || '';
    setting.block_agents = setting.block_agents?.join('\n') || '';
    setting.allow_prefixes = setting.allow_prefixes?.join('\n') || '';

    setAkismetSetting(setting);
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

    pluginSaveAkismetSetting(setting)
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
        title={intl.formatMessage({ id: 'plugin.akismet.setting' })}
      >
        {fetched && (
          <div className="mt-normal">
            <ProForm
              onFinish={onSubmit}
              initialValues={akismetSetting}
              formRef={formRef}
            >
              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <Divider>Akismet</Divider>
                  <ProFormRadio.Group
                    name={'open'}
                    label={intl.formatMessage({
                      id: 'plugin.akismet.open.name',
                    })}
                    options={[
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.open.false',
                        }),
                        value: false,
                      },
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.open.true',
                        }),
                        value: true,
                      },
                    ]}
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.description',
                    })}
                  />
                  <ProFormText
                    name="api_key"
                    label={intl.formatMessage({ id: 'plugin.akismet.api-key' })}
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.api-key.description',
                    })}
                  />
                  <ProFormCheckbox.Group
                    name="check_type"
                    label={intl.formatMessage({
                      id: 'plugin.akismet.check-type',
                    })}
                    options={[
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.check-type.guestbook',
                        }),
                        value: 1,
                      },
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.check-type.comment',
                        }),
                        value: 2,
                      },
                    ]}
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.check-type.description',
                    })}
                  />
                </Col>
                <Col xs={24} sm={12}>
                  <Divider>reCAPTCHA</Divider>
                  <ProFormRadio.Group
                    name={'recaptcha_open'}
                    label={intl.formatMessage({
                      id: 'plugin.akismet.open.recaptcha',
                    })}
                    options={[
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.open.false',
                        }),
                        value: false,
                      },
                      {
                        label: intl.formatMessage({
                          id: 'plugin.akismet.open.true',
                        }),
                        value: true,
                      },
                    ]}
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.recaptcha.description',
                    })}
                  />
                  <ProFormText
                    name="recaptcha_site_key"
                    label="reCAPTCHA Site Key"
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.recaptcha-site-key-tips',
                    })}
                  />
                  <ProFormText
                    name="recaptcha_private_key"
                    label="reCAPTCHA Private Key"
                    extra={intl.formatMessage({
                      id: 'plugin.akismet.recaptcha-private-key-tips',
                    })}
                  />
                </Col>
              </Row>
            </ProForm>
          </div>
        )}
      </Card>
    </NewContainer>
  );
};

export default PluginAkismet;
