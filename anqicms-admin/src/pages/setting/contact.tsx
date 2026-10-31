import AttachmentSelect from '@/components/attachment';
import NewContainer from '@/components/NewContainer';
import { getSettingContact, saveSettingContact } from '@/services/setting';
import { PlusOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormInstance,
  ProFormText,
  ProFormDigit,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Col, message, Modal, Row } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingContactFrom: React.FC<any> = () => {
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>(null);
  const [qrcode, setQrcode] = useState<string>('');
  const [shopLogo, setShopLogo] = useState<string>('');
  const [extraFields, setExtraFields] = useState<any[]>([]);
  const [activeTabKey, setActiveTabKey] = useState<string>('base');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingContact();
    let setting = res.data || null;
    setSetting(setting);
    setQrcode(setting?.qrcode || '');
    setShopLogo(setting?.shop_logo || '');
    setExtraFields(setting.extra_fields || []);
    formRef.current?.setFieldsValue(setting);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleSelectQrcode = (row: any) => {
    setQrcode(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveQrcode = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setQrcode('');
      },
    });
  };

  const handleSelectShopLogo = (row: any) => {
    setShopLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveShopLogo = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setShopLogo('');
      },
    });
  };

  const onSubmit = async (values: any) => {
    // 页面按 tab 条件渲染，onFinish 的 values 只含当前 tab 字段，
    // 后端整包覆盖保存会清空其它 tab 的配置，必须取全量表单值
    const allValues: any = {
      ...(formRef.current?.getFieldsValue(true) || values),
    };
    allValues.qrcode = qrcode;
    allValues.shop_logo = shopLogo;
    allValues.extra_fields = extraFields;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingContact(allValues)
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

  const tabContnet: Record<string, React.ReactNode> = {
    base: (
      <div>
        <ProFormText
          name="user_name"
          label={intl.formatMessage({ id: 'setting.contact.username' })}
          width="lg"
        />
        <ProFormText
          name="cellphone"
          label={intl.formatMessage({ id: 'setting.contact.cellphone' })}
          width="lg"
        />
        <ProFormText
          name="address"
          label={intl.formatMessage({ id: 'setting.contact.address' })}
          width="lg"
        />
        <ProFormText
          name="email"
          label={intl.formatMessage({ id: 'setting.contact.email' })}
          width="lg"
        />
        <ProFormText
          name="wechat"
          label={intl.formatMessage({ id: 'setting.contact.wechat' })}
          width="lg"
        />
        <ProFormText
          label={intl.formatMessage({ id: 'setting.contact.qrcode' })}
          width="lg"
        >
          <AttachmentSelect onSelect={handleSelectQrcode} open={false}>
            <div className="ant-upload-item">
              {qrcode ? (
                <>
                  <img src={qrcode} style={{ width: '100%' }} />
                  <a className="delete" onClick={handleRemoveQrcode}>
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
      </div>
    ),
    media: (
      <div>
        <ProFormText name="qq" label="QQ" width="lg" />
        <ProFormText name="whats_app" label="WhatsApp" width="lg" />
        <ProFormText name="facebook" label="Facebook" width="lg" />
        <ProFormText name="twitter" label="Twitter" width="lg" />
        <ProFormText name="tiktok" label="Tiktok" width="lg" />
        <ProFormText name="pinterest" label="Pinterest" width="lg" />
        <ProFormText name="linkedin" label="Linkedin" width="lg" />
        <ProFormText name="instagram" label="Instagram" width="lg" />
        <ProFormText name="youtube" label="Youtube" width="lg" />
      </div>
    ),
    shop: (
      <div>
        <ProFormText
          name="shop_name"
          label={intl.formatMessage({ id: 'setting.shop.name' })}
          width="lg"
          placeholder={intl.formatMessage({ id: 'setting.shop.name' })}
          extra={intl.formatMessage({
            id: 'setting.shop.name-description',
          })}
        />
        <ProFormText
          label={intl.formatMessage({ id: 'setting.shop.logo' })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.shop.logo-description',
          })}
        >
          <AttachmentSelect onSelect={handleSelectShopLogo} open={false}>
            <div className="ant-upload-item">
              {shopLogo ? (
                <>
                  <img src={shopLogo} style={{ width: '100%' }} />
                  <a className="delete" onClick={handleRemoveShopLogo}>
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
        <ProFormTextArea
          name="shop_description"
          label={intl.formatMessage({ id: 'setting.shop.description' })}
          width="lg"
          fieldProps={{ rows: 3 }}
          placeholder={intl.formatMessage({
            id: 'setting.shop.description-placeholder',
          })}
          extra={intl.formatMessage({
            id: 'setting.shop.description-description',
          })}
        />
        <ProFormText
          name="business_hours"
          label={intl.formatMessage({
            id: 'setting.shop.business-hours',
          })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.shop.business-hours-placeholder',
          })}
          extra={intl.formatMessage({
            id: 'setting.shop.business-hours-description',
          })}
        />
        <ProFormText
          name="delivery_location"
          label={intl.formatMessage({
            id: 'setting.shop.delivery-location',
          })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.shop.delivery-location-placeholder',
          })}
        />
        <ProFormText
          name="delivery_latitude"
          label={intl.formatMessage({
            id: 'setting.shop.delivery-latitude',
          })}
          width="lg"
        />
        <ProFormText
          name="delivery_longitude"
          label={intl.formatMessage({
            id: 'setting.shop.delivery-longitude',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.shop.delivery-longitude-description',
          })}
        />
        <ProFormDigit
          name="shipping_time"
          label={intl.formatMessage({ id: 'setting.shop.shipping-time' })}
          width="lg"
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.shop.shipping-time.suffix',
            }),
          }}
        />
        <ProFormText
          name="shipping_time_remark"
          label={intl.formatMessage({
            id: 'setting.shop.shipping-time-description',
          })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.shop.shipping-time-placeholder',
          })}
          extra={intl.formatMessage({
            id: 'setting.shop.shipping-time-description',
          })}
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
        initialValues={setting}
        onFinish={onSubmit}
        title={intl.formatMessage({ id: 'menu.setting.contact' })}
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
              key: 'media',
              label: intl.formatMessage({ id: 'setting.tab.social' }),
            },
            {
              key: 'shop',
              label: intl.formatMessage({ id: 'setting.shop.info' }),
            },
            {
              key: 'diy',
              label: intl.formatMessage({ id: 'setting.system.diy-params' }),
            },
          ]}
        >
          {tabContnet[activeTabKey] || null}
        </Card>
      </ProForm>
    </NewContainer>
  );
};

export default SettingContactFrom;
