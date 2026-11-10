import NewContainer from '@/components/NewContainer';
import { useVipModal } from '@/components/vipModal';
import {
  pluginGetTitleImageConfig,
  pluginSaveTitleImageConfig,
  pluginTitleImagePreview,
  pluginTitleImageUploadFile,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProForm,
  ProFormDigit,
  ProFormGroup,
  ProFormRadio,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  ColorPicker,
  Image,
  Modal,
  Row,
  Space,
  Upload,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

const PluginTitleImage: React.FC<any> = () => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const { initialState, setInitialState } = useModel('@@initialState');
  const [setting, setSetting] = useState<any>(null);
  const [fetched, setFetched] = useState<boolean>(false);
  const [visible, setVisible] = useState<boolean>(false);
  const [previewData, setPreviewData] = useState<string>('');
  const [newKey, setNewKey] = useState<string>('');
  let previewText = '';
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetTitleImageConfig();
    setSetting(res.data || {});
    setFetched(true);
  };

  const getPreviewData = () => {
    pluginTitleImagePreview({ text: previewText }).then((res) => {
      setPreviewData(res.data);
    });
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      getPreviewData();
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
    getPreviewData();
  }, []);

  const changeOpen = (e: any) => {
    setting.open = e.target.value;
    setSetting(Object.assign({}, setting));
  };

  const onSubmit = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    let data = Object.assign(setting, values);
    data.width = Number(data.width);
    data.height = Number(data.height);
    pluginSaveTitleImageConfig(data)
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
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginTitleImageUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        if (field === 'bg_image') {
          if (!setting.bg_images) {
            setting.bg_images = [];
          }
          setting.bg_images.push(res.data);
        } else {
          setting[field] = res.data;
        }
        setSetting(Object.assign({}, setting));
        getPreviewData();
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemoveBgImage = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setting.bg_images.splice(index, 1);
        setSetting(Object.assign({}, setting));
        getPreviewData();
      },
    });
  };

  const handleRemoveFont = (e: any) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setting.font_path = '';
        setSetting(Object.assign({}, setting));
        getPreviewData();
      },
    });
  };

  const onChangeColor = (c: string) => {
    setting.font_color = c;
    setSetting(Object.assign({}, setting));
    getPreviewData();
  };

  const onChangeBgColor = (c: string) => {
    setting.font_bg_color = c;
    setSetting(Object.assign({}, setting));
    getPreviewData();
  };

  const cleanFontBgColor = () => {
    setting.font_bg_color = '';
    setSetting(Object.assign({}, setting));
    getPreviewData();
  };

  const confirmChangeText = async (values: any) => {
    previewText = values.text;
    setVisible(false);
    getPreviewData();
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card key={newKey}>
        <Row gutter={16}>
          <Col span={12}>
            {fetched && (
              <ProForm
                title={intl.formatMessage({ id: 'menu.plugin.contentops.titleimage' })}
                layout="vertical"
                initialValues={setting}
                onFinish={onSubmit}
              >
                <ProFormRadio.Group
                  name="open"
                  label={intl.formatMessage({ id: 'plugin.titleimage.open' })}
                  options={[
                    {
                      value: false,
                      label: intl.formatMessage({
                        id: 'plugin.titleimage.open.no',
                      }),
                    },
                    {
                      value: true,
                      label: intl.formatMessage({
                        id: 'plugin.titleimage.open.yes',
                      }),
                    },
                  ]}
                  fieldProps={{
                    onChange: changeOpen,
                  }}
                  extra={intl.formatMessage({
                    id: 'plugin.titleimage.open.description',
                  })}
                  disabled={isVip === false}
                />
                {!isVip ? (
                  <div
                    className="link"
                    onClick={() => {
                      checkVip(() => {});
                    }}
                  >
                    <FormattedMessage id="plugin.titleimage.vip-tip" />
                  </div>
                ) : null}
                <div style={{ display: setting.open ? 'block' : 'none' }}>
                  <ProFormRadio.Group
                    name="draw_sub"
                    label={intl.formatMessage({
                      id: 'plugin.titleimage.draw-sub',
                    })}
                    options={[
                      {
                        value: false,
                        label: intl.formatMessage({
                          id: 'plugin.titleimage.open.no',
                        }),
                      },
                      {
                        value: true,
                        label: intl.formatMessage({
                          id: 'plugin.titleimage.open.yes',
                        }),
                      },
                    ]}
                    extra={intl.formatMessage({
                      id: 'plugin.titleimage.draw-sub.description',
                    })}
                  />
                  <ProFormGroup
                    title={intl.formatMessage({ id: 'plugin.titleimage.size' })}
                  >
                    <ProFormText
                      name="width"
                      width="sm"
                      fieldProps={{
                        suffix: intl.formatMessage({
                          id: 'plugin.titleimage.width',
                        }),
                      }}
                      placeholder={intl.formatMessage({
                        id: 'plugin.titleimage.width.placeholder',
                      })}
                    />
                    ×
                    <ProFormText
                      name="height"
                      width="sm"
                      fieldProps={{
                        suffix: intl.formatMessage({
                          id: 'plugin.titleimage.height',
                        }),
                      }}
                      placeholder={intl.formatMessage({
                        id: 'plugin.titleimage.height.placeholder',
                      })}
                    />
                  </ProFormGroup>
                  <ProFormText
                    width="sm"
                    label={intl.formatMessage({
                      id: 'plugin.titleimage.color',
                    })}
                    extra={intl.formatMessage({
                      id: 'plugin.titleimage.color.default',
                    })}
                  >
                    <ColorPicker
                      key="1"
                      showText
                      value={setting.font_color}
                      onChange={(e) => {
                        onChangeColor(e.toHex());
                      }}
                    />
                  </ProFormText>
                  <ProFormText
                    width="sm"
                    label={intl.formatMessage({
                      id: 'plugin.titleimage.color_bg',
                    })}
                    extra={intl.formatMessage({
                      id: 'plugin.titleimage.color_bg.default',
                    })}
                  >
                    <Space align="center" size={16}>
                      <ColorPicker
                        key="2"
                        showText
                        value={setting.font_bg_color}
                        onChange={(e) => {
                          onChangeBgColor(e.toHex());
                        }}
                        allowClear
                        onClear={() => cleanFontBgColor()}
                      />
                    </Space>
                  </ProFormText>
                  <ProFormDigit
                    name="font_size"
                    label={intl.formatMessage({
                      id: 'plugin.titleimage.font-size',
                    })}
                    width="lg"
                    placeholder={intl.formatMessage({
                      id: 'plugin.titleimage.font-size.placeholder',
                    })}
                  />
                  {!setting.bg_images && (
                    <ProFormRadio.Group
                      name="noise"
                      label={intl.formatMessage({
                        id: 'plugin.titleimage.noise',
                      })}
                      options={[
                        {
                          value: false,
                          label: intl.formatMessage({
                            id: 'plugin.titleimage.noise.no',
                          }),
                        },
                        {
                          value: true,
                          label: intl.formatMessage({
                            id: 'plugin.titleimage.noise.yes',
                          }),
                        },
                      ]}
                      extra={intl.formatMessage({
                        id: 'plugin.titleimage.noise.description',
                      })}
                    />
                  )}
                  <ProFormText
                    label={intl.formatMessage({
                      id: 'plugin.titleimage.bg-image',
                    })}
                    extra={intl.formatMessage({
                      id: 'plugin.titleimage.bg-image.description',
                    })}
                  >
                    <div className="insert-image">
                      <Row gutter={[16, 16]} className="image-list">
                        {setting.bg_images?.map((item: any, index: number) => (
                          <Col span={4} key={index}>
                            <div className="image-item">
                              <div className="inner">
                                <div className="link">
                                  <Image
                                    className="img"
                                    preview={true}
                                    src={
                                      (initialState?.system?.base_url || '') +
                                      '/' +
                                      item
                                    }
                                  />
                                  <span
                                    className="close"
                                    onClick={() => handleRemoveBgImage(index)}
                                  >
                                    <FormattedMessage id="setting.system.delete" />
                                  </span>
                                </div>
                              </div>
                            </div>
                          </Col>
                        ))}
                        <Col span={4}>
                          <div className="image-item">
                            <div className="inner">
                              <div className="link">
                                <Upload
                                  name="file"
                                  className="logo-uploader"
                                  showUploadList={false}
                                  accept=".jpg,.jpeg,.png,.gif,.webp,.bmp"
                                  customRequest={async (e) =>
                                    handleUploadFile('bg_image', e)
                                  }
                                >
                                  <div className="ant-upload-item">
                                    <div className="add">
                                      <PlusOutlined />
                                      <div style={{ marginTop: 8 }}>
                                        <FormattedMessage id="plugin.titleimage.bg-image.upload" />
                                      </div>
                                    </div>
                                  </div>
                                </Upload>
                              </div>
                            </div>
                          </div>
                        </Col>
                      </Row>
                    </div>
                  </ProFormText>
                  <ProFormText
                    label={intl.formatMessage({ id: 'plugin.titleimage.font' })}
                  >
                    <Upload
                      name="file"
                      className="logo-uploader"
                      showUploadList={false}
                      accept=".ttf"
                      customRequest={async (e) =>
                        handleUploadFile('font_path', e)
                      }
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
                </div>
                <p>
                  <br />
                </p>
              </ProForm>
            )}
          </Col>
          <Col span={12}>
            <div className="preview-text">
              <Button
                onClick={() => {
                  setVisible(true);
                }}
              >
                <FormattedMessage id="plugin.titleimage.preview.text.edit" />
              </Button>
            </div>
            <img className="preview" src={previewData} />
          </Col>
        </Row>
      </Card>
      {visible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.titleimage.preview.text' })}
          open={visible}
          modalProps={{
            onCancel: () => setVisible(false),
          }}
          onFinish={confirmChangeText}
        >
          <ProFormText name="text" />
        </ModalForm>
      )}
      <VipModal />
    </NewContainer>
  );
};

export default PluginTitleImage;
