import NewContainer from '@/components/NewContainer';
import {
  getSettingCache,
  saveSettingCache,
  saveSettingMigrateDB,
} from '@/services/setting';
import { removeStore } from '@/utils/store';
import { ProForm, ProFormRadio, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Modal, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

const ToolCacheForm: React.FC<any> = () => {
  const [setting, setSetting] = useState<any>(null);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingCache();
    let setting = res.data || null;
    setSetting(setting);
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
    removeStore('unsaveArchive');
    saveSettingCache(values)
      .then((res) => {
        message.success(res.msg);
        getSetting();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const onSubmitUpdate = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    values.update = true;
    saveSettingCache(values)
      .then((res) => {
        message.success(res.msg);
        getSetting();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const onSubmitMigrate = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'tool.cache.confirm-migrate' }),
      okType: 'primary',
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        saveSettingMigrateDB({})
          .then((res) => {
            message.success(res.msg);
          })
          .catch((err) => {
            console.log(err);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <div key={newKey}>
        <Card title={intl.formatMessage({ id: 'tool.cache.temp-cache' })}>
          {setting && (
            <ProForm submitter={false}>
              <ProFormText
                name="last_update"
                fieldProps={{
                  value:
                    setting.last_update > 0
                      ? dayjs(setting.last_update * 1000).format(
                          'YYYY-MM-DD HH:mm',
                        )
                      : intl.formatMessage({ id: 'tool.cache.never' }),
                }}
                label={intl.formatMessage({ id: 'tool.cache.last-time' })}
                width="lg"
                readonly
              />
              <ProFormText>
                <Button onClick={() => onSubmit({})} type="primary">
                  <FormattedMessage id="tool.cache.clean" />
                </Button>
              </ProFormText>
            </ProForm>
          )}
        </Card>
        <div className="mt-normal">
          <Card title="">
            {setting && (
              <ProForm onFinish={onSubmitUpdate} initialValues={setting}>
                <ProFormRadio.Group
                  name="cache_type"
                  label={intl.formatMessage({ id: 'tool.cache.cache-type' })}
                  options={[
                    {
                      value: '',
                      label: intl.formatMessage({
                        id: 'tool.cache.cache-type.auto',
                      }),
                    },
                    {
                      value: 'file',
                      label: intl.formatMessage({
                        id: 'tool.cache.cache-type.file',
                      }),
                    },
                    {
                      value: 'memory',
                      label: intl.formatMessage({
                        id: 'tool.cache.cache-type.memory',
                      }),
                    },
                  ]}
                  extra={intl.formatMessage({
                    id: 'tool.cache.cache-type.description',
                  })}
                />
              </ProForm>
            )}
          </Card>
        </div>
        <div className="mt-normal">
          <Card title={intl.formatMessage({ id: 'tool.cache.table-upgrade' })}>
            <ProForm submitter={false}>
              <Alert
                className="mb-normal"
                message={intl.formatMessage({
                  id: 'tool.cache.table-upgrade.description',
                })}
              />
              <ProFormText>
                <Button onClick={() => onSubmitMigrate()} type="primary">
                  <FormattedMessage id="tool.cache.table-upgrade" />
                </Button>
              </ProFormText>
            </ProForm>
          </Card>
        </div>
      </div>
    </NewContainer>
  );
};

export default ToolCacheForm;
