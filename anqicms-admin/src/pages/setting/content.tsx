import AttachmentSelect from '@/components/attachment';
import NewContainer from '@/components/NewContainer';
import { getCategories } from '@/services';
import {
  convertImagetoWebp,
  getSettingContent,
  rebuildThumb,
  saveSettingContent,
} from '@/services/setting';
import { PlusOutlined } from '@ant-design/icons';
import {
  ProForm,
  ProFormGroup,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Divider, message, Modal } from 'antd';
import React, { useEffect, useRef, useState } from 'react';

const SettingContentFrom: React.FC<any> = () => {
  const formRef = useRef<ProFormInstance>();
  const [setting, setSetting] = useState<any>(null);
  const [resize_image, setResizeImage] = useState<number>(0);
  const [defaultThumbType, setDefaultThumbType] = useState<number>(0);
  const [useWebp, setUseWebp] = useState<number>(0);
  const [activeTabKey, setActiveTabKey] = useState<string>('base');
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await getSettingContent();
    let setting = res.data || null;
    setDefaultThumbType(setting?.default_thumb_type || 0);
    setResizeImage(setting?.resize_image || 0);
    setUseWebp(setting?.use_webp || 0);
    setSetting(setting);
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

  const handleSelectDefaultThumb = (rows: any[]) => {
    if (!setting.default_thumbs) {
      setting.default_thumbs = [];
    }
    for (let i = 0; i < rows.length; i++) {
      const row = rows[i];
      if (!setting.default_thumbs.includes(row.file_path)) {
        setting.default_thumbs.push(row.file_path);
      }
    }
    setSetting({
      ...setting,
      default_thumbs: [...setting.default_thumbs],
    });
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleRemoveDefaultThumb = (e: any, index: number) => {
    e.stopPropagation();
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        setting.default_thumbs.splice(index, 1);
        setSetting({
          ...setting,
          default_thumbs: [...setting.default_thumbs],
        });
      },
    });
  };

  const handleConvertToWebp = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.content.confirm-convert-webp' }),
      content: intl.formatMessage({
        id: 'setting.content.confirm-convert-webp.content',
      }),
      onOk: () => {
        convertImagetoWebp({}).then((res) => {
          message.info(res.msg);
        });
      },
    });
  };

  const handleRebuildThumb = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.content.confirm-thumbnal' }),
      content: intl.formatMessage({
        id: 'setting.content.confirm-thumbnal.content',
      }),
      onOk: () => {
        rebuildThumb({}).then((res) => {
          message.info(res.msg);
        });
      },
    });
  };

  const onSubmit = async (values: any) => {
    // 页面按 tab 条件渲染，onFinish 的 values 只含当前 tab 字段，
    // 后端整包覆盖保存会清空其它 tab 的配置，必须取全量表单值
    const allValues: any = {
      ...(formRef.current?.getFieldsValue(true) || values),
    };
    allValues.default_thumb_type = Number(allValues.default_thumb_type); // 0 = default, 1 = category
    allValues.default_thumbs = setting.default_thumbs || [];
    allValues.thumb_category_id = Number(allValues.thumb_category_id);
    allValues.filter_outlink = Number(allValues.filter_outlink);
    allValues.url_token_type = Number(allValues.url_token_type);
    allValues.remote_download = Number(allValues.remote_download);
    allValues.resize_image = Number(allValues.resize_image);
    allValues.resize_width = Number(allValues.resize_width);
    allValues.thumb_crop = Number(allValues.thumb_crop);
    allValues.thumb_width = Number(allValues.thumb_width);
    allValues.thumb_height = Number(allValues.thumb_height);
    allValues.quality = Number(allValues.quality);
    allValues.use_sort = Number(allValues.use_sort);
    allValues.max_page = Number(allValues.max_page);
    allValues.max_limit = Number(allValues.max_limit);

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    saveSettingContent(allValues)
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
        <ProFormRadio.Group
          name="editor"
          label={intl.formatMessage({ id: 'setting.content.editor' })}
          options={[
            {
              value: '',
              label: intl.formatMessage({
                id: 'setting.content.editor.fulltext',
              }),
            },
            {
              value: 'markdown',
              label: intl.formatMessage({
                id: 'setting.content.editor.markdown',
              }),
            },
            {
              value: 'simple',
              label: intl.formatMessage({
                id: 'setting.content.editor.simple',
              }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.content.editor.description',
          })}
        />
        <ProFormRadio.Group
          name="remote_download"
          label={intl.formatMessage({
            id: 'setting.content.remote-download',
          })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
        />
        <ProFormRadio.Group
          name="filter_outlink"
          label={intl.formatMessage({
            id: 'setting.content.outlink-filter',
          })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.outlink-filter.allow',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.content.outlink-filter.remove',
              }),
            },
            {
              value: 2,
              label: intl.formatMessage({
                id: 'setting.content.outlink-filter.nofollow',
              }),
            },
          ]}
        />
      </div>
    ),
    rule: (
      <div>
        <ProFormRadio.Group
          name="url_token_type"
          label={intl.formatMessage({ id: 'setting.content.urltoken' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.urltoken.long',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.content.urltoken.short',
              }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.content.urltoken.description',
          })}
        />
        <ProFormRadio.Group
          name="multi_category"
          label={intl.formatMessage({
            id: 'setting.content.multi-category',
          })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.content.multi-category.description',
          })}
        />
        <ProFormRadio.Group
          name="use_sort"
          label={intl.formatMessage({ id: 'setting.content.archive-sort' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.content.archive-sort.description',
          })}
        />
        <ProFormRadio.Group
          name="match_tag"
          label={intl.formatMessage({ id: 'setting.content.auto-match-tag' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={intl.formatMessage({
            id: 'setting.content.auto-match-tag.description',
          })}
        />
        <ProFormText
          name="max_page"
          label={intl.formatMessage({ id: 'setting.content.max-page' })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.content.max-page.placeholder',
          })}
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.content.max-page.suffix',
            }),
          }}
          extra={intl.formatMessage({
            id: 'setting.content.max-page.description',
          })}
        />
        <ProFormText
          name="max_limit"
          label={intl.formatMessage({ id: 'setting.content.max-limit' })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.content.max-limit.placeholder',
          })}
          fieldProps={{
            suffix: intl.formatMessage({
              id: 'setting.content.max-limit.suffix',
            }),
          }}
          extra={intl.formatMessage({
            id: 'setting.content.max-limit.description',
          })}
        />
      </div>
    ),
    image: (
      <div>
        <ProFormRadio.Group
          name="img_alt_auto"
          label={intl.formatMessage({ id: 'setting.content.img-alt-auto' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          extra={
            <FormattedMessage id="setting.content.img-alt-auto.description" />
          }
        />
        <ProFormRadio.Group
          name="use_webp"
          label={intl.formatMessage({ id: 'setting.content.use-webp' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
          fieldProps={{
            onChange: (e) => {
              setUseWebp(e.target.value);
            },
          }}
          extra={
            <div>
              <span>
                <FormattedMessage id="setting.content.use-webp.description" />
              </span>
              <span>
                <FormattedMessage id="setting.content.use-webp.description.tips" />
                <Button size="small" onClick={handleConvertToWebp}>
                  <FormattedMessage id="setting.content.use-webp.description.convert" />
                </Button>
              </span>
            </div>
          }
        />
        {useWebp === 1 && (
          <ProFormRadio.Group
            name="convert_gif"
            label={intl.formatMessage({
              id: 'setting.content.convert-gif',
            })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({
                  id: 'setting.content.notenable',
                }),
              },
              {
                value: 1,
                label: intl.formatMessage({ id: 'setting.content.enable' }),
              },
            ]}
            extra={
              <div>
                <span>
                  <FormattedMessage id="setting.content.convert-gif.description" />
                </span>
              </div>
            }
          />
        )}
        <ProFormText
          name="quality"
          label={intl.formatMessage({ id: 'setting.content.quality' })}
          width="lg"
          placeholder={intl.formatMessage({
            id: 'setting.content.quality.placeholder',
          })}
          fieldProps={{
            suffix: '%',
          }}
          extra={intl.formatMessage({
            id: 'setting.content.quality.description',
          })}
        />
        <ProFormRadio.Group
          name="resize_image"
          label={intl.formatMessage({ id: 'setting.content.resize-image' })}
          fieldProps={{
            onChange: (e: any) => {
              setResizeImage(e.target.value);
            },
          }}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.notenable',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({ id: 'setting.content.enable' }),
            },
          ]}
        />
        {resize_image === 1 && (
          <ProFormText
            name="resize_width"
            label={intl.formatMessage({
              id: 'setting.content.resize-width',
            })}
            width="lg"
            placeholder={intl.formatMessage({
              id: 'setting.content.resize-width.placeholder',
            })}
            fieldProps={{
              suffix: intl.formatMessage({
                id: 'setting.content.resize-width.suffix',
              }),
            }}
          />
        )}
        <ProFormRadio.Group
          name="thumb_crop"
          label={intl.formatMessage({ id: 'setting.content.thumb-crop' })}
          options={[
            {
              value: 0,
              label: intl.formatMessage({
                id: 'setting.content.thumb-crop.bylong',
              }),
            },
            {
              value: 1,
              label: intl.formatMessage({
                id: 'setting.content.thumb-crop.byshort',
              }),
            },
            {
              value: 2,
              label: intl.formatMessage({
                id: 'setting.content.thumb-crop.short-crop',
              }),
            },
          ]}
        />
        <ProFormGroup
          title={intl.formatMessage({ id: 'setting.content.thumb-size' })}
        >
          <ProFormText
            name="thumb_width"
            width="sm"
            fieldProps={{
              suffix: intl.formatMessage({
                id: 'setting.content.thumb-size.width',
              }),
            }}
          />
          ×
          <ProFormText
            name="thumb_height"
            width="sm"
            fieldProps={{
              suffix: intl.formatMessage({
                id: 'setting.content.thumb-size.height',
              }),
            }}
          />
        </ProFormGroup>
        <div className="text-muted mb-normal">
          <span>
            <FormattedMessage id="setting.system.thumb-size.tips" />
            <Button size="small" onClick={handleRebuildThumb}>
              <FormattedMessage id="setting.content.make-thumb" />
            </Button>
          </span>
        </div>
        <Divider />
        <ProFormRadio.Group
          name="default_thumb_type"
          label={intl.formatMessage({
            id: 'setting.content.default-thumb',
          })}
          width="lg"
          extra={intl.formatMessage({
            id: 'setting.content.default-thumb.description',
          })}
          options={[
            {
              label: intl.formatMessage({
                id: 'setting.content.default-thumb.thumbs',
              }),
              value: 0,
            },
            {
              label: intl.formatMessage({
                id: 'setting.content.default-thumb.category',
              }),
              value: 3,
            },
          ]}
          fieldProps={{
            onChange: (e) => {
              setDefaultThumbType(e.target.value);
            },
          }}
        />
        {defaultThumbType === 0 ? (
          <ProFormText>
            {setting?.default_thumbs?.map((item: any, index: number) => (
              <div key={index} className="ant-upload-item">
                <img src={item} style={{ width: '100%' }} />
                <a
                  className="delete"
                  onClick={(e) => handleRemoveDefaultThumb(e, index)}
                >
                  <FormattedMessage id="setting.system.delete" />
                </a>
              </div>
            ))}
            <AttachmentSelect
              onSelect={handleSelectDefaultThumb}
              multiple
              open={false}
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
          </ProFormText>
        ) : (
          <ProFormSelect
            label={intl.formatMessage({
              id: 'setting.content.default-thumb.category',
            })}
            name="thumb_category_id"
            width={'lg'}
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
      </div>
    ),
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProForm
        formRef={formRef}
        initialValues={setting}
        onFinish={onSubmit}
        title={intl.formatMessage({ id: 'menu.setting.content' })}
      >
        <Card
          className="mb-normal"
          key={newKey}
          activeTabKey={activeTabKey}
          onTabChange={(tabKey) => setActiveTabKey(tabKey)}
          tabList={[
            {
              key: 'base',
              label: intl.formatMessage({ id: 'setting.tab.content-base' }),
            },
            {
              key: 'rule',
              label: intl.formatMessage({ id: 'setting.tab.list-rules' }),
            },
            {
              key: 'image',
              label: intl.formatMessage({ id: 'setting.tab.image' }),
            },
          ]}
        >
          {tabContnet[activeTabKey] || null}
        </Card>
      </ProForm>
    </NewContainer>
  );
};

export default SettingContentFrom;
