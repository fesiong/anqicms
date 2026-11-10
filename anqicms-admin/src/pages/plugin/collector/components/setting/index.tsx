import AttachmentSelect from '@/components/attachment';
import { getCategories } from '@/services/category';
import {
  getCollectorSetting,
  saveCollectorSetting,
} from '@/services/collector';
import { supportLanguages } from '@/utils';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, injectIntl } from '@umijs/max';
import { Col, Image, Input, Row, Space, Tag, message } from 'antd';
import React from 'react';
import { IntlShape } from 'react-intl';
import './index.less';

export type CollectorSettingProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
  intl: IntlShape;
};

class CollectorSetting extends React.Component<CollectorSettingProps> {
  state: { [key: string]: any } = {
    visible: false,
    fetched: false,
    setting: {},
    tmpInput: {},
    insertImage: 0,
    collect_mode: 0,
    proxyOpen: false,
    categories: [],
  };

  componentDidMount() {
    getCollectorSetting().then((res) => {
      const setting = res.data;
      if (!setting.title_exclude) {
        setting.title_exclude = [];
      }
      if (!setting.title_exclude_prefix) {
        setting.title_exclude_prefix = [];
      }
      if (!setting.title_exclude_suffix) {
        setting.title_exclude_suffix = [];
      }
      if (!setting.content_exclude_line) {
        setting.content_exclude_line = [];
      }
      if (!setting.content_exclude) {
        setting.content_exclude = [];
      }
      if (!setting.link_exclude) {
        setting.link_exclude = [];
      }
      if (!setting.content_replace) {
        setting.content_replace = [];
      }
      if (!setting.images) {
        setting.images = [];
      }
      this.setState({
        setting: setting,
        fetched: true,
        insertImage: setting.insert_image,
        collect_mode: setting.collect_mode || 0,
        proxyOpen: setting.proxy_config?.open || false,
      });
    });
    getCategories().then((res) => {
      this.setState({
        categories: res.data,
      });
    });
  }

  handleSetVisible = (visible: boolean) => {
    this.setState({
      visible,
    });
  };

  handleSubmit = async (values: any) => {
    const { setting } = this.state;
    const postData = Object.assign(setting, values);

    const hide = message.loading(
      this.props.intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveCollectorSetting(postData)
      .then((res) => {
        message.info(res.msg);
        this.handleSetVisible(false);
        this.props.onCancel();
      })
      .finally(() => {
        hide();
      });
  };

  handleRemove = (field: string, index: number) => {
    const { setting } = this.state;
    setting[field].splice(index, 1);
    this.setState({
      setting,
    });
  };

  handleChangeTmpInput = (field: string, e: any) => {
    const { tmpInput } = this.state;
    tmpInput[field] = e.target.value;
    this.setState({
      tmpInput,
    });
  };

  handleAddField = (field: string) => {
    const { tmpInput, setting } = this.state;
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
    } else {
      setting[field].push(tmpInput[field]);
      tmpInput[field] = '';
    }
    this.setState({
      tmpInput,
      setting,
    });
  };

  onChangeInsertImage = (e: any) => {
    this.setState({
      insertImage: e.target.value,
    });
  };

  handleSelectLogo = (rows: any) => {
    const { setting } = this.state;
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
    this.setState({
      setting,
    });
  };

  setCollectMode = (mode: number) => {
    this.setState({
      collect_mode: mode,
    });
  };

  setproxyOpen = (isOpen: boolean) => {
    this.setState({
      proxyOpen: isOpen,
    });
  };

