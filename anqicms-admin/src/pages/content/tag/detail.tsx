import NewContainer from '@/components/NewContainer';
import AiGenerate from '@/components/aiGenerate';
import AiGetTdk from '@/components/aitdk';
import AttachmentSelect from '@/components/attachment';
import CollapseItem from '@/components/collaspeItem';
import {
  anqiExtractDescription,
  getArchives,
  getCategories,
  getDesignTemplateFiles,
  getSettingContent,
  getTagFields,
  getTagInfo,
  saveTag,
} from '@/services';
import {
  DeleteOutlined,
  DownOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
  UpOutlined,
} from '@ant-design/icons';
import {
  ProForm,
  ProFormCheckbox,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  Button,
  Card,
  Col,
  Image,
  message,
  Modal,
  Row,
  Space,
  Tag,
} from 'antd';
import React, { lazy, useEffect, useRef, useState } from 'react';
import './index.less';
const MarkdownEditor = lazy(() => import('@/components/markdown'));
const NewAiEditor = lazy(() => import('@/components/newAiEditor'));
const SimpleEditor = lazy(() => import('@/components/simpleEditor'));

const ArchiveTagDetail: React.FC = () => {
  const formRef = useRef<ProFormInstance>();
  const intl = useIntl();
  const [content, setContent] = useState<string>('');
  const [contentSetting, setContentSetting] = useState<any>({});
  const [loaded, setLoaded] = useState<boolean>(false);
  const [tag, setTag] = useState<any>({});
  const [tagLogo, setTagLogo] = useState<string>('');
  const [newKey, setNewKey] = useState<string>('');
  const [tagFields, setTagFields] = useState<any>([]);
  const [extraContent, setExtraContent] = useState<any>({});
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);
  const [categories, setCategories] = useState<any[]>([]);
  const [selectedArchives, setSelectedArchives] = useState<any[]>([]);
  const editorRef = useRef<any>(null);
  const [aiTitle, setAiTitle] = useState<string>('');
  const [aiVisible, setAiVisible] = useState<boolean>(false);
  const [aiTdkVisible, setAiTdkVisible] = useState<boolean>(false);

  const getSelectedArchives = (arcIds: number[]) => {
    if (arcIds.length > 0) {
      // 存在了再处理
      getArchives({
        id: arcIds.join(','),
        limit: 20,
      }).then((res) => {
        if (res.data) {
          setSelectedArchives(res.data);
          setSearchArchives(res.data);
        }
      });
    }
  };

  const initData = async () => {
    const searchParams = new URLSearchParams(window.location.search);
    let id = searchParams.get('id') || 0;
    if (id === 'new') {
      id = 0;
    }
    const res1 = await getTagInfo({ id: id });
    const tag = res1?.data || {};
    if (typeof tag.extra === 'undefined' || tag.extra === null) {
      tag.extra = {};
    }
    setTag(tag);
    setTagLogo(res1?.data?.logo || '');
    setContent(res1?.data?.content || '');
    const resTag = await getTagFields();
    const tagFields = resTag?.data || [];
    setTagFields(tagFields);
    let extraContent: any = {};
    let arcIds = [];
    // eslint-disable-next-line guard-for-in
    for (let i in tagFields) {
      let field = tagFields[i];
      if (field.type === 'editor') {
        extraContent[field.field_name] = tag.extra?.[field.field_name] || '';
      } else if (
        field.type === 'archive' &&
        tag.extra?.[field.field_name]?.length > 0
      ) {
        arcIds.push(...tag.extra[field.field_name]);
      }
    }
    setExtraContent(extraContent);
    getSelectedArchives(arcIds);
    getCategories().then((res) => {
      setCategories(res.data || []);
    });

    const res2 = await getSettingContent();
    setContentSetting(res2.data || {});
    setLoaded(true);
  };

  useEffect(() => {
    initData();
  }, []);

  const onTabChange = (key: string) => {
    setLoaded(false);
    initData().then(() => {
      setNewKey(key);
    });
  };

  const handleSelectLogo = (row: any) => {
    setTagLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanLogo = (e: any) => {
    e.stopPropagation();
    setTagLogo('');
  };

  const onSubmit = async (values: any) => {
    let tagInfo = Object.assign(tag, values);
    tagInfo.content = content;
    tagInfo.logo = tagLogo;
    if (tagInfo.title === '') {
      message.error(intl.formatMessage({ id: 'content.title.required' }));
      return;
    }
    // eslint-disable-next-line guard-for-in
    for (let field in extraContent) {
      if (!tagInfo.extra?.[field]) {
        tagInfo.extra[field] = null;
      }
      tagInfo.extra[field] = extraContent[field];
    }
    const res = await saveTag(tagInfo);
    if (res.code === 0) {
      message.info(res.msg);
      history.back();
    } else {
      message.error(res.msg);
    }
  };

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 's' && (event.ctrlKey || event.metaKey)) {
      const values = formRef.current?.getFieldsFormatValue?.();
      // 自动保存
      onSubmit(values);

      event.preventDefault();
    }
  };

  const handleCleanExtraField = (field: string) => {
    const extra: any = {};
    extra[field] = null;
    formRef.current?.setFieldsValue({ extra });

    delete tag.extra[field];
    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const handleUploadExtraField = (field: string, row: any) => {
    const extra: any = {};
    extra[field] = row.file_path;
    formRef.current?.setFieldsValue({ extra });
    if (!tag.extra[field]) {
      tag.extra[field] = null;
    }
    tag.extra[field] = row.file_path;

    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const handleCleanExtraFieldItem = (field: string, index: number) => {
    tag.extra[field]?.splice(index, 1);
    const extra: any = {};
    extra[field] = tag.extra[field];
    formRef?.current?.setFieldsValue({ extra });

    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const handleUploadExtraFieldItem = (field: string, rows: any) => {
    console.log(tag.extra);
    if (!tag.extra[field]) {
      tag.extra[field] = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in tag.extra[field]) {
        if (tag.extra[field][i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        tag.extra[field].push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = tag.extra[field];
    formRef?.current?.setFieldsValue({ extra });

    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const handleMoveExtraFieldItem = (
    field: string,
    index: number,
    direction: 'up' | 'down',
  ) => {
    if (direction === 'up') {
      if (index <= 0) {
        return;
      }
      const temp = tag.extra[field][index];
      tag.extra[field][index] = tag.extra[field][index - 1];
      tag.extra[field][index - 1] = temp;
    } else {
      if (index >= tag.extra[field].length - 1) {
        return;
      }
      const temp = tag.extra[field][index];
      tag.extra[field][index] = tag.extra[field][index + 1];
      tag.extra[field][index + 1] = temp;
    }
    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const onAddExtraTextsField = (field: string) => {
    if (!tag.extra[field]) {
      tag.extra[field] = [];
    }
    tag.extra[field].push({
      key: '',
      value: '',
    });
    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
    const extra: any = {};
    extra[field] = tag.extra[field];
    formRef?.current?.setFieldsValue({ extra });
  };

  const onChangeExtraTextsField = (
    field: string,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    if (!tag.extra[field][idx]) {
      tag.extra[field][idx] = {};
    }
    tag.extra[field][idx][keyName] = value;
    const extra: any = {};
    extra[field] = { idx: { keyName: value } };
    formRef?.current?.setFieldsValue({ extra });
    setTag((prev: any) => ({
      ...prev,
      extra: {
        ...tag.extra,
      },
    }));
  };

  const onMoveUpExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx > 0) {
      const tmp = tag.extra[field][idx];
      tag.extra[field][idx] = tag.extra[field][idx - 1];
      tag.extra[field][idx - 1] = tmp;
      const extra: any = {};
      extra[field] = tag.extra[field];
      formRef?.current?.setFieldsValue({ extra });
      setTag((prev: any) => ({
        ...prev,
        extra: {
          ...tag.extra,
        },
      }));
    }
  };

  const onMoveDownExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx < tag.extra[field].length - 1) {
      const tmp = tag.extra[field][idx];
      tag.extra[field][idx] = tag.extra[field][idx + 1];
      tag.extra[field][idx + 1] = tmp;
      const extra: any = {};
      extra[field] = tag.extra[field];
      formRef?.current?.setFieldsValue({ extra });
      setTag((prev: any) => ({
        ...prev,
        extra: {
          ...tag.extra,
        },
      }));
    }
  };

  const onRemoveExtraTextsField = (field: string, idx: number) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.module.field.delete.confirm',
      }),
      content: intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: () => {
        if (tag.extra[field].length === 1) {
          tag.extra[field] = [];
        } else {
          tag.extra[field].splice(idx, 1);
        }
        const extra: any = {};
        extra[field] = tag.extra[field];
        formRef?.current?.setFieldsValue({ extra });
        setTag((prev: any) => ({
          ...prev,
          extra: {
            ...tag.extra,
          },
        }));
      },
    });
  };

  const onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (selectedArchives) {
        searchItems.push(...selectedArchives);
      } else {
        searchItems.push({
          id: 0,
          title: intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      setSearchArchives(searchItems.concat(res.data || []));
    });
  };
  const updateExtraContent = async (field: string, html: string) => {
    extraContent[field] = html;
    setExtraContent(extraContent);
  };

  const aiGenerateArticle = () => {
    const values = formRef.current?.getFieldsFormatValue?.() || {};
    setAiTitle(values.title);
    setAiVisible(true);
  };

  const aiTdkGenerate = () => {
    setAiTdkVisible(true);
  };

  const onFinishAiGenerate = async (values: any) => {
    setAiVisible(false);
    formRef.current?.setFieldsValue({ title: values.title });
    //
    let content = values.content.trim();
    setContent(content);
    editorRef.current?.setInnerContent(content);
  };

  const onFinishAiTdk = async (values: any) => {
    setAiTdkVisible(false);
    let data: any = {};
    if (values.title?.length > 0) {
      data.seo_title = values.title;
    }
    if (values.keywords?.length > 0) {
      data.keywords = values.keywords;
    }
    if (values.description?.length > 0) {
      data.description = values.description;
    }
    formRef.current?.setFieldsValue(data);
  };

  const handleExtractDescription = () => {
    if (content.length < 100) {
      message.error(
        intl.formatMessage({
          id: 'content.archive.content.length.error',
        }),
      );
      return false;
    }
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.archive.extract.description',
      }),
      content: intl.formatMessage({
        id: 'content.archive.extract.description.content',
      }),
      onOk: () => {
        anqiExtractDescription({
          text: content,
        })
          .then((res) => {
            if (res.code === 0) {
              formRef?.current?.setFieldsValue({
                description: res.data,
              });
            } else {
              message.info(res.msg);
            }
          })
          .catch((err) => {
            message.error(
              err.msg ||
                intl.formatMessage({
                  id: 'content.submit.failure',
                }),
            );
          });
      },
    });
  };

  return (
    <NewContainer
      onTabChange={(key) => onTabChange(key)}
      title={
        tag.id > 0
          ? intl.formatMessage({ id: 'content.tags.edit' })
          : intl.formatMessage({ id: 'content.tags.add' })
      }
    >
      <Card key={newKey} onKeyDown={handleKeyDown}>
        {loaded && (
          <ProForm
            formRef={formRef}
            initialValues={tag}
            layout="horizontal"
            onFinish={onSubmit}
          >
            <Row gutter={20}>
              <Col sm={18} xs={24}>
                <ProFormText
                  name="title"
                  label={intl.formatMessage({ id: 'content.tags.name' })}
                />
                <ProFormText
                  name="keywords"
                  label={intl.formatMessage({ id: 'content.keywords.name' })}
                  extra={intl.formatMessage({
                    id: 'content.keywords.description',
                  })}
                />
                <ProFormTextArea
                  name="description"
                  label={intl.formatMessage({ id: 'content.description.name' })}
                  extra={
                    <div className="mt-small">
                      <Space size={20}>
                        <span
                          className="link extract-tag"
                          onClick={aiTdkGenerate}
                        >
                          <FormattedMessage id="component.aitdk.btn.generate" />
                        </span>
                        <span
                          className="link extract-tag"
                          onClick={handleExtractDescription}
                        >
                          <FormattedMessage id="content.description.extract" />
                        </span>
                      </Space>
                    </div>
                  }
                />
                {tagFields && (
                  <CollapseItem
                    header={intl.formatMessage({
                      id: 'content.param.extra-fields',
                    })}
                    open
                    showArrow
                    key="2"
                  >
                    <Row gutter={20}>
                      {tagFields.map(
                        (item: any, index: number) =>
                          item.type !== 'editor' && (
                            <Col
                              sm={
                                item.type === 'timeline' ||
                                item.type === 'images' ||
                                item.type === 'texts'
                                  ? 24
                                  : 12
                              }
                              xs={24}
                              key={index}
                            >
                              {item.type === 'text' ? (
                                <ProFormText
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  required={item.required ? true : false}
                                  placeholder={
                                    item.content &&
                                    intl.formatMessage({
                                      id: 'content.param.default',
                                    }) + item.content
                                  }
                                />
                              ) : item.type === 'number' ? (
                                <ProFormDigit
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  required={item.required ? true : false}
                                  placeholder={
                                    item.content &&
                                    intl.formatMessage({
                                      id: 'content.param.default',
                                    }) + item.content
                                  }
                                />
                              ) : item.type === 'textarea' ? (
                                <ProFormTextArea
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  required={item.required ? true : false}
                                  placeholder={
                                    item.content &&
                                    intl.formatMessage({
                                      id: 'content.param.default',
                                    }) + item.content
                                  }
                                />
                              ) : item.type === 'editor' ? (
                                ''
                              ) : item.type === 'radio' ? (
                                <ProFormRadio.Group
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  request={async () => {
                                    const tmpData = item.content.split('\n');
                                    const data = [];
                                    for (const item1 of tmpData) {
                                      data.push({
                                        label: item1,
                                        value: item1,
                                      });
                                    }
                                    return data;
                                  }}
                                />
                              ) : item.type === 'checkbox' ? (
                                <ProFormCheckbox.Group
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  request={async () => {
                                    const tmpData = item.content.split('\n');
                                    const data = [];
                                    for (const item1 of tmpData) {
                                      data.push({
                                        label: item1,
                                        value: item1,
                                      });
                                    }
                                    return data;
                                  }}
                                />
                              ) : item.type === 'select' ? (
                                <ProFormSelect
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  request={async () => {
                                    const tmpData = item.content.split('\n');
                                    const data = [];
                                    for (const item1 of tmpData) {
                                      data.push({
                                        label: item1,
                                        value: item1,
                                      });
                                    }
                                    return data;
                                  }}
                                />
                              ) : item.type === 'image' ? (
                                <ProFormText
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                >
                                  {tag.extra?.[item.field_name] ? (
                                    <div className="ant-upload-item">
                                      <Image
                                        preview={{
                                          src: tag.extra[item.field_name],
                                        }}
                                        src={tag.extra[item.field_name]}
                                      />
                                      <span
                                        className="delete"
                                        onClick={() =>
                                          handleCleanExtraField(item.field_name)
                                        }
                                      >
                                        <DeleteOutlined />
                                      </span>
                                    </div>
                                  ) : (
                                    <AttachmentSelect
                                      onSelect={(row) =>
                                        handleUploadExtraField(
                                          item.field_name,
                                          row,
                                        )
                                      }
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
                                  )}
                                </ProFormText>
                              ) : item.type === 'images' ? (
                                <ProFormText
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                >
                                  {tag.extra?.[item.field_name]?.length
                                    ? tag.extra[item.field_name]?.map(
                                        (inner: string, idx: number) => (
                                          <div
                                            className="ant-upload-item"
                                            key={idx}
                                          >
                                            <Image
                                              preview={{
                                                src: inner,
                                              }}
                                              src={inner}
                                            />
                                            <div className="ant-upload-item-action">
                                              <Tag
                                                onClick={() =>
                                                  handleMoveExtraFieldItem(
                                                    item.field_name,
                                                    idx,
                                                    'up',
                                                  )
                                                }
                                              >
                                                <LeftOutlined />
                                              </Tag>
                                              <Tag
                                                color="red"
                                                onClick={() =>
                                                  handleCleanExtraFieldItem(
                                                    item.field_name,
                                                    idx,
                                                  )
                                                }
                                              >
                                                <DeleteOutlined />
                                              </Tag>
                                              <Tag
                                                onClick={() =>
                                                  handleMoveExtraFieldItem(
                                                    item.field_name,
                                                    idx,
                                                    'down',
                                                  )
                                                }
                                              >
                                                <RightOutlined />
                                              </Tag>
                                            </div>
                                          </div>
                                        ),
                                      )
                                    : null}
                                  <AttachmentSelect
                                    onSelect={(rows) =>
                                      handleUploadExtraFieldItem(
                                        item.field_name,
                                        rows,
                                      )
                                    }
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
                                </ProFormText>
                              ) : item.type === 'file' ? (
                                <ProFormText
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                >
                                  {tag.extra?.[item.field_name] ? (
                                    <div className="ant-upload-item ant-upload-file">
                                      <span>{tag.extra[item.field_name]}</span>
                                      <span
                                        className="delete"
                                        onClick={() =>
                                          handleCleanExtraField(item.field_name)
                                        }
                                      >
                                        <DeleteOutlined />
                                      </span>
                                    </div>
                                  ) : (
                                    <AttachmentSelect
                                      onSelect={(row) =>
                                        handleUploadExtraField(
                                          item.field_name,
                                          row,
                                        )
                                      }
                                      open={false}
                                    >
                                      <Button>
                                        <FormattedMessage id="setting.system.upload" />
                                      </Button>
                                    </AttachmentSelect>
                                  )}
                                </ProFormText>
                              ) : item.type === 'texts' ? (
                                <ProFormText label={item.name}>
                                  <div className="text-groups">
                                    <div className="text-group">
                                      <div className="text-key">Key</div>
                                      <div className="text-value">Value</div>
                                      <div className="text-action"></div>
                                    </div>
                                    {tag.extra?.[item.field_name]?.length
                                      ? tag.extra[item.field_name].map(
                                          (inner: any, idx: number) => (
                                            <div
                                              className="text-group"
                                              key={idx}
                                            >
                                              <div className="text-key">
                                                <ProFormText
                                                  name={[
                                                    'extra',
                                                    item.field_name,
                                                    idx,
                                                    'key',
                                                  ]}
                                                  fieldProps={{
                                                    onChange: (e: any) => {
                                                      onChangeExtraTextsField(
                                                        item.field_name,
                                                        idx,
                                                        'key',
                                                        e.target.value,
                                                      );
                                                    },
                                                  }}
                                                />
                                              </div>
                                              <div className="text-value">
                                                <ProFormText
                                                  name={[
                                                    'extra',
                                                    item.field_name,
                                                    idx,
                                                    'value',
                                                  ]}
                                                  fieldProps={{
                                                    onChange: (e: any) => {
                                                      onChangeExtraTextsField(
                                                        item.field_name,
                                                        idx,
                                                        'value',
                                                        e.target.value,
                                                      );
                                                    },
                                                  }}
                                                />
                                              </div>
                                              <div className="text-action">
                                                <Tag
                                                  onClick={() =>
                                                    onMoveUpExtraTextsField(
                                                      item.field_name,
                                                      idx,
                                                    )
                                                  }
                                                >
                                                  <UpOutlined />
                                                </Tag>
                                                <Tag
                                                  onClick={() =>
                                                    onMoveDownExtraTextsField(
                                                      item.field_name,
                                                      idx,
                                                    )
                                                  }
                                                >
                                                  <DownOutlined />
                                                </Tag>
                                                <Tag
                                                  color="red"
                                                  onClick={() =>
                                                    onRemoveExtraTextsField(
                                                      item.field_name,
                                                      idx,
                                                    )
                                                  }
                                                >
                                                  <DeleteOutlined />
                                                </Tag>
                                              </div>
                                            </div>
                                          ),
                                        )
                                      : null}
                                    <div className="text-group">
                                      <div className="text-key">
                                        <Tag
                                          color="blue"
                                          className="add-line"
                                          onClick={() =>
                                            onAddExtraTextsField(
                                              item.field_name,
                                            )
                                          }
                                        >
                                          {intl.formatMessage({
                                            id: 'content.param.add-line',
                                          })}
                                        </Tag>
                                      </div>
                                    </div>
                                  </div>
                                </ProFormText>
                              ) : item.type === 'archive' ? (
                                <ProFormText label={item.name}>
                                  <ProFormSelect
                                    name={['extra', item.field_name]}
                                    showSearch
                                    mode="multiple"
                                    options={searchArchives.map((a: any) => ({
                                      title: a.title,
                                      label: a.title,
                                      value: a.id,
                                    }))}
                                    fieldProps={{
                                      onSearch: (e) => {
                                        onSearchArchives(e);
                                      },
                                    }}
                                  />
                                </ProFormText>
                              ) : item.type === 'category' ? (
                                <ProFormText label={item.name}>
                                  <ProFormSelect
                                    showSearch
                                    name={['extra', item.field_name]}
                                    mode={'single'}
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
                                                  ?.map(
                                                    (parent: any) =>
                                                      parent.title,
                                                  )
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
                                      filterOption: (
                                        input: string,
                                        option: any,
                                      ) =>
                                        (option?.title ?? option?.label)
                                          .toLowerCase()
                                          .includes(input.toLowerCase()),
                                    }}
                                  />
                                </ProFormText>
                              ) : null}
                            </Col>
                          ),
                      )}
                    </Row>
                    {tagFields.map(
                      (item: any, index: number) =>
                        item.type === 'editor' && (
                          <ProFormText
                            key={index}
                            label={item.name}
                            required={item.required ? true : false}
                            extra={
                              item.content &&
                              intl.formatMessage({
                                id: 'content.param.default',
                              }) + item.content
                            }
                          >
                            {contentSetting.editor === 'markdown' ? (
                              <MarkdownEditor
                                className="mb-normal"
                                setContent={(html) =>
                                  updateExtraContent(item.field_name, html)
                                }
                                content={extraContent[item.field_name] || ''}
                                ref={null}
                              />
                            ) : contentSetting.editor === 'simple' ? (
                              <SimpleEditor
                                className="mb-normal"
                                setContent={(html) =>
                                  updateExtraContent(item.field_name, html)
                                }
                                content={extraContent[item.field_name] || ''}
                                ref={null}
                              />
                            ) : (
                              <NewAiEditor
                                className="mb-normal"
                                setContent={(html) =>
                                  updateExtraContent(item.field_name, html)
                                }
                                content={extraContent[item.field_name] || ''}
                                key={item.field_name}
                                field={item.field_name}
                                ref={null}
                              />
                            )}
                          </ProFormText>
                        ),
                    )}
                  </CollapseItem>
                )}
                {contentSetting.editor === 'markdown' ? (
                  <MarkdownEditor
                    className="mb-normal"
                    setContent={async (html: string) => {
                      setContent(html);
                    }}
                    content={content}
                    ref={editorRef}
                  />
                ) : contentSetting.editor === 'simple' ? (
                  <SimpleEditor
                    className="mb-normal"
                    setContent={async (html: string) => {
                      setContent(html);
                    }}
                    content={content}
                    ref={editorRef}
                  />
                ) : (
                  <NewAiEditor
                    className="mb-normal"
                    setContent={async (html: string) => {
                      setContent(html);
                    }}
                    key="content"
                    field="content"
                    ref={editorRef}
                    content={content}
                  />
                )}
              </Col>
              <Col sm={6} xs={24}>
                <div className="mb-normal">
                  <Row gutter={[16, 16]}>
                    <Col span={12}>
                      <Button
                        block
                        type="primary"
                        onClick={() => {
                          onSubmit(formRef.current?.getFieldsFormatValue?.());
                        }}
                      >
                        <FormattedMessage id="content.submit.ok" />
                      </Button>
                    </Col>
                    <Col span={12}>
                      <Button
                        block
                        onClick={() => {
                          history.back();
                        }}
                      >
                        <FormattedMessage id="design.back" />
                      </Button>
                    </Col>
                    <Col span={12}>
                      <Button
                        block
                        onClick={() => {
                          aiGenerateArticle();
                        }}
                      >
                        <FormattedMessage id="content.submit.aigenerate" />
                      </Button>
                    </Col>
                    <Col span={12}>
                      <Button
                        block
                        onClick={() => {
                          aiTdkGenerate();
                        }}
                      >
                        <FormattedMessage id="component.aitdk.btn.generate" />
                      </Button>
                    </Col>
                  </Row>
                </div>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.tag.template' })}
                >
                  <ProFormSelect
                    showSearch
                    name="category_id"
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
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.category.thumb' })}
                >
                  {tagLogo ? (
                    <div className="ant-upload-item">
                      <Image
                        preview={{
                          src: tagLogo,
                        }}
                        src={tagLogo}
                      />
                      <span className="delete" onClick={handleCleanLogo}>
                        <DeleteOutlined />
                      </span>
                    </div>
                  ) : (
                    <AttachmentSelect onSelect={handleSelectLogo} open={false}>
                      <div className="ant-upload-item">
                        <div className="add">
                          <PlusOutlined />
                          <div style={{ marginTop: 8 }}>
                            <FormattedMessage id="setting.system.upload" />
                          </div>
                        </div>
                      </div>
                    </AttachmentSelect>
                  )}
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.seo-title.name' })}
                >
                  <ProFormText
                    name="seo_title"
                    placeholder={intl.formatMessage({
                      id: 'content.tags.seo-title.placeholder',
                    })}
                    extra={intl.formatMessage({
                      id: 'content.tags.seo-title.description',
                    })}
                  />
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({
                    id: 'content.tags.first-letter.name',
                  })}
                >
                  <ProFormText
                    name="first_letter"
                    placeholder={intl.formatMessage({
                      id: 'content.url-token.placeholder',
                    })}
                    extra={intl.formatMessage({
                      id: 'content.tags.first-letter.description',
                    })}
                  />
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.url-token.name' })}
                >
                  <ProFormText
                    name="url_token"
                    placeholder={intl.formatMessage({
                      id: 'content.url-token.placeholder',
                    })}
                    extra={intl.formatMessage({ id: 'content.url-token.tips' })}
                  />
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({
                    id: 'content.category.template',
                  })}
                >
                  <ProFormSelect
                    showSearch
                    name="template"
                    request={async () => {
                      const res = await getDesignTemplateFiles({});
                      const data = [
                        {
                          path: '',
                          remark: intl.formatMessage({
                            id: 'content.default-template',
                          }),
                        },
                      ].concat(res.data || []);
                      for (const i in data) {
                        if (!data[i].remark) {
                          data[i].remark = data[i].path;
                        } else {
                          data[i].remark =
                            data[i].path + '(' + data[i].remark + ')';
                        }
                      }
                      return data;
                    }}
                    fieldProps={{
                      fieldNames: {
                        label: 'remark',
                        value: 'path',
                      },
                    }}
                    extra={
                      <div>
                        <FormattedMessage id="content.category.default" />:{' '}
                        tag/list.html
                      </div>
                    }
                  />
                </Card>
              </Col>
            </Row>
          </ProForm>
        )}
        {aiVisible && (
          <AiGenerate
            open={aiVisible}
            title={aiTitle}
            editor={contentSetting.editor}
            onCancel={() => setAiVisible(false)}
            onSubmit={onFinishAiGenerate}
          />
        )}
        {aiTdkVisible && (
          <AiGetTdk
            open={aiTdkVisible}
            content={content}
            onCancel={() => setAiTdkVisible(false)}
            onSubmit={onFinishAiTdk}
          />
        )}
      </Card>
    </NewContainer>
  );
};

export default ArchiveTagDetail;
