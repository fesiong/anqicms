import AttachmentSelect from '@/components/attachment';
import { useVipModal } from '@/components/vipModal';
import {
  checkOpenAIApi,
  getAiGenerateSetting,
  saveAiGenerateSetting,
} from '@/services';
import { getCategories } from '@/services/category';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage } from '@umijs/max';
import { Col, Image, Input, Row, Space, Tag, message } from 'antd';
import React, { useEffect, useState } from 'react';
import { useIntl } from 'react-intl';
import './index.less';

export type CollectorSettingProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const CollectorSetting: React.FC<CollectorSettingProps> = (props) => {
  const { isVip, checkVip, VipModal } = useVipModal();
  const [visible, setVisible] = useState<boolean>(false);
  const [fetched, setFetched] = useState<boolean>(false);
  const [setting, setSetting] = useState<any>({});
  const [tmpInput, setTmpInput] = useState<any>({});
  const [insertImage, setInsertImage] = useState<number>(0);
  const [aiEngine, setAiEngine] = useState<string>('');
  const [categories, setCategories] = useState<any[]>([]);
  const intl = useIntl();

  useEffect(() => {
    getAiGenerateSetting().then((res) => {
      const setting = res.data;
      if (!setting.content_replace) {
        setting.content_replace = [];
      }
      if (!setting.open_ai_keys) {
        setting.open_ai_keys = [];
      }
      if (!setting.images) {
        setting.images = [];
      }
      setSetting(setting);
      setFetched(true);
      setInsertImage(setting.insert_image);
      setAiEngine(setting.ai_engine || '');
    });
    getCategories({ type: 1 }).then((res) => {
      setCategories(res.data || []);
    });
  }, []);

  const handleSetVisible = (visible: boolean) => {
    setVisible(visible);
  };

  const handleSubmit = async (values: any) => {
    if (values.demand.length > 2000) {
      message.error(
        intl.formatMessage({
          id: 'plugin.aigenerate.demand.required',
        }),
      );
      return;
    }
    const postData = Object.assign(setting, values);

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveAiGenerateSetting(postData)
      .then((res) => {
        message.info(res.msg);
        handleSetVisible(false);
        props.onCancel();
      })
      .finally(() => {
        hide();
      });
  };

  const handleRemove = (field: string, index: number) => {
    setting[field].splice(index, 1);
    setSetting({ ...setting });
  };

  const handleChangeTmpInput = (field: string, e: any) => {
    tmpInput[field] = e.target.value;
    setTmpInput({ ...tmpInput });
  };

  const handleAddField = (field: string) => {
    if (field === 'content_replace') {
      if (!tmpInput['from'] || tmpInput['from'] === tmpInput['to']) {
        return;
      }
      let exists = false;
      for (const item of setting[field]) {
        if (item.from === tmpInput['from']) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting[field].push({
          from: tmpInput['from'],
          to: tmpInput['to'],
        });

        tmpInput['from'] = '';
        tmpInput['to'] = '';
      }
    } else if (field === 'open_ai_keys') {
      if (!tmpInput['key']) {
        return;
      }
      let exists = false;
      for (const item of setting[field]) {
        if (item.key === tmpInput['key']) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting[field].push({
          key: tmpInput['key'],
          invalid: false,
        });

        tmpInput['key'] = '';
      }
    } else {
      setting[field].push(tmpInput[field]);
      tmpInput[field] = '';
    }
    setTmpInput({ ...tmpInput });
    setSetting({ ...setting });
  };

  const onChangeInsertImage = (e: any) => {
    setInsertImage(e.target.value);
  };

  const handleSelectLogo = (rows: any) => {
    for (const row of rows) {
      let exists = false;
      for (let i in setting['images']) {
        if (setting['images'][i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        setting['images'].push(row.file_path);
      }
    }
    setSetting({ ...setting });
  };

  const handleChangeAiEngine = (e: any) => {
    setAiEngine(e.target.value);
  };

  const handleCheckOpenAIApi = () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'plugin.aigenerate.checking' }),
      0,
    );
    checkOpenAIApi()
      .then((res) => {
        if (res.code === 0) {
          message.success(res.msg);
          setting.api_valid = true;
        } else {
          message.error(res.msg);
          setting.api_valid = false;
        }
        setSetting({ ...setting });
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <>
      <VipModal />
      <div
        onClick={() => {
          handleSetVisible(!visible);
        }}
      >
        {props.children}
      </div>
      {fetched && (
        <ModalForm
          width={800}
          title={intl.formatMessage({
            id: 'plugin.aigenerate.setting',
          })}
          initialValues={setting}
          open={visible}
          //layout="horizontal"
          onOpenChange={(flag) => {
            handleSetVisible(flag);
            if (!flag) {
              props.onCancel(flag);
            }
          }}
          onFinish={async (values) => {
            handleSubmit(values);
          }}
        >
          <ProFormRadio.Group
            name="open"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.isopen',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.isopen.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.isopen.yes',
                }),
                value: true,
              },
            ]}
          />
          <ProFormRadio.Group
            name="language"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.language',
            })}
            options={[
              {
                cn: '英语',
                value: 'en',
                label: 'en',
              },
              {
                cn: '简体中文',
                value: 'zh-CN',
                label: 'zh-cn',
              },
            ].map((item) => {
              return {
                label: intl.formatMessage({
                  id: 'content.translate.' + item.label,
                }),
                value: item.value,
              };
            })}
          />
          <ProFormRadio.Group
            name="double_title"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.double-title',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.no',
                }),
                value: false,
              },
              {
                label: intl.formatMessage({
                  id: 'content.module.field.isfilter.yes',
                }),
                value: true,
              },
            ]}
            extra={intl.formatMessage({
              id: 'plugin.aigenerate.double-title.description',
            })}
          />
          <ProFormRadio.Group
            name="double_split"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.double-split',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.bracket',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.line',
                }),
                value: 1,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.question',
                }),
                value: 2,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.comma',
                }),
                value: 3,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.colon',
                }),
                value: 4,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.double-split.random',
                }),
                value: 5,
              },
            ]}
          />
          <ProFormTextArea
            name="demand"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.demand',
            })}
            fieldProps={{
              maxLength: 1000,
            }}
            extra={intl.formatMessage({
              id: 'plugin.aigenerate.demand.description',
            })}
          />
          <ProFormRadio.Group
            name="ai_engine"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.source',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.source.anqicms',
                }),
                value: '',
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.source.openai',
                }),
                value: 'openai',
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.source.deepseek',
                }),
                value: 'deepseek',
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.source.spark',
                }),
                value: 'spark',
              },
            ]}
            fieldProps={{
              onChange: (e) => {
                handleChangeAiEngine(e);
              },
            }}
            extra={
              <div>
                <span>
                  <FormattedMessage id="plugin.aigenerate.source.description" />
                </span>
                <Tag
                  style={{ marginLeft: 10 }}
                  className="link"
                  onClick={handleCheckOpenAIApi}
                >
                  <FormattedMessage id="plugin.aigenerate.source.check-openai" />
                </Tag>
              </div>
            }
            disabled={isVip === false}
          />
          {!isVip ? (
            <div
              className="link mb-normal"
              onClick={() => {
                checkVip(() => {});
              }}
            >
              <FormattedMessage id="plugin.aigenerate.vip-tip" />
            </div>
          ) : null}
          {(aiEngine === 'openai' || aiEngine === 'deepseek') && (
            <>
              <ProFormText
                name={'open_ai_api'}
                label={intl.formatMessage({
                  id: 'plugin.aigenerate.openai.base-url',
                })}
                extra={intl.formatMessage({
                  id:
                    aiEngine === 'deepseek'
                      ? 'plugin.aigenerate.openai.base-url.deepseek'
                      : 'plugin.aigenerate.openai.base-url.openai',
                })}
              />
              <ProFormText
                name={'open_ai_model'}
                label={intl.formatMessage({
                  id: 'plugin.aigenerate.openai.model',
                })}
                extra={intl.formatMessage({
                  id:
                    aiEngine === 'deepseek'
                      ? 'plugin.aigenerate.openai.model.deepseek'
                      : 'plugin.aigenerate.openai.model.openai',
                })}
              />
              <ProFormText
                label="API Keys"
                extra={
                  <div>
                    <div className="text-muted">
                      <div>
                        <span className="text-red">*</span>
                        <FormattedMessage id="plugin.aigenerate.openai.description" />
                      </div>
                    </div>
                    <div className="tag-lists">
                      <Space size={[12, 12]} wrap>
                        {setting.open_ai_keys?.map(
                          (tag: any, index: number) => (
                            <span className="edit-tag" key={index}>
                              <span className="key">{tag.key}</span>
                              <span className="divide">
                                <span className="value">
                                  {tag.invalid
                                    ? intl.formatMessage({
                                        id: 'plugin.aigenerate.openai.invalid',
                                      })
                                    : intl.formatMessage({
                                        id: 'plugin.aigenerate.openai.valid',
                                      })}
                                </span>
                              </span>
                              <span
                                className="close"
                                onClick={() =>
                                  handleRemove('open_ai_keys', index)
                                }
                              >
                                ×
                              </span>
                            </span>
                          ),
                        )}
                      </Space>
                    </div>
                  </div>
                }
              >
                <Input.Group compact>
                  <Input
                    value={tmpInput.key || ''}
                    onChange={(e) => handleChangeTmpInput('key', e)}
                    onPressEnter={() => handleAddField('open_ai_keys')}
                    suffix={
                      <a onClick={() => handleAddField('open_ai_keys')}>
                        <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                      </a>
                    }
                  />
                </Input.Group>
              </ProFormText>
            </>
          )}
          {aiEngine === 'spark' && (
            <>
              <div className="mb-normal">
                <FormattedMessage id="plugin.aigenerate.spark.description" />:
                <a
                  href="https://xinghuo.xfyun.cn/sparkapi?ch=gjp"
                  target="_blank"
                  rel="noreferrer"
                >
                  https://xinghuo.xfyun.cn/sparkapi?ch=gjp
                </a>
              </div>
              <ProFormRadio.Group
                name={['spark', 'version']}
                label={intl.formatMessage({
                  id: 'plugin.aigenerate.spark.version',
                })}
                options={[
                  { label: 'Spark Lite(Free)', value: '1.5' },
                  { label: 'Spark Pro', value: '3.0' },
                  { label: 'Spark Max', value: '3.5' },
                  { label: 'Spark4.0 Ultra', value: '4.0' },
                ]}
              />
              <ProFormText name={['spark', 'app_id']} label="APPID" />
              <ProFormText name={['spark', 'api_secret']} label="APISecret" />
              <ProFormText name={['spark', 'api_key']} label="APIKey" />
            </>
          )}
          <ProFormSelect
            label={intl.formatMessage({
              id: 'plugin.aigenerate.default-category',
            })}
            name="category_ids"
            mode="multiple"
            required
            extra={intl.formatMessage({
              id: 'plugin.aigenerate.default-category.description',
            })}
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
          />
          <ProFormRadio.Group
            name="save_type"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.save-type',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.save-type.draft',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.save-type.release',
                }),
                value: 1,
              },
            ]}
          />
          <ProForm.Group>
            <ProFormDigit
              name="start_hour"
              label={intl.formatMessage({
                id: 'plugin.aigenerate.start-time',
              })}
              placeholder=""
              extra={intl.formatMessage({
                id: 'plugin.aigenerate.start-time.description',
              })}
            />
            <ProFormDigit
              name="end_hour"
              label={intl.formatMessage({
                id: 'plugin.aigenerate.end-time',
              })}
              placeholder=""
              extra={intl.formatMessage({
                id: 'plugin.aigenerate.end-time.description',
              })}
            />
            <ProFormDigit
              name="daily_limit"
              label={intl.formatMessage({
                id: 'plugin.aigenerate.daily-limit',
              })}
              placeholder=""
              extra={intl.formatMessage({
                id: 'plugin.aigenerate.daily-limit.description',
              })}
            />
          </ProForm.Group>
          <ProFormRadio.Group
            name="insert_image"
            label={intl.formatMessage({
              id: 'plugin.aigenerate.insert-image',
            })}
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.insert-image.default',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.insert-image.diy',
                }),
                value: 2,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.aigenerate.insert-image.category',
                }),
                value: 3,
              },
            ]}
            fieldProps={{
              onChange: (e) => {
                onChangeInsertImage(e);
              },
            }}
          />
          {insertImage === 2 && (
            <ProFormText
              label={intl.formatMessage({
                id: 'plugin.aigenerate.insert-image.list',
              })}
            >
              <div className="insert-image">
                <Row gutter={[16, 16]} className="image-list">
                  {setting.images?.map((item: any, index: number) => (
                    <Col span={4} key={index}>
                      <div className="image-item">
                        <div className="inner">
                          <div className="link">
                            <Image className="img" preview={true} src={item} />
                            <span
                              className="close"
                              onClick={() => handleRemove('images', index)}
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
                          <AttachmentSelect
                            onSelect={handleSelectLogo}
                            open={false}
                            multiple={true}
                          >
                            <div className="ant-upload-item">
                              <div className="add">
                                <PlusOutlined />
                                <div style={{ marginTop: 8 }}>
                                  <FormattedMessage id="setting.system.upload" />
                                </div>
                              </div>
                            </div>
                          </AttachmentSelect>
                        </div>
                      </div>
                    </div>
                  </Col>
                </Row>
              </div>
            </ProFormText>
          )}
          {insertImage === 3 && (
            <ProFormSelect
              label={intl.formatMessage({
                id: 'plugin.aigenerate.image.category',
              })}
              name="image_category_id"
              required
              extra={intl.formatMessage({
                id: 'plugin.aigenerate.image.category.description',
              })}
              request={async () => {
                const res = await getCategories();
                const data = (res.data || []).concat(
                  {
                    id: 0,
                    title: intl.formatMessage({
                      id: 'plugin.aigenerate.image.category.default',
                    }),
                  },
                  {
                    id: -1,
                    title: intl.formatMessage({
                      id: 'plugin.aigenerate.image.category.all',
                    }),
                  },
                  {
                    id: -2,
                    title: intl.formatMessage({
                      id: 'plugin.aigenerate.image.category.match',
                    }),
                  },
                );
                return data;
              }}
              fieldProps={{
                fieldNames: {
                  label: 'title',
                  value: 'id',
                },
              }}
            />
          )}
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.aigenerate.replace',
            })}
            extra={
              <div>
                <div className="text-muted">
                  <p>
                    <FormattedMessage id="plugin.aigenerate.replace.tips1" />
                  </p>
                  <p>
                    <FormattedMessage id="plugin.aigenerate.replace.tips2" />
                  </p>
                  <p>
                    <FormattedMessage id="plugin.aigenerate.replace.tips3" />
                  </p>
                  <p>
                    <FormattedMessage id="plugin.aigenerate.replace.rules" />
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.email" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.date" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.time" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.cellphone" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.qq" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.wechat" />
                    </Tag>
                    、
                    <Tag>
                      <FormattedMessage id="plugin.aigenerate.replace.rule.website" />
                    </Tag>
                  </p>
                  <div>
                    <span className="text-red">*</span>{' '}
                    <FormattedMessage id="plugin.aigenerate.replace.notice" />
                  </div>
                </div>
                <div className="tag-lists">
                  <Space size={[12, 12]} wrap>
                    {setting.content_replace?.map((tag: any, index: number) => (
                      <span className="edit-tag" key={index}>
                        <span className="key">{tag.from}</span>
                        <span className="divide">
                          <FormattedMessage id="plugin.aigenerate.replace.to" />
                        </span>
                        <span className="value">
                          {tag.to ||
                            intl.formatMessage({
                              id: 'plugin.aigenerate.empty',
                            })}
                        </span>
                        <span
                          className="close"
                          onClick={() => handleRemove('content_replace', index)}
                        >
                          ×
                        </span>
                      </span>
                    ))}
                  </Space>
                </div>
              </div>
            }
          >
            <Input.Group compact>
              <Input
                style={{ width: '40%' }}
                value={tmpInput.from || ''}
                onChange={(e) => handleChangeTmpInput('from', e)}
                onPressEnter={() => handleAddField('content_replace')}
              />
              <span className="input-divide">
                <FormattedMessage id="plugin.aigenerate.replace.to" />
              </span>
              <Input
                style={{ width: '50%' }}
                value={tmpInput.to || ''}
                onChange={(e) => handleChangeTmpInput('to', e)}
                onPressEnter={() => handleAddField('content_replace')}
                suffix={
                  <a onClick={() => handleAddField('content_replace')}>
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                }
              />
            </Input.Group>
          </ProFormText>
        </ModalForm>
      )}
    </>
  );
};

export default CollectorSetting;
