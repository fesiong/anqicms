import NewContainer from '@/components/NewContainer';
import AttachmentSelect from '@/components/attachment';
import CollapseItem from '@/components/collaspeItem';
import MarkdownEditor from '@/components/markdown';
import NewAiEditor from '@/components/newAiEditor';
import {
  getArchives,
  getCategories,
  getCategoryInfo,
  getDesignTemplateFiles,
  getModules,
  getSettingContent,
  saveCategory,
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
import { Button, Card, Col, Image, Modal, Row, Tag, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import '../index.less';

const categoryType = 1;
const moduleId = 2;

const ArchiveCategoryDetail: React.FC = () => {
  const formRef = useRef<ProFormInstance>();
  const editorRef = useRef(null);
  const [content, setContent] = useState<string>('');
  const [categoryImages, setCategoryImages] = useState<string[]>([]);
  const [categoryLogo, setCategoryLogo] = useState<string>('');
  const [currentModule, setCurrentModule] = useState<any>({
    category_fields: [],
  });
  const intl = useIntl();
  const [extraContent, setExtraContent] = useState<any>({});
  const [contentSetting, setContentSetting] = useState<any>({});
  const [loaded, setLoaded] = useState<boolean>(false);
  const [category, setCategory] = useState<any>({ status: 1 });
  const [modules, setModules] = useState<any[]>([]);
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);
  const [selectedArchives, setSelectedArchives] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');

  const changeModule = (e: any, tmpModels?: any) => {
    let newModules = tmpModels || modules;
    for (let item of newModules) {
      if (item.id === e) {
        setCurrentModule(item);
        return item;
      }
    }
  };

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
    let parent_id = Number(searchParams.get('parent_id') || 0);

    let modRes = await getModules();
    setModules(modRes.data || []);
    let catRes = await getCategoryInfo({
      id: id,
    });
    let cat = catRes.data || { status: 1, parent_id: parent_id, extra: {} };
    if (typeof cat.extra === 'undefined' || cat.extra === null) {
      cat.extra = {};
    }
    let moduleId = cat.module_id || 2;
    if (parent_id > 0) {
      let parentRes = await getCategoryInfo({
        id: parent_id,
      });
      if (parentRes.data) {
        moduleId = parentRes.data.module_id;
      }
      cat.module_id = moduleId;
    }
    let module = changeModule(moduleId, modRes.data);
    let extraContent: any = {};
    let arcIds = [];
    // eslint-disable-next-line guard-for-in
    for (let i in module.category_fields) {
      let field = module.category_fields[i];
      if (field.type === 'editor') {
        extraContent[field.field_name] = cat.extra?.[field.field_name] || '';
      } else if (
        field.type === 'archive' &&
        cat.extra?.[field.field_name]?.length > 0
      ) {
        arcIds.push(...cat.extra[field.field_name]);
      }
    }
    setExtraContent(extraContent);
    setCategory(cat);
    getSelectedArchives(arcIds);

    setContent(cat.content || '');
    setCategoryImages(cat.images || []);
    setCategoryLogo(cat.logo || '');

    getSettingContent().then((res) => {
      setContentSetting(res.data || {});
      setLoaded(true);
    });
  };

  const onTabChange = (key: string) => {
    setLoaded(false);
    initData().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    initData();
  }, []);

  const onSubmit = async (values: any) => {
    let cat = Object.assign(category, values);
    cat.content = content;
    cat.type = categoryType;
    cat.images = categoryImages;
    cat.logo = categoryLogo;
    cat.module_id = moduleId;
    if (cat.title === '') {
      message.error(
        intl.formatMessage({ id: 'content.category.input.required' }),
      );
      return;
    }
    // eslint-disable-next-line guard-for-in
    for (let field in extraContent) {
      if (!cat.extra?.[field]) {
        cat.extra[field] = null;
      }
      cat.extra[field] = extraContent[field];
    }
    let res = await saveCategory(cat);
    if (res.code === 0) {
      message.info(res.msg);
      history.back();
    } else if (res.msg === 'token duplication') {
      Modal.confirm({
        content:
          intl.formatMessage({ id: 'content.url-token.name' }) +
          '"' +
          cat.url_token +
          '"' +
          intl.formatMessage({ id: 'content.url-token.dublicate.tips' }),
        okText: intl.formatMessage({ id: 'content.submit.ok-force' }),
        okType: 'danger',
        cancelText: intl.formatMessage({ id: 'content.submit.cancel' }),
        onOk: () => {
          cat.force = true;
          onSubmit(cat);
        },
      });
    } else {
      message.error(res.msg);
    }
  };

  const handleSelectImages = (rows: any) => {
    for (const row of rows) {
      let exists = false;
      for (let i in categoryImages) {
        if (categoryImages[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        categoryImages.push(row.file_path);
      }
    }
    setCategoryImages([].concat(categoryImages));
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    categoryImages.splice(index, 1);
    setCategoryImages([].concat(categoryImages));
  };

  const handleSelectLogo = (row: any) => {
    setCategoryLogo(row.file_path);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleCleanLogo = (e: any) => {
    e.stopPropagation();
    setCategoryLogo('');
  };

  const updateExtraContent = async (field: string, html: string) => {
    extraContent[field] = html;
    setExtraContent(extraContent);
  };

  const handleCleanExtraField = (field: string) => {
    const extra: any = {};
    extra[field] = null;
    formRef.current?.setFieldsValue({ extra });

    delete category.extra[field];
    setCategory(category);
  };

  const handleUploadExtraField = (field: string, row: any) => {
    const extra: any = {};
    extra[field] = row.file_path;
    formRef.current?.setFieldsValue({ extra });
    if (!category.extra[field]) {
      category.extra[field] = null;
    }
    category.extra[field] = row.file_path;

    setCategory(category);
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
      const temp = category.extra[field][index];
      category.extra[field][index] = category.extra[field][index - 1];
      category.extra[field][index - 1] = temp;
    } else {
      if (index >= category.extra[field].length - 1) {
        return;
      }
      const temp = category.extra[field][index];
      category.extra[field][index] = category.extra[field][index + 1];
      category.extra[field][index + 1] = temp;
    }
    setCategory(Object.assign({}, category));
  };

  const handleCleanExtraFieldItem = (field: string, index: number) => {
    category.extra[field]?.splice(index, 1);
    const extra: any = {};
    extra[field] = category.extra[field];
    formRef?.current?.setFieldsValue({ extra });

    setCategory(Object.assign({}, category));
  };

  const handleUploadExtraFieldItem = (field: string, rows: any) => {
    if (!category.extra[field]) {
      category.extra[field] = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in category.extra[field]) {
        if (category.extra[field][i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        category.extra[field].push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = category.extra[field];
    formRef?.current?.setFieldsValue({ extra });

    setCategory(Object.assign({}, category));
  };

  const onAddExtraTextsField = (field: string) => {
    if (!category.extra[field]) {
      category.extra[field] = [];
    }
    category.extra[field].push({
      key: '',
      value: '',
    });
    const extra: any = {};
    extra[field] = category.extra[field];
    formRef?.current?.setFieldsValue({ extra });
    setCategory(Object.assign({}, category));
  };

  const onChangeExtraTextsField = (
    field: string,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    if (!category.extra[field][idx]) {
      category.extra[field][idx] = {};
    }
    category.extra[field][idx][keyName] = value;
    const extra: any = {};
    extra[field] = { idx: { keyName: value } };
    formRef?.current?.setFieldsValue({ extra });
    setCategory(category);
  };

  const onMoveUpExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx > 0) {
      const tmp = category.extra[field][idx];
      category.extra[field][idx] = category.extra[field][idx - 1];
      category.extra[field][idx - 1] = tmp;
      const extra: any = {};
      extra[field] = category.extra[field];
      formRef?.current?.setFieldsValue({ extra });
      setCategory(Object.assign({}, category));
    }
  };

  const onMoveDownExtraTextsField = (field: string, idx: number) => {
    // 移动
    if (idx < category.extra[field].length - 1) {
      const tmp = category.extra[field][idx];
      category.extra[field][idx] = category.extra[field][idx + 1];
      category.extra[field][idx + 1] = tmp;
      const extra: any = {};
      extra[field] = category.extra[field];
      formRef?.current?.setFieldsValue({ extra });
      setCategory(Object.assign({}, category));
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
        if (category.extra[field].length === 1) {
          category.extra[field] = [];
        } else {
          category.extra[field].splice(idx, 1);
        }
        setCategory(Object.assign({}, category));
        const extra: any = {};
        extra[field] = category.extra[field];
        formRef?.current?.setFieldsValue({ extra });
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
  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 's' && (event.ctrlKey || event.metaKey)) {
      const values = formRef.current?.getFieldsFormatValue?.();
      // 自动保存
      onSubmit(values);

      event.preventDefault();
    }
  };

  return (
    <NewContainer
      onTabChange={(key) => onTabChange(key)}
      title={
        category.id > 0
          ? intl.formatMessage({ id: 'content.category.edit' })
          : intl.formatMessage({ id: 'content.category.new' })
      }
    >
      <Card key={newKey} onKeyDown={handleKeyDown}>
        {loaded && (
          <ProForm
            formRef={formRef}
            initialValues={category}
            layout="horizontal"
            onFinish={onSubmit}
          >
            <Row gutter={20}>
              <Col sm={18} xs={24}>
                <ProFormSelect
                  label={intl.formatMessage({ id: 'content.category.parent' })}
                  name="parent_id"
                  width="lg"
                  request={async () => {
                    let res = await getCategories({
                      type: categoryType,
                      module_id: moduleId,
                    });
                    let categories = res.data || [];
                    // 排除自己
                    if (category.id) {
                      let tmpCategory = [];
                      for (let i in categories) {
                        if (
                          categories[i].id === category.id ||
                          categories[i].parent_id === category.id ||
                          categories[i].module_id !== category.module_id
                        ) {
                          continue;
                        }
                        tmpCategory.push(categories[i]);
                      }
                      categories = tmpCategory;
                    }
                    categories = [
                      {
                        id: 0,
                        title: intl.formatMessage({
                          id: 'content.category.top',
                        }),
                        spacer: '',
                      },
                    ].concat(categories);
                    return categories;
                  }}
                  readonly={
                    category.id || category.module_id > 0 ? false : true
                  }
                  fieldProps={{
                    fieldNames: {
                      label: 'title',
                      value: 'id',
                    },
                    optionItemRender(item: any) {
                      return (
                        <div
                          dangerouslySetInnerHTML={{
                            __html: (item.spacer || '') + item.title,
                          }}
                        ></div>
                      );
                    },
                  }}
                />
                <ProFormText
                  name="title"
                  label={intl.formatMessage({ id: 'content.category.title' })}
                />
                <ProFormTextArea
                  name="description"
                  label={intl.formatMessage({
                    id: 'content.category.description',
                  })}
                />
                <ProFormText
                  name="seo_title"
                  label={intl.formatMessage({ id: 'content.seo-title.name' })}
                  placeholder={intl.formatMessage({
                    id: 'content.category.seo-title.placeholder',
                  })}
                  extra={intl.formatMessage({
                    id: 'content.category.seo-title.description',
                  })}
                />
                <ProFormText
                  name="keywords"
                  label={intl.formatMessage({ id: 'content.keywords.name' })}
                  extra={intl.formatMessage({
                    id: 'content.keywords.description',
                  })}
                />
                <ProFormRadio.Group
                  name="status"
                  label={intl.formatMessage({ id: 'content.category.status' })}
                  options={[
                    {
                      value: 0,
                      label: intl.formatMessage({
                        id: 'content.category.status.hide',
                      }),
                    },
                    {
                      value: 1,
                      label: intl.formatMessage({
                        id: 'content.category.status.ok',
                      }),
                    },
                  ]}
                  extra={intl.formatMessage({
                    id: 'content.category.status.description',
                  })}
                />
                {currentModule.category_fields && (
                  <CollapseItem
                    header={intl.formatMessage({
                      id: 'content.param.extra-fields',
                    })}
                    open
                    showArrow
                    key="2"
                  >
                    <Row gutter={20}>
                      {currentModule.category_fields?.map(
                        (item: any, index: number) =>
                          item.type !== 'editor' && (
                            <Col sm={12} xs={24} key={index}>
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
                              ) : item.type === 'radio' ? (
                                <ProFormRadio.Group
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                  request={async () => {
                                    const tmpData = item.content.split('\n');
                                    const data = [];
                                    for (const item1 of tmpData) {
                                      data.push({ label: item1, value: item1 });
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
                                      data.push({ label: item1, value: item1 });
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
                                      data.push({ label: item1, value: item1 });
                                    }
                                    return data;
                                  }}
                                />
                              ) : item.type === 'image' ? (
                                <ProFormText
                                  name={['extra', item.field_name]}
                                  label={item.name}
                                >
                                  {category.extra?.[item.field_name] ? (
                                    <div className="ant-upload-item">
                                      <Image
                                        preview={{
                                          src: category.extra[item.field_name],
                                        }}
                                        src={category.extra[item.field_name]}
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
                                  {category.extra?.[item.field_name]?.length
                                    ? category.extra[item.field_name].map(
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
                                  {category.extra?.[item.field_name] ? (
                                    <div className="ant-upload-item ant-upload-file">
                                      <span>
                                        {category.extra[item.field_name]}
                                      </span>
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
                                    {category.extra?.[item.field_name]?.length
                                      ? category.extra[item.field_name].map(
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
                                    request={async () => {
                                      const res = await getCategories({
                                        type: 1,
                                      });
                                      const categories = (res.data || []).map(
                                        (cat: any) => ({
                                          spacer: cat.spacer,
                                          label:
                                            cat.title +
                                            (cat.status === 1
                                              ? ''
                                              : intl.formatMessage({
                                                  id: 'setting.nav.hide',
                                                })),
                                          value: cat.id,
                                        }),
                                      );
                                      return categories;
                                    }}
                                    fieldProps={{
                                      optionItemRender(item: any) {
                                        return (
                                          <div
                                            dangerouslySetInnerHTML={{
                                              __html: item.spacer + item.label,
                                            }}
                                          ></div>
                                        );
                                      },
                                    }}
                                  />
                                </ProFormText>
                              ) : null}
                            </Col>
                          ),
                      )}
                    </Row>
                    {currentModule.category_fields?.map(
                      (item: any, index: number) =>
                        item.type === 'editor' ? (
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
                        ) : (
                          ''
                        ),
                    )}
                  </CollapseItem>
                )}
                {loaded && (
                  <>
                    {contentSetting.editor === 'markdown' ? (
                      <MarkdownEditor
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
                  </>
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
                  </Row>
                </div>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.category.banner' })}
                >
                  {categoryImages.length
                    ? categoryImages.map((item: string, index: number) => (
                        <div className="ant-upload-item" key={index}>
                          <Image
                            preview={{
                              src: item,
                            }}
                            src={item}
                          />
                          <span
                            className="delete"
                            onClick={handleCleanImages.bind(this, index)}
                          >
                            <DeleteOutlined />
                          </span>
                        </div>
                      ))
                    : null}
                  <AttachmentSelect
                    onSelect={handleSelectImages}
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
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.category.thumb' })}
                >
                  {categoryLogo ? (
                    <div className="ant-upload-item">
                      <Image
                        preview={{
                          src: categoryLogo,
                        }}
                        src={categoryLogo}
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
                  title={intl.formatMessage({ id: 'content.category.sort' })}
                >
                  <ProFormDigit
                    name="sort"
                    extra={intl.formatMessage({
                      id: 'content.category.sort.description',
                    })}
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
                        {currentModule.table_name}/list.html
                      </div>
                    }
                  />
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({ id: 'content.category.inherit' })}
                >
                  <ProFormRadio.Group
                    name="is_inherit"
                    options={[
                      {
                        value: 0,
                        label: intl.formatMessage({
                          id: 'content.category.inherit.false',
                        }),
                      },
                      {
                        value: 1,
                        label: intl.formatMessage({
                          id: 'content.category.inherit.true',
                        }),
                      },
                    ]}
                    extra={intl.formatMessage({
                      id: 'content.category.inherit.description',
                    })}
                  />
                </Card>
                <Card
                  className="aside-card"
                  size="small"
                  title={intl.formatMessage({
                    id: 'content.archive-template.name',
                  })}
                >
                  <ProFormSelect
                    showSearch
                    name="detail_template"
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
                        {currentModule.table_name}/detail.html
                      </div>
                    }
                  />
                </Card>
              </Col>
            </Row>
          </ProForm>
        )}
      </Card>
    </NewContainer>
  );
};

export default ArchiveCategoryDetail;
