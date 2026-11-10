import {
  pluginGetWatermarkConfig,
  pluginSaveWatermarkConfig,
  pluginWatermarkGenerate,
  pluginWatermarkPreview,
  pluginWatermarkUploadFile,
} from '@/services';
import { PageContainer, ProForm, ProFormRadio, ProFormText } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Col, ColorPicker, Modal, Row, Slider, Upload, message } from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

let loading = false;

const PluginWatermark: React.FC<any> = () => {
  const [setting, setSetting] = useState<any>(null);
  const [fetched, setFetched] = useState<boolean>(false);
  const [previewData, setPreviewData] = useState<string>('');
  const intl = useIntl();

  const getPreviewData = () => {
    if (loading) {
      return;
    }
    loading = true;
    pluginWatermarkPreview()
      .then((res) => {
        setPreviewData(res.data);
      })
      .finally(() => {
        loading = false;
      });
  };

  const getSetting = async () => {
    const res = await pluginGetWatermarkConfig();
    let tmpSetting = res.data || {};
    setSetting(tmpSetting);
    setFetched(true);
    getPreviewData();
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onChangeField = (field: string, val: any) => {
    setting[field] = val;
    setSetting(Object.assign({}, setting));
  };

  const changeOpen = (e: any) => {
    setting.open = e.target.value;
    setSetting(Object.assign({}, setting));
  };

  const changeType = (e: any) => {
    setting.type = e.target.value;
    setSetting(Object.assign({}, setting));
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    let data = Object.assign(setting, values);
    data.width = Number(data.width);
    data.height = Number(data.height);
    pluginSaveWatermarkConfig(data)
      .then((res) => {
        message.success(res.msg);
        getPreviewData();
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleUploadFile = (field: string, e: any) => {
    const formData = new FormData();
    formData.append('file', e.file);
    formData.append('name', field);
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginWatermarkUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        setting[field] = res.data;
        setSetting(Object.assign({}, setting));
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemoveImage = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.material.category.delete.confirm' }),
      onOk: async () => {
        setting.image_path = '';
        setSetting(Object.assign({}, setting));
      },
    });
  };

  const handleRemoveFont = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.material.category.delete.confirm' }),
      onOk: async () => {
        setting.color = '';
        setSetting(Object.assign({}, setting));
      },
    });
  };

  const handleGenerate = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.watermark.generate.confirm' }),
      content: intl.formatMessage({ id: 'plugin.watermark.generate.content' }),
      onOk: async () => {
        pluginWatermarkGenerate({}).then((res) => {
          message.success(res.msg);
        });
      },
    });
  };

  const onChangeColor = (c: any) => {
    setting.color = c.toHex();
    setSetting(Object.assign({}, setting));
  };

  return (
    <PageContainer>
      <Card>
        <Row gutter={16}>
          <Col sm={12} xs={24}>
            {fetched && (
              <ProForm
                title={intl.formatMessage({ id: 'menu.plugin.system.watermark' })}
                layout="vertical"
                initialValues={setting}
                onFinish={onSubmit}
              >
                <ProFormRadio.Group
                  name="open"
                  label={intl.formatMessage({ id: 'plugin.watermark.open' })}
                  options={[
                    {
                      value: false,
                      label: intl.formatMessage({ id: 'plugin.watermark.open.no' }),
                    },
                    {
                      value: true,
                      label: intl.formatMessage({ id: 'plugin.watermark.open.yes' }),
                    },
                  ]}
                  fieldProps={{
                    onChange: changeOpen,
                  }}
                  extra={intl.formatMessage({ id: 'plugin.watermark.open.description' })}
                />

                <div style={{ display: setting.open ? 'block' : 'none' }}>
                  <ProFormRadio.Group
                    name="type"
                    label={intl.formatMessage({ id: 'plugin.watermark.type' })}
                    options={[
                      {
                        value: 0,
                        label: intl.formatMessage({ id: 'plugin.watermark.type.image' }),
                      },
                      {
                        value: 1,
                        label: intl.formatMessage({ id: 'plugin.watermark.type.text' }),
                      },
                    ]}
                    fieldProps={{
                      onChange: changeType,
                    }}
                  />
                  {setting.type === 0 && (
                    <ProFormText label={intl.formatMessage({ id: 'plugin.watermark.image' })}>
                      <Upload
                        name="file"
                        className="logo-uploader"
                        showUploadList={false}
                        accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
                        customRequest={async (e) => handleUploadFile('image_path', e)}
                      >
                        <Button>
                          <FormattedMessage id="plugin.titleimage.bg-image.upload" />
                        </Button>
                      </Upload>
                      {setting.image_path && (
                        <div className="upload-file">
                          <span>{setting.image_path}</span>
                          <a className="delete" onClick={handleRemoveImage}>
                            <FormattedMessage id="setting.system.delete" />
                          </a>
                        </div>
                      )}
                    </ProFormText>
                  )}
                  {setting.type === 1 && (
                    <>
                      <ProFormText
                        name="text"
                        label={intl.formatMessage({ id: 'plugin.watermark.text' })}
                        fieldProps={{
                          onBlur: (e) => onChangeField('text', e.target.value),
                        }}
                      />
                      <ProFormText
                        width="sm"
                        label={intl.formatMessage({ id: 'plugin.titleimage.color' })}
                        extra={intl.formatMessage({ id: 'plugin.titleimage.color.default' })}
                      >
                        <ColorPicker
                          showText
                          value={setting.color}
                          onChange={(e) => {
                            onChangeColor(e);
                          }}
                        />
                      </ProFormText>
                      <ProFormText label={intl.formatMessage({ id: 'plugin.titleimage.font' })}>
                        <Upload
                          name="file"
                          className="logo-uploader"
                          showUploadList={false}
                          accept=".ttf"
                          customRequest={async (e) => handleUploadFile('font_path', e)}
                        >
                          <Button>
                            <FormattedMessage id="plugin.titleimage.font.upload" />
                          </Button>
                        </Upload>
                        {setting.font_path && (
                          <div className="upload-file">
                            <span>{setting.font_path}</span>
                            <a className="delete" onClick={handleRemoveFont}>
                              <FormattedMessage id="setting.system.delete" />
                            </a>
                          </div>
                        )}
                      </ProFormText>
                    </>
                  )}
                  <ProFormRadio.Group
                    name="position"
                    label={intl.formatMessage({ id: 'plugin.watermark.position' })}
                    options={[
                      {
                        value: 5,
                        label: intl.formatMessage({ id: 'plugin.watermark.position.center' }),
                      },
                      {
                        value: 1,
                        label: intl.formatMessage({ id: 'plugin.watermark.position.left-top' }),
                      },
                      {
                        value: 3,
                        label: intl.formatMessage({ id: 'plugin.watermark.position.right-top' }),
                      },
                      {
                        value: 7,
                        label: intl.formatMessage({ id: 'plugin.watermark.position.left-bottom' }),
                      },
                      {
                        value: 9,
                        label: intl.formatMessage({ id: 'plugin.watermark.position.right-bottom' }),
                      },
                    ]}
                    fieldProps={{
                      onChange: (e) => {
                        onChangeField('position', e.target.value);
                      },
                    }}
                  />
                  <ProFormText label={intl.formatMessage({ id: 'plugin.watermark.size' })}>
                    <Slider
                      min={5}
                      max={50}
                      defaultValue={setting.size}
                      onChange={(e) => onChangeField('size', e)}
                    />
                  </ProFormText>
                  <ProFormText label={intl.formatMessage({ id: 'plugin.watermark.opacity' })}>
                    <Slider
                      min={10}
                      max={100}
                      defaultValue={setting.opacity}
                      onChange={(e) => onChangeField('opacity', e)}
                    />
                  </ProFormText>
                  <ProFormText
                    name="min_size"
                    label={intl.formatMessage({ id: 'plugin.watermark.min-size' })}
                    fieldProps={{
                      addonAfter: intl.formatMessage({ id: 'plugin.watermark.min-size.suffix' }),
                      onBlur: (e) => onChangeField('text', e.target.value),
                    }}
                    extra={intl.formatMessage({ id: 'plugin.watermark.min-size.description' })}
                  />
                </div>
                <p>
                  <br />
                </p>
              </ProForm>
            )}
          </Col>
          <Col sm={12} xs={24}>
            <img className="preview" src={previewData} />
            <div className="mt-normal text-center">
              <Button onClick={() => handleGenerate()}>
                <FormattedMessage id="plugin.watermark.batch-add" />
              </Button>
            </div>
          </Col>
        </Row>
      </Card>
    </PageContainer>
  );
};

export default PluginWatermark;