  render() {
    const {
      visible,
      fetched,
      setting,
      tmpInput,
      insertImage,
      collect_mode,
      proxyOpen,
      categories,
    } = this.state;

    return (
      <>
        <div
          onClick={() => {
            this.handleSetVisible(!visible);
          }}
        >
          {this.props.children}
        </div>
        {fetched && (
          <ModalForm
            width={800}
            title={this.props.intl.formatMessage({
              id: 'plugin.collector.setting',
            })}
            initialValues={setting}
            open={visible}
            //layout="horizontal"
            onOpenChange={(flag) => {
              this.handleSetVisible(flag);
              if (!flag) {
                this.props.onCancel(flag);
              }
            }}
            onFinish={async (values) => {
              this.handleSubmit(values);
            }}
          >
            <ProFormRadio.Group
              name="auto_collect"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.auto-collect',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.auto-collect.no',
                  }),
                  value: false,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.auto-collect.yes',
                  }),
                  value: true,
                },
              ]}
            />
            <ProFormRadio.Group
              name="language"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.language',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'content.translate.zh-cn',
                  }),
                  value: 'zh-CN',
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'content.translate.en',
                  }),
                  value: 'en',
                },
              ]}
            />
            <ProFormRadio.Group
              name="collect_mode"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.mode',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.mode.article',
                  }),
                  value: 0,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.mode.ask',
                  }),
                  value: 1,
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  this.setCollectMode(e.target.value);
                },
              }}
              extra={
                <div>
                  <FormattedMessage id="plugin.collector.mode.description" />
                </div>
              }
            />
            {collect_mode === 0 && (
              <ProFormText
                name="from_website"
                label={this.props.intl.formatMessage({
                  id: 'plugin.collector.source',
                })}
                placeholder={this.props.intl.formatMessage({
                  id: 'plugin.collector.source.placeholder',
                })}
                extra={
                  <div>
                    <FormattedMessage id="plugin.collector.source.description" />
                  </div>
                }
              />
            )}
            <ProFormSelect
              label={this.props.intl.formatMessage({
                id: 'plugin.aigenerate.default-category',
              })}
              name="category_ids"
              mode="multiple"
              required
              extra={
                <div>
                  <FormattedMessage id="plugin.collector.category.description" />
                  <span className="text-red">
                    <FormattedMessage id="plugin.collector.category.notice" />
                  </span>
                </div>
              }
              options={[
                {
                  title: this.props.intl.formatMessage({
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
              name={['proxy_config', 'open']}
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.proxy_config.name',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.proxy_config.close',
                  }),
                  value: false,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.proxy_config.open',
                  }),
                  value: true,
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  this.setproxyOpen(e.target.value);
                },
              }}
            />
            {proxyOpen && (
              <>
                <ProFormRadio.Group
                  name={['proxy_config', 'platform']}
                  label={this.props.intl.formatMessage({
                    id: 'plugin.collector.proxy_config.platform',
                  })}
                  readonly
                  fieldProps={{
                    value: 'juliangip',
                  }}
                  options={[
                    {
                      label: this.props.intl.formatMessage({
                        id: 'plugin.collector.proxy_config.platform.juliang',
                      }),
                      value: 'juliangip',
                    },
                  ]}
                  extra={
                    <div>
                      {this.props.intl.formatMessage({
                        id: 'plugin.collector.proxy_config.platform.description',
                      })}
                      <a
                        href="https://www.juliangip.com/user/reg?inviteCode=1043204"
                        target="_blank"
                        rel="noreferrer"
                      >
                        https://www.juliangip.com/user/reg?inviteCode=1043204
                      </a>
                    </div>
                  }
                />
                <ProFormText
                  name={['proxy_config', 'api_url']}
                  label={this.props.intl.formatMessage({
                    id: 'plugin.collector.proxy_config.api_url',
                  })}
                  extra={
                    <div>
                      <FormattedMessage id="plugin.collector.proxy_config.api_url.description" />
                    </div>
                  }
                />
                <ProForm.Group>
                  <ProFormDigit
                    name={['proxy_config', 'concurrent']}
                    label={this.props.intl.formatMessage({
                      id: 'plugin.collector.concurrent',
                    })}
                    placeholder={this.props.intl.formatMessage({
                      id: 'plugin.collector.concurrent.placeholder',
                    })}
                    extra={this.props.intl.formatMessage({
                      id: 'plugin.collector.concurrent.description',
                    })}
                  />
                  <ProFormDigit
                    name={['proxy_config', 'expire']}
                    label={this.props.intl.formatMessage({
                      id: 'plugin.collector.expire',
                    })}
                    addonAfter={this.props.intl.formatMessage({
                      id: 'plugin.collector.expire.addon',
                    })}
                    placeholder={this.props.intl.formatMessage({
                      id: 'plugin.collector.expire.placeholder',
                    })}
                    extra={this.props.intl.formatMessage({
                      id: 'plugin.collector.expire.description',
                    })}
                  />
                </ProForm.Group>
              </>
            )}
            <ProFormRadio.Group
              name="save_type"
              label={this.props.intl.formatMessage({
                id: 'plugin.aigenerate.save-type',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.aigenerate.save-type.draft',
                  }),
                  value: 0,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.aigenerate.save-type.release',
                  }),
                  value: 1,
                },
              ]}
            />
            <ProFormDigit
              name="title_min_length"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.min-title',
              })}
              placeholder={this.props.intl.formatMessage({
                id: 'plugin.collector.min-title.placeholder',
              })}
              extra={this.props.intl.formatMessage({
                id: 'plugin.collector.min-title.description',
              })}
            />
            <ProFormDigit
              name="content_min_length"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.min-content',
              })}
              placeholder={this.props.intl.formatMessage({
                id: 'plugin.collector.min-content.placeholder',
              })}
              extra={this.props.intl.formatMessage({
                id: 'plugin.collector.min-content.description',
              })}
            />
            <ProFormRadio.Group
              name="auto_pseudo"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.pseudo',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.pseudo.no',
                  }),
                  value: false,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.pseudo.yes',
                  }),
                  value: true,
                },
              ]}
              extra={this.props.intl.formatMessage({
                id: 'plugin.collector.pseudo.description',
              })}
            />
            <ProFormRadio.Group
              name="auto_translate"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.translate',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.translate.no',
                  }),
                  value: false,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.translate.yes',
                  }),
                  value: true,
                },
              ]}
              extra={this.props.intl.formatMessage({
                id: 'plugin.collector.translate.description',
              })}
            />
            <ProFormSelect
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.to-language',
              })}
              name="to_language"
              extra={this.props.intl.formatMessage({
                id: 'plugin.collector.to-language.description',
              })}
              options={supportLanguages.map((item) => {
                return {
                  label: item.label,
                  value: item.value,
                };
              })}
            />
            <ProForm.Group>
              <ProFormDigit
                name="start_hour"
                label={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.start-time',
                })}
                placeholder={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.start-time.placeholder',
                })}
                extra={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.start-time.description',
                })}
              />
              <ProFormDigit
                name="end_hour"
                label={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.end-time',
                })}
                placeholder={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.end-time.placeholder',
                })}
                extra={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.end-time.description',
                })}
              />
              <ProFormDigit
                name="daily_limit"
                label={this.props.intl.formatMessage({
                  id: 'plugin.collector.daily-limit',
                })}
                extra={this.props.intl.formatMessage({
                  id: 'plugin.collector.daily-limit.description',
                })}
              />
            </ProForm.Group>
            <ProFormRadio.Group
              name="insert_image"
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.insert-image',
              })}
              options={[
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.insert-image.remove',
                  }),
                  value: 0,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.collector.insert-image.contain',
                  }),
                  value: 1,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.aigenerate.insert-image.diy',
                  }),
                  value: 2,
                },
                {
                  label: this.props.intl.formatMessage({
                    id: 'plugin.aigenerate.insert-image.category',
                  }),
                  value: 3,
                },
              ]}
              fieldProps={{
                onChange: (e) => {
                  this.onChangeInsertImage(e);
                },
              }}
            />
            {insertImage === 2 && (
              <ProFormText
                label={this.props.intl.formatMessage({
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
                              <Image
                                className="img"
                                preview={true}
                                src={item}
                              />
                              <span
                                className="close"
                                onClick={this.handleRemove.bind(
                                  this,
                                  'images',
                                  index,
                                )}
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
                              onSelect={this.handleSelectLogo}
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
                label={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.image.category',
                })}
                name="image_category_id"
                required
                extra={this.props.intl.formatMessage({
                  id: 'plugin.aigenerate.image.category.description',
                })}
                request={async () => {
                  const res = await getCategories();
                  const data = (res.data || []).concat(
                    {
                      id: 0,
                      title: this.props.intl.formatMessage({
                        id: 'plugin.aigenerate.image.category.default',
                      }),
                    },
                    {
                      id: -1,
                      title: this.props.intl.formatMessage({
                        id: 'plugin.aigenerate.image.category.all',
                      }),
                    },
                    {
                      id: -2,
                      title: this.props.intl.formatMessage({
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
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.title-exclude',
              })}
              fieldProps={{
                value: tmpInput.title_exclude || '',
                onChange: this.handleChangeTmpInput.bind(this, 'title_exclude'),
                onPressEnter: this.handleAddField.bind(this, 'title_exclude'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'title_exclude')}>
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.title-exclude.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(
                              this,
                              'title_exclude',
                              index,
                            )}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.title-prefix',
              })}
              fieldProps={{
                value: tmpInput.title_exclude_prefix || '',
                onChange: this.handleChangeTmpInput.bind(
                  this,
                  'title_exclude_prefix',
                ),
                onPressEnter: this.handleAddField.bind(
                  this,
                  'title_exclude_prefix',
                ),
                suffix: (
                  <a
                    onClick={this.handleAddField.bind(
                      this,
                      'title_exclude_prefix',
                    )}
                  >
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.title-prefix.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude_prefix?.map(
                        (tag: any, index: number) => (
                          <span className="edit-tag" key={index}>
                            <span className="key">{tag}</span>
                            <span
                              className="close"
                              onClick={this.handleRemove.bind(
                                this,
                                'title_exclude_prefix',
                                index,
                              )}
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
            />
            <ProFormText
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.title-suffix',
              })}
              fieldProps={{
                value: tmpInput.title_exclude_suffix || '',
                onChange: this.handleChangeTmpInput.bind(
                  this,
                  'title_exclude_suffix',
                ),
                onPressEnter: this.handleAddField.bind(
                  this,
                  'title_exclude_suffix',
                ),
                suffix: (
                  <a
                    onClick={this.handleAddField.bind(
                      this,
                      'title_exclude_suffix',
                    )}
                  >
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.title-suffix.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.title_exclude_suffix?.map(
                        (tag: any, index: number) => (
                          <span className="edit-tag" key={index}>
                            <span className="key">{tag}</span>
                            <span
                              className="close"
                              onClick={this.handleRemove.bind(
                                this,
                                'title_exclude_suffix',
                                index,
                              )}
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
            />
            <ProFormText
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.content-exclude-line',
              })}
              fieldProps={{
                value: tmpInput.content_exclude_line || '',
                onChange: this.handleChangeTmpInput.bind(
                  this,
                  'content_exclude_line',
                ),
                onPressEnter: this.handleAddField.bind(
                  this,
                  'content_exclude_line',
                ),
                suffix: (
                  <a
                    onClick={this.handleAddField.bind(
                      this,
                      'content_exclude_line',
                    )}
                  >
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.content-exclude-line.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.content_exclude_line?.map(
                        (tag: any, index: number) => (
                          <span className="edit-tag" key={index}>
                            <span className="key">{tag}</span>
                            <span
                              className="close"
                              onClick={this.handleRemove.bind(
                                this,
                                'content_exclude_line',
                                index,
                              )}
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
            />
            <ProFormText
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.link-exclude',
              })}
              fieldProps={{
                value: tmpInput.link_exclude || '',
                onChange: this.handleChangeTmpInput.bind(this, 'link_exclude'),
                onPressEnter: this.handleAddField.bind(this, 'link_exclude'),
                suffix: (
                  <a onClick={this.handleAddField.bind(this, 'link_exclude')}>
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.link-exclude.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.link_exclude?.map((tag: any, index: number) => (
                        <span className="edit-tag" key={index}>
                          <span className="key">{tag}</span>
                          <span
                            className="close"
                            onClick={this.handleRemove.bind(
                              this,
                              'link_exclude',
                              index,
                            )}
                          >
                            ×
                          </span>
                        </span>
                      ))}
                    </Space>
                  </div>
                </div>
              }
            />
            <ProFormText
              label={this.props.intl.formatMessage({
                id: 'plugin.collector.content-exclude',
              })}
              fieldProps={{
                value: tmpInput.content_exclude || '',
                onChange: this.handleChangeTmpInput.bind(
                  this,
                  'content_exclude',
                ),
                onPressEnter: this.handleAddField.bind(this, 'content_exclude'),
                suffix: (
                  <a
                    onClick={this.handleAddField.bind(this, 'content_exclude')}
                  >
                    <FormattedMessage id="plugin.aigenerate.enter-to-add" />
                  </a>
                ),
              }}
              extra={
                <div>
                  <div className="text-muted">
                    <FormattedMessage id="plugin.collector.content-exclude.tips" />
                  </div>
                  <div className="tag-lists">
                    <Space size={[12, 12]} wrap>
                      {setting.content_exclude?.map(
                        (tag: any, index: number) => (
                          <span className="edit-tag" key={index}>
                            <span className="key">{tag}</span>
                            <span
                              className="close"
                              onClick={this.handleRemove.bind(
                                this,
                                'content_exclude',
                                index,
                              )}
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
            />
            <ProFormText
              label={this.props.intl.formatMessage({
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
                      {setting.content_replace?.map(
                        (tag: any, index: number) => (
                          <span className="edit-tag" key={index}>
                            <span className="key">{tag.from}</span>
                            <span className="divide">
                              <FormattedMessage id="plugin.aigenerate.replace.to" />
                            </span>
                            <span className="value">
                              {tag.to ||
                                this.props.intl.formatMessage({
                                  id: 'plugin.aigenerate.empty',
                                })}
                            </span>
                            <span
                              className="close"
                              onClick={this.handleRemove.bind(
                                this,
                                'content_replace',
                                index,
                              )}
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
                  style={{ width: '40%' }}
                  value={tmpInput.from || ''}
                  onChange={this.handleChangeTmpInput.bind(this, 'from')}
                  onPressEnter={this.handleAddField.bind(
                    this,
                    'content_replace',
                  )}
                />
                <span className="input-divide">
                  <FormattedMessage id="plugin.aigenerate.replace.to" />
                </span>
                <Input
                  style={{ width: '50%' }}
                  value={tmpInput.to || ''}
                  onChange={this.handleChangeTmpInput.bind(this, 'to')}
                  onPressEnter={this.handleAddField.bind(
                    this,
                    'content_replace',
                  )}
                  suffix={
                    <a
                      onClick={this.handleAddField.bind(
                        this,
                        'content_replace',
                      )}
                    >
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
  }
}

export default injectIntl(CollectorSetting);
