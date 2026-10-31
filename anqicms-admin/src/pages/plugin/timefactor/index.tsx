import { useVipModal } from '@/components/vipModal';
import {
  getCategories,
  getModules,
  pluginGetTimefactorSetting,
  pluginSaveTimefactorSetting,
} from '@/services';
import {
  PageContainer,
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Card, message } from 'antd';
import React, { useEffect, useState } from 'react';

const PluginTimeFactor: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [setting, setSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [modules, setModules] = useState<any[]>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [renewOpen, setRenewOpen] = useState<boolean>(false);
  const [releaseOpen, setReleaseOpen] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    getModules().then((res) => {
      const data = res.data || [];
      const tmpData = [];
      for (let i in data) {
        if (Object.prototype.hasOwnProperty.call(data, i)) {
          tmpData.push({ label: data[i].name, value: data[i].id });
        }
      }
      setModules(tmpData);
    });
    getCategories({
      type: 1,
    }).then((res) => {
      const data = res.data || [];
      const tmpData = [];
      for (let i in data) {
        if (Object.prototype.hasOwnProperty.call(data, i)) {
          tmpData.push({ label: data[i].title, value: data[i].id });
        }
      }
      setCategories(tmpData);
    });
    pluginGetTimefactorSetting().then((res) => {
      let data = res.data || {};
      if (!data.category_ids) {
        data.category_ids = [];
      }
      setSetting(data);
      setRenewOpen(data.open || false);
      setReleaseOpen(data.release_open || false);
      setFetched(true);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (values: any) => {
    if (values.open) {
      if (!values.module_ids || values.module_ids.length === 0) {
        message.error(
          intl.formatMessage({ id: 'plugin.timefactor.module.required' }),
        );
        return;
      }
      if (!values.types || values.types.length === 0) {
        message.error(
          intl.formatMessage({ id: 'plugin.timefactor.types.required' }),
        );
        return;
      }
      if (values.start_day === 0) {
        message.error(
          intl.formatMessage({ id: 'plugin.timefactor.start-day.required' }),
        );
        return;
      }
      if (values.start_day <= values.end_day) {
        message.error(
          intl.formatMessage({ id: 'plugin.timefactor.end-day.error' }),
        );
      }
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveTimefactorSetting(values)
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
        <Alert
          message={
            <div
              dangerouslySetInnerHTML={{
                __html: intl.formatMessage({ id: 'plugin.timefactor.tips' }),
              }}
            ></div>
          }
        />
        {fetched && (
          <div className="mt-normal">
            <ProForm onFinish={onSubmit} initialValues={setting}>
              <Card
                size="small"
                title={intl.formatMessage({ id: 'plugin.timefactor.setting' })}
                bordered={false}
              >
                <ProFormRadio.Group
                  name="open"
                  label={intl.formatMessage({ id: 'plugin.timefactor.open' })}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'plugin.timefactor.open.no',
                      }),
                      value: false,
                    },
                    {
                      label: intl.formatMessage({
                        id: 'plugin.timefactor.open.yes',
                      }),
                      value: true,
                    },
                  ]}
                  fieldProps={{
                    onChange: (e) => {
                      setRenewOpen(e.target.value);
                    },
                  }}
                  disabled={isVip === false}
                  extra={
                    !isVip ? (
                      <div
                        className="link"
                        onClick={() => {
                          checkVip(() => {});
                        }}
                      >
                        <FormattedMessage id="plugin.timefactor.vip-tip" />
                      </div>
                    ) : null
                  }
                />
                {renewOpen && (
                  <>
                    <ProFormCheckbox.Group
                      name="types"
                      label={intl.formatMessage({
                        id: 'plugin.timefactor.types',
                      })}
                      options={[
                        {
                          value: 'created_time',
                          label: intl.formatMessage({
                            id: 'plugin.timefactor.types.created-time',
                          }),
                        },
                        {
                          value: 'updated_time',
                          label: intl.formatMessage({
                            id: 'plugin.timefactor.types.updated-time',
                          }),
                        },
                      ]}
                      extra={intl.formatMessage({
                        id: 'plugin.timefactor.types.description',
                      })}
                    />
                    <ProForm.Group>
                      <ProFormDigit
                        name="start_day"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.start-day',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.start-day.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.start-day.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.start-day.description',
                        })}
                      />
                      <ProFormDigit
                        name="end_day"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.end-day',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.end-day.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.end-day.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.end-day.description',
                        })}
                      />
                      <ProFormDigit
                        name="daily_update"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.daily-update',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.daily-update.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.daily-update.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.daily-update.description',
                        })}
                      />
                    </ProForm.Group>
                    <ProFormRadio.Group
                      name="do_publish"
                      label={intl.formatMessage({
                        id: 'plugin.timefactor.republish',
                      })}
                      options={[
                        {
                          label: intl.formatMessage({
                            id: 'plugin.timefactor.republish.no',
                          }),
                          value: false,
                        },
                        {
                          label: intl.formatMessage({
                            id: 'plugin.timefactor.republish.yes',
                          }),
                          value: true,
                        },
                      ]}
                      extra={intl.formatMessage({
                        id: 'plugin.timefactor.republish.description',
                      })}
                    />
                  </>
                )}
                <ProFormRadio.Group
                  name="release_open"
                  label={intl.formatMessage({
                    id: 'plugin.timefactor.release-draft',
                  })}
                  options={[
                    {
                      label: intl.formatMessage({
                        id: 'plugin.timefactor.release-draft.no',
                      }),
                      value: false,
                    },
                    {
                      label: intl.formatMessage({
                        id: 'plugin.timefactor.release-draft.yes',
                      }),
                      value: true,
                    },
                  ]}
                  fieldProps={{
                    onChange: (e) => {
                      setReleaseOpen(e.target.value);
                    },
                  }}
                />
                {releaseOpen && (
                  <>
                    <div style={{ width: 200 }}>
                      <ProFormDigit
                        name="daily_limit"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.daily-limit',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.daily-limit.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.daily-limit.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.daily-limit.description',
                        })}
                      />
                    </div>
                    <ProForm.Group>
                      <ProFormDigit
                        name="start_time"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.start-time',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.start-time.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.start-time.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.start-time.description',
                        })}
                      />
                      <ProFormDigit
                        name="end_time"
                        label={intl.formatMessage({
                          id: 'plugin.timefactor.end-time',
                        })}
                        placeholder={intl.formatMessage({
                          id: 'plugin.timefactor.end-time.placeholder',
                        })}
                        addonAfter={intl.formatMessage({
                          id: 'plugin.timefactor.start-time.suffix',
                        })}
                        extra={intl.formatMessage({
                          id: 'plugin.timefactor.end-time.description',
                        })}
                      />
                    </ProForm.Group>
                  </>
                )}
                <ProFormCheckbox.Group
                  name={'module_ids'}
                  label={intl.formatMessage({ id: 'plugin.timefactor.module' })}
                  options={modules}
                />
                <ProFormCheckbox
                  name={'random'}
                  label={intl.formatMessage({ id: 'plugin.timefactor.random' })}
                >
                  {intl.formatMessage({ id: 'plugin.timefactor.random' })}
                </ProFormCheckbox>
                <ProFormSelect
                  name={'category_ids'}
                  label={intl.formatMessage({
                    id: 'plugin.timefactor.category',
                  })}
                  mode="multiple"
                  options={categories}
                  placeholder={intl.formatMessage({
                    id: 'plugin.timefactor.category.placeholder',
                  })}
                />
              </Card>
            </ProForm>
          </div>
        )}
      </Card>
      <VipModal />
    </PageContainer>
  );
};

export default PluginTimeFactor;
