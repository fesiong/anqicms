import {
  anqiRestart,
  checkVersion,
  getVersion,
  upgradeVersion,
} from '@/services';
import {
  PageContainer,
  ProForm,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Modal, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';

let loading = false;

const ToolUpgradeForm: React.FC<any> = () => {
  const [setting, setSetting] = useState<any>(null);
  const [newVersion, setNewVersion] = useState<any>(null);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getVersion();
    let setting = res.data || null;

    setSetting(setting);

    checkVersion().then((res) => {
      setNewVersion(res.data || null);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const upgradeSubmit = async () => {
    if (loading) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'tool.confirm-upgrade' }),
      onOk: () => {
        loading = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'tool.upgrading' }),
          0,
        );
        upgradeVersion({ version: newVersion.version })
          .then((res) => {
            Modal.info({
              content: res.msg,
              okText: intl.formatMessage({ id: 'tool.confirm-restart' }),
              onOk() {
                const hide2 = message.loading(
                  intl.formatMessage({ id: 'tool.restarting' }),
                  0,
                );
                anqiRestart({})
                  .then(() => {})
                  .catch(() => {})
                  .finally(() => {
                    setTimeout(() => {
                      hide2();
                      window.location.reload();
                    }, 3000);
                  });
              },
            });
            getSetting();
          })
          .catch((err) => {
            console.log(err);
          })
          .finally(() => {
            loading = false;
            hide();
          });
      },
    });
  };

  return (
    <PageContainer>
      <Card>
        {setting && (
          <ProForm
            submitter={false}
            title={intl.formatMessage({ id: 'menu.upgrade' })}
          >
            <ProFormText
              name="old_version"
              fieldProps={{
                value: setting.version,
              }}
              label={intl.formatMessage({ id: 'tool.version' })}
              width="lg"
              readonly
            />
            {newVersion ? (
              <div>
                <ProFormText
                  label={intl.formatMessage({ id: 'tool.new-version' })}
                  width="lg"
                  readonly
                >
                  <Space>
                    <div className="text-primary">{newVersion.version}</div>
                    {newVersion.trial && (
                      <Tag color="orange">
                        {intl.formatMessage({
                          id: 'dashboard.soft-info.trial-version',
                        })}
                      </Tag>
                    )}
                  </Space>
                </ProFormText>
                <ProFormText
                  label={intl.formatMessage({ id: 'tool.version.description' })}
                  width="lg"
                  readonly
                >
                  <div
                    className="elem-quote"
                    dangerouslySetInnerHTML={{ __html: newVersion.description }}
                  ></div>
                </ProFormText>
                <div className="mt-normal">
                  <Button type="primary" onClick={upgradeSubmit}>
                    <FormattedMessage id="tool.upgrade-to-new" />
                    <FormattedMessage id="dashboard.soft-info.trial-version" />
                  </Button>
                </div>
              </div>
            ) : (
              <div>
                <FormattedMessage id="tool.version.tips.before" />{' '}
                <a href="https://www.anqicms.com/download" target={'_blank'}>
                  https://www.anqicms.com/download
                </a>{' '}
                <FormattedMessage id="tool.version.tips.after" />
              </div>
            )}
          </ProForm>
        )}
      </Card>
    </PageContainer>
  );
};

export default ToolUpgradeForm;
