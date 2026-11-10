import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  getCategories,
  getModules,
  pluginBuildLLMs,
  pluginGetLLMs,
  pluginGetLLMsBuildStatus,
  pluginSaveLLMs,
} from '@/services';
import {
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Col, Progress, Row, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useState } from 'react';

let intervalXhr: any = null;
const PluginLLMs: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const formRef = React.createRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>({});
  const [fetched, setFetched] = useState<boolean>(false);
  const [buildStatus, setBuildStatus] = useState<any>(null);
  const [categories, setCategories] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetLLMs();
    setSetting(res.data || { open: false });
    getCategories({ type: 1 }).then((res) => {
      setCategories(res.data || []);
    });
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

  const handleChangeField = (value: any, field: string) => {
    setSetting({
      ...setting,
      [field]: value,
    });
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    let postData = Object.assign(setting, values);
    pluginSaveLLMs(postData)
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

  const rebuildLLMs = () => {
    let values = formRef.current?.getFieldsValue();
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginBuildLLMs(values)
      .then((res) => {
        message.info(res.msg);
        if (res.code === 0) {
          intervalXhr = setInterval(() => {
            pluginGetLLMsBuildStatus().then((res) => {
              setBuildStatus(res.data || null);
              if (
                !res.data ||
                res.data?.status === 2 ||
                res.data?.percent === 100
              ) {
                clearInterval(intervalXhr);
                setBuildStatus(null);
              }
            });
          }, 1000);
        }
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Alert
          className="mb-normal"
          message={
            <div>
              <div>
                <FormattedMessage id="plugin.llms.description" />
              </div>
              {!isVip ? (
                <div
                  className="link mt-normal"
                  onClick={() => {
                    checkVip(
                      () => {},
                      intl.formatMessage({ id: 'plugin.llms.vip-feature' }),
                    );
                  }}
                >
                  <FormattedMessage id="plugin.llms.vip-tip" />
                </div>
              ) : null}
            </div>
          }
        />
        {fetched && (
          <ProForm
            onFinish={onSubmit}
            initialValues={setting}
            formRef={formRef}
          >
            <ProFormRadio.Group
              name="open"
              label={intl.formatMessage({ id: 'plugin.llms.open' })}
              options={[
                {
                  value: false,
                  label: intl.formatMessage({
                    id: 'plugin.llms.open.no',
                  }),
                },
                {
                  value: true,
                  label: intl.formatMessage({
                    id: 'plugin.llms.open.yes',
                  }),
                },
              ]}
              fieldProps={{
                onChange: (e) => handleChangeField(e.target.value, 'open'),
              }}
            />

            {setting.open && (
              <Row gutter={16} className="mb-normal">
                <Col span={12}>
                  <div>
                    <Card
                      size="small"
                      title={intl.formatMessage({
                        id: 'plugin.llms.content-setting',
                      })}
                    >
                      <>
                        <ProFormRadio.Group
                          name="auto_build"
                          label={intl.formatMessage({
                            id: 'plugin.llms.update-frequency',
                          })}
                          options={[
                            {
                              value: 0,
                              label: intl.formatMessage({
                                id: 'plugin.llms.update-frequency.immediately',
                              }),
                            },
                            {
                              value: 1,
                              label: intl.formatMessage({
                                id: 'plugin.llms.update-frequency.daily',
                              }),
                            },
                            {
                              value: 2,
                              label: intl.formatMessage({
                                id: 'plugin.llms.update-frequency.weekly',
                              }),
                            },
                            {
                              value: 3,
                              label: intl.formatMessage({
                                id: 'plugin.llms.update-frequency.never',
                              }),
                            },
                          ]}
                        />
                        <ProFormDigit
                          name="max_post_per_type"
                          label={intl.formatMessage({
                            id: 'plugin.llms.max-posts-pertype',
                          })}
                          layout="horizontal"
                          extra={intl.formatMessage({
                            id: 'plugin.llms.max-posts-pertype.description',
                          })}
                        />
                        <ProFormDigit
                          name="max_words"
                          label={intl.formatMessage({
                            id: 'plugin.llms.max-words',
                          })}
                          layout="vertical"
                          extra={intl.formatMessage({
                            id: 'plugin.llms.max-words.description',
                          })}
                        />
                        <ProFormCheckbox name="include_metadata">
                          <FormattedMessage id="plugin.llms.include-metadata" />
                        </ProFormCheckbox>
                        <ProFormCheckbox name="include_description">
                          <FormattedMessage id="plugin.llms.include-description" />
                        </ProFormCheckbox>
                        <ProFormCheckbox name="include_category">
                          <FormattedMessage id="plugin.llms.include-category" />
                        </ProFormCheckbox>
                        <ProFormCheckbox name="include_tag">
                          <FormattedMessage id="plugin.llms.include-tag" />
                        </ProFormCheckbox>
                        <ProFormCheckbox name="include_extra">
                          <FormattedMessage id="plugin.llms.include-extra" />
                        </ProFormCheckbox>
                        <ProFormSelect
                          name={'exclude_module_ids'}
                          label={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-module',
                          })}
                          mode="multiple"
                          request={async () => {
                            let res = await getModules({});
                            const tmpModules = (res.data || []).map(
                              (item: any) => ({
                                label: item.name,
                                value: item.id,
                              }),
                            );
                            return tmpModules;
                          }}
                          placeholder={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-module.description',
                          })}
                        />
                        <ProFormSelect
                          name={'exclude_category_ids'}
                          label={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-category',
                          })}
                          mode="multiple"
                          options={[
                            {
                              title: intl.formatMessage({
                                id: 'content.please-select',
                              }),
                              value: 0,
                              status: 1,
                            },
                          ]
                            .concat(categories)
                            .map((cat: any) => ({
                              title: cat.title,
                              label: (
                                <div title={cat.title}>
                                  {cat.parents?.length > 0 ? (
                                    <span className="text-muted">
                                      {cat.parents
                                        ?.map((parent: any) => parent.title)
                                        .join(' > ')}
                                      {' > '}
                                    </span>
                                  ) : (
                                    ''
                                  )}
                                  {cat.title}
                                </div>
                              ),
                              value: cat.id,
                              disabled: cat.status !== 1,
                            }))}
                          fieldProps={{
                            showSearch: true,
                            filterOption: (input: string, option: any) =>
                              (option?.title ?? option?.label)
                                .toLowerCase()
                                .includes(input.toLowerCase()),
                          }}
                          placeholder={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-category.description',
                          })}
                        />
                        <ProFormSelect
                          name={'exclude_page_ids'}
                          label={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-page',
                          })}
                          mode="multiple"
                          request={async () => {
                            let res = await getCategories({ type: 3 });
                            const tmpData = (res.data || []).map(
                              (item: any) => ({
                                label: item.title,
                                value: item.id,
                              }),
                            );
                            return tmpData;
                          }}
                          placeholder={intl.formatMessage({
                            id: 'plugin.sitemap.exculde-page.description',
                          })}
                        />
                      </>
                    </Card>
                  </div>
                </Col>
                <Col span={12}>
                  <Card
                    size="small"
                    title={intl.formatMessage({
                      id: 'plugin.llms.extra-setting',
                    })}
                    extra={
                      !isVip ? (
                        <div
                          className="link"
                          onClick={() => {
                            checkVip(
                              () => {},
                              intl.formatMessage({
                                id: 'plugin.llms.vip-feature',
                              }),
                            );
                          }}
                        >
                          <FormattedMessage id="plugin.llms.click-view-vip" />
                        </div>
                      ) : null
                    }
                  >
                    <ProFormTextArea
                      name="llms_title"
                      label={intl.formatMessage({ id: 'plugin.llms.title' })}
                      extra={intl.formatMessage({
                        id: 'plugin.llms.title.description',
                      })}
                      disabled={isVip === false}
                    />
                    <ProFormTextArea
                      name="llms_description"
                      label={intl.formatMessage({
                        id: 'plugin.llms.description.name',
                      })}
                      extra={intl.formatMessage({
                        id: 'plugin.llms.description.description',
                      })}
                      disabled={isVip === false}
                    />
                    <ProFormTextArea
                      name="llms_after_description"
                      label={intl.formatMessage({
                        id: 'plugin.llms.after-description.name',
                      })}
                      extra={intl.formatMessage({
                        id: 'plugin.llms.after-description.description',
                      })}
                      disabled={isVip === false}
                    />
                    <ProFormTextArea
                      name="llms_end_description"
                      label={intl.formatMessage({
                        id: 'plugin.llms.end-description.name',
                      })}
                      extra={intl.formatMessage({
                        id: 'plugin.llms.end-description.description',
                      })}
                      disabled={isVip === false}
                    />
                  </Card>
                  <Card
                    className="mt-normal"
                    size="small"
                    title={intl.formatMessage({ id: 'plugin.llms.status' })}
                  >
                    {!setting.file_status ? (
                      <Alert
                        type="warning"
                        message={intl.formatMessage({
                          id: 'plugin.llms.status.notexist',
                        })}
                      ></Alert>
                    ) : (
                      <>
                        <Alert
                          type="success"
                          message={intl.formatMessage({
                            id: 'plugin.llms.status.exist',
                          })}
                        ></Alert>
                        <div className="mt-normal">
                          {intl.formatMessage({
                            id: 'plugin.llms.status.last-update',
                          })}
                          :{' '}
                          {dayjs(setting.last_update * 1000).format(
                            'YYYY-MM-DD HH:mm:ss',
                          )}
                        </div>
                      </>
                    )}
                    <Space size={16} className="mt-normal">
                      <Button type="primary" onClick={rebuildLLMs}>
                        {intl.formatMessage({ id: 'plugin.llms.build' })}
                      </Button>
                      <Button
                        onClick={() => {
                          window.open(setting.file_url);
                        }}
                      >
                        {intl.formatMessage({ id: 'plugin.llms.view' })}
                      </Button>
                    </Space>
                    {buildStatus && (
                      <div className="mt-normal">
                        <Progress
                          percent={buildStatus.percent}
                          percentPosition={{ align: 'center', type: 'outer' }}
                          format={() =>
                            `${buildStatus.current} / ${buildStatus.total} (${buildStatus.percent}%)`
                          }
                          size="small"
                        />
                      </div>
                    )}
                  </Card>
                </Col>
              </Row>
            )}
          </ProForm>
        )}
      </Card>
      <VipModal />
    </NewContainer>
  );
};

export default PluginLLMs;
