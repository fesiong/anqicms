import AttachmentSelect from '@/components/attachment';
import NewContainer from '@/components/NewContainer';
import {
  deleteSystemFavicon,
  getSettingSystem,
  saveSettingSystem,
  saveSystemFavicon,
} from '@/services/setting';
import { PlusOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { Button, Card, Col, message, Modal, Row, Upload } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingSystemFrom: React.FC<any> = () => {
  const formRef = useRef<ProFormInstance>();
  const { initialState, setInitialState } = useModel('@@initialState');
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({});
  const [siteLogo, setSiteLogo] = useState<string>('');
  const [site_close, setSiteClose] = useState<number>(0);
  const [extraFields, setExtraFields] = useState<any[]>([]);
  const [activeTabKey, setActiveTabKey] = useState<string>('base');
  const [newKey, setNewKey] = useState<string>('');
  // 站点模式（0=普通内容模式，1=商城模式）的服务端现值，
  // 用于切换时的二次确认比对
  const shopModeRef = useRef<number>(0);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingSystem();
    let setting = res.data || { system: {}, template_names: [] };
    setSetting(setting);
    setSiteLogo(setting.system?.site_logo || '');
    setSiteClose(setting.system?.site_close || 0);
    setExtraFields(setting.system.extra_fields || []);
    shopModeRef.current = setting.system?.shop_mode || 0;
    setFetched(true);
    formRef.current?.setFieldsValue(setting.system);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleSelectLogo = (row: any) => {
    setSiteLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveLogo = (e: any) => {
    e.stopPropagation();
    setSiteLogo('');
  };

  const handleRemoveFavicon = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete-ico' }),
      onOk: () => {
        deleteSystemFavicon({}).then((res) => {
          message.info(res.msg);
          getSetting();
        });
      },
    });
  };

  const handleUploadFavicon = async (e: any) => {
    const formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSystemFavicon(formData)
      .then((res) => {
        message.success(res.msg);
        if (res.data?.favicon) {
          setSetting({
            ...setting,
            system: { ...setting.system, favicon: res.data.favicon },
          });
        }
      })
      .finally(() => {
        hide();
      });
  };

  const doSubmit = async (allValues: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingSystem(allValues)
      .then(async (res) => {
        message.success(res.msg);
        shopModeRef.current = allValues.shop_mode || 0;
        const system = await initialState?.fetchSystemSetting?.();
        if (system) {
          await setInitialState((s) => ({
            ...s,
            system: system,
          }));
        }
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const onSubmit = async (values: any) => {
    // 页面按 tab 条件渲染，未挂载 tab 的字段不会出现在 onFinish 的 values 里，
    // 而后端是整包覆盖保存，直接提交会清空其它 tab 的配置。
    // 这里必须取全量表单值（切 tab 后未挂载字段的值仍保留在 store 中）。
    const allValues = {
      ...(formRef.current?.getFieldsValue(true) || values),
    };
    allValues.site_logo = siteLogo;
    allValues.extra_fields = extraFields;
    // 站点模式切换影响面大（菜单与前台商城接口整体启停），需要二次确认
    if ((allValues.shop_mode || 0) !== shopModeRef.current) {
      Modal.confirm({
        title: intl.formatMessage({ id: 'setting.system.shop-mode.confirm' }),
        content: intl.formatMessage({
          id: 'setting.system.shop-mode.confirm-content',
        }),
        okText: intl.formatMessage({ id: 'setting.system.shop-mode.confirm-ok' }),
        cancelText: intl.formatMessage({
          id: 'setting.system.shop-mode.confirm-cancel',
        }),
        onOk: () => {
          return doSubmit(allValues);
        },
      });
      return;
    }
    return doSubmit(allValues);
  };

  const getLangName = (lang: string) => {
    switch (lang) {
      case 'zh':
      case 'zh-CN':
        return '中文';
      case 'en':
      case 'en-US':
        return 'English';
      case 'ja-JP':
        return '日本語';
      case 'ko-KR':
        return '한국어';
      case 'zh-TW':
        return '繁體中文';
      case 'fa-IR':
        return 'فارسی';
      case 'pt-BR':
        return 'Português';
      case 'ru-RU':
        return 'Русский';
      case 'id-ID':
        return 'Bahasa Indonesia';
      case 'bn-BD':
        return 'বাংলা';
      default:
        return lang;
    }
  };

  const tabContnet: Record<string, React.ReactNode> = {
    base: (
      <div>
        <ProFormText
          name="site_name"
          label={intl.formatMessage({
            id: 'setting.system.site-name',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.system.site-name-description',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'setting.system.site-name-error',
              }),
            },
          ]}
        />
        <ProFormText
          label={intl.formatMessage({
            id: 'setting.system.site-logo',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.system.site-logo-description',
          })}
        >
          <AttachmentSelect onSelect={handleSelectLogo} open={false}>
            <div className="ant-upload-item">
              {siteLogo ? (
                <>
                  <img src={siteLogo} style={{ width: '100%' }} />
                  <a className="delete" onClick={handleRemoveLogo}>
                    <FormattedMessage id="setting.system.delete" />
                  </a>
                </>
              ) : (
                <div className="add">
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>
                    <FormattedMessage id="setting.system.upload" />
                  </div>
                </div>
              )}
            </div>
          </AttachmentSelect>
        </ProFormText>
        <ProFormText
          label={intl.formatMessage({
            id: 'setting.system.site-ico',
          })}
          extra={intl.formatMessage({
            id: 'setting.system.site-ico-description',
          })}
        >
          <Upload
            name="file"
            className="logo-uploader"
            showUploadList={false}
            accept=".jpg,.jpeg,.png,.gif,.webp,.ico,.bmp"
            customRequest={async (e) => handleUploadFavicon(e)}
          >
            <div className="ant-upload-item">
              {setting.system?.favicon ? (
                <>
                  <img src={setting.system.favicon} style={{ width: '100%' }} />
                  <a className="delete" onClick={handleRemoveFavicon}>
                    <FormattedMessage id="setting.system.delete" />
                  </a>
                </>
              ) : (
                <div className="add">
                  <PlusOutlined />
                  <div style={{ marginTop: 8 }}>
                    <FormattedMessage id="setting.system.upload" />
                  </div>
                </div>
              )}
            </div>
          </Upload>
        </ProFormText>
        <ProFormText
          name="site_icp"
          label={intl.formatMessage({
            id: 'setting.system.site-icp',
          })}
          width="lg"
          extra={
            <div>
              <FormattedMessage id="setting.system.site-icp-description-before" />
              <a
                href="https://beian.miit.gov.cn/"
                target="_blank"
                rel="noreferrer"
              >
                beian.miit.gov.cn
              </a>
              <FormattedMessage id="setting.system.site-icp-description-after" />
            </div>
          }
        />
        <ProFormTextArea
          name="site_copyright"
          width="lg"
          label={intl.formatMessage({
            id: 'setting.system.site-copyright',
          })}
          placeholder={intl.formatMessage({
            id: 'setting.system.site-copyright-placeholder',
          })}
          extra={intl.formatMessage({
            id: 'setting.system.site-copyright-description',
          })}
        />
        <ProFormSelect
          name="language"
          width="lg"
          label={intl.formatMessage({
            id: 'setting.system.language',
          })}
          request={async () => {
            let names = [];
            for (let item of setting.languages) {
              names.push({ label: getLangName(item), value: item });
            }
            return names;
          }}
          extra={intl.formatMessage({
            id: 'setting.system.language-description',
          })}
        />
      </div>
    ),
    address: (
      <div>
        <ProFormText
          name="base_url"
          label={intl.formatMessage({
            id: 'setting.system.base-url',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.system.base-url-description',
          })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'setting.system.base-url-error',
              }),
            },
          ]}
        />
        <ProFormText
          name="front_url"
          label={intl.formatMessage({
            id: 'setting.system.front-url',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.system.front-url-description',
          })}
        />
        <ProFormText
          name="mobile_url"
          label={intl.formatMessage({
            id: 'setting.system.mobile-url',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.system.mobile-url-description',
          })}
        />
        <ProFormText
          name="admin_url"
          label={intl.formatMessage({
            id: 'setting.system.admin-url',
          })}
          width="lg"
          fieldProps={{
            suffix: '/system/',
            placeholder: intl.formatMessage({
              id: 'setting.system.admin-url-placeholder',
            }),
          }}
          extra={
            <div>
              <div>
                <FormattedMessage id="setting.system.admin-url-description-before" />
              </div>
              <div>
                <FormattedMessage id="setting.system.admin-url-description-notice" />
                <span className="text-red">
                  <FormattedMessage id="setting.system.admin-url-description-notice-value" />
                </span>
                <FormattedMessage id="setting.system.admin-url-description-after" />
              </div>
            </div>
          }
        />
      </div>
    ),
    runtime: (
      <div>
        <ProFormRadio.Group
          name="site_close"
          label={intl.formatMessage({
            id: 'setting.system.site-close',
          })}
          extra={intl.formatMessage({
            id: 'setting.system.site-close-description',
          })}
          fieldProps={{
            onChange: (e: any) => {
              setSiteClose(e.target.value);
            },
          }}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.system.normal',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.system.close',
              }),
            },
            {
              value: 2,
              label: intl.formatMessage({
                id: 'setting.system.spider',
              }),
            },
          ]}
        />
        {(site_close === 1 || site_close === 2) && (
          <ProFormTextArea
            name="site_close_tips"
            label={intl.formatMessage({
              id: 'setting.system.site-close-tips',
            })}
            width="lg"
            extra={intl.formatMessage({
              id: 'setting.system.site-close-tips-description',
            })}
          />
        )}
        <ProFormRadio.Group
          name="ban_spider"
          label={intl.formatMessage({
            id: 'setting.system.spider-visible',
          })}
          extra={intl.formatMessage({
            id: 'setting.system.spider-visible-description',
          })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.system.visible',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.system.ban-spider',
              }),
            },
          ]}
        />
        <ProFormRadio.Group
          name="shop_mode"
          label={intl.formatMessage({
            id: 'setting.system.shop-mode',
          })}
          extra={intl.formatMessage({
            id: 'setting.system.shop-mode-description',
          })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.system.shop-mode.normal',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.system.shop-mode.shop',
              }),
            },
          ]}
        />
      </div>
    ),
    diy: (
      <div>
        <div className="mb-normal">
          <Button
            size="small"
            onClick={(e) => {
              e.stopPropagation();
              extraFields.push({ name: '', value: '', remark: '' });
              setExtraFields([].concat(extraFields));
            }}
          >
            <FormattedMessage id="setting.system.add-param" />
          </Button>
        </div>
        {extraFields.map((row: any, index: number) => (
          <Row key={index} gutter={16}>
            <Col sm={8} xs={12}>
              <ProFormText
                name={['extra_fields', index, 'name']}
                label={intl.formatMessage({
                  id: 'setting.system.param-name',
                })}
                fieldProps={{
                  value: row.name,
                  onChange: (e: any) => {
                    extraFields[index].name = e.target.value;
                    setExtraFields([].concat(extraFields));
                  },
                }}
                required={true}
                extra={intl.formatMessage({
                  id: 'setting.system.param-name-description',
                })}
              />
            </Col>
            <Col sm={8} xs={12}>
              <ProFormText
                name={['extra_fields', index, 'value']}
                label={intl.formatMessage({
                  id: 'setting.system.param-value',
                })}
                fieldProps={{
                  value: row.value,
                  onChange: (e: any) => {
                    extraFields[index].value = e.target.value;
                    setExtraFields([].concat(extraFields));
                  },
                }}
              />
            </Col>
            <Col sm={6} xs={12}>
              <ProFormText
                name={['extra_fields', index, 'remark']}
                label={intl.formatMessage({
                  id: 'setting.system.remark',
                })}
                fieldProps={{
                  value: row.remark,
                  onChange: (e: any) => {
                    extraFields[index].remark = e.target.value;
                    setExtraFields([].concat(extraFields));
                  },
                }}
              />
            </Col>
            <Col sm={2} xs={12}>
              <Button
                style={{ marginTop: '30px' }}
                onClick={() => {
                  Modal.confirm({
                    title: intl.formatMessage({
                      id: 'setting.system.confirm-delete-param',
                    }),
                    onOk: () => {
                      extraFields.splice(index, 1);
                      setExtraFields([].concat(extraFields));
                    },
                  });
                }}
              >
                <FormattedMessage id="setting.system.delete" />
              </Button>
            </Col>
          </Row>
        ))}
      </div>
    ),
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProForm
        formRef={formRef}
        initialValues={setting.system}
        onFinish={onSubmit}
        title={intl.formatMessage({ id: 'menu.setting.system' })}
      >
        <Card
          className="mb-normal"
          key={newKey}
          activeTabKey={activeTabKey}
          onTabChange={(tabKey) => setActiveTabKey(tabKey)}
          tabList={[
            {
              key: 'base',
              label: intl.formatMessage({ id: 'setting.tab.base' }),
            },
            {
              key: 'address',
              label: intl.formatMessage({ id: 'setting.tab.address' }),
            },
            {
              key: 'runtime',
              label: intl.formatMessage({ id: 'setting.tab.runtime' }),
            },
            {
              key: 'diy',
              label: intl.formatMessage({ id: 'setting.system.diy-params' }),
            },
          ]}
        >
          {(fetched && tabContnet[activeTabKey]) || null}
        </Card>
      </ProForm>
    </NewContainer>
  );
};

export default SettingSystemFrom;
