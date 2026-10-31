import NewContainer from '@/components/NewContainer';
import AiGenerate from '@/components/aiGenerate';
import AiGetTdk from '@/components/aitdk';
import AttachmentSelect from '@/components/attachment';
import CollapseItem from '@/components/collaspeItem';
import {
  anqiExtractDescription,
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
  QuestionCircleOutlined,
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
  Popover,
  Row,
  Space,
  Tag,
} from 'antd';
import React, { lazy, Suspense, useEffect, useRef, useState } from 'react';
import { getRuntimeShopMode } from '@/utils/shopMode';
import '../index.less';
const MarkdownEditor = lazy(() => import('@/components/markdown'));
const NewAiEditor = lazy(() => import('@/components/newAiEditor'));
const SimpleEditor = lazy(() => import('@/components/simpleEditor'));

const categoryType = 1;

const ArchiveCategoryDetail: React.FC = () => {
  const formRef = useRef<ProFormInstance>();
  const editorRef = useRef<any>(null);
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
  const [categories, setCategories] = useState<any[]>([]);
  const [searchArchives, setSearchArchives] = useState<any[]>([
    {
      id: 0,
      title: intl.formatMessage({
        id: 'content.parent_id.empty',
      }),
    },
  ]);
  const [selectedArchives, setSelectedArchives] = useState<any[]>([]);
  const [aiTitle, setAiTitle] = useState<string>('');
  const [aiVisible, setAiVisible] = useState<boolean>(false);
  const [aiTdkVisible, setAiTdkVisible] = useState<boolean>(false);
  const [dragImageIndex, setDragImageIndex] = useState<number>(-1);
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
    // 商城模式产品独立管理，分类页排除产品模块；普通模式保留
    const modList = modRes.data || [];
    setModules(
      getRuntimeShopMode()
        ? modList.filter((item: any) => item.id !== 2)
        : modList,
    );
    let catRes = await getCategoryInfo({
      id: id,
    });
    let cat = catRes.data || { status: 1, parent_id: parent_id, extra: {} };
    if (typeof cat.extra === 'undefined' || cat.extra === null) {
      cat.extra = {};
    }
    let moduleId = cat.module_id || 1;
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
    getCategories({}).then((res) => {
      const list = res.data || [];
      // 仅商城模式排除产品分类
      setCategories(
        getRuntimeShopMode()
          ? list.filter((item: any) => item.module_id !== 2)
          : list,
      );
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
    const images = [...categoryImages];
    for (const row of rows) {
      let exists = false;
      for (let i in images) {
        if (images[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        images.push(row.file_path);
      }
    }
    setCategoryImages(images);
    message.success(
      intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  const handleDragStart = (index: number) => {
    setDragImageIndex(index);
  };

  const handleDragOver = (e: any, index: number) => {
    e.preventDefault();
    if (dragImageIndex === -1 || dragImageIndex === index) {
      return;
    }
    const images = [...categoryImages];
    const [moved] = images.splice(dragImageIndex, 1);
    images.splice(index, 0, moved);
    setDragImageIndex(index);
    setCategoryImages(images);
  };

  const handleDragEnd = () => {
    setDragImageIndex(-1);
  };

  const handleDrop = (e: any) => {
    e.preventDefault();
    handleDragEnd();
  };

  const handleMoveImages = (index: number, direction: string, e: any) => {
    e.stopPropagation();
    const images = [...categoryImages];
    if (direction === 'up') {
      if (index <= 0) {
        return;
      }
      const temp = images[index];
      images[index] = images[index - 1];
      images[index - 1] = temp;
    } else {
      if (index >= images.length - 1) {
        return;
      }
      const temp = images[index];
      images[index] = images[index + 1];
      images[index + 1] = temp;
    }
    setCategoryImages(images);
  };

  const handleCleanImages = (index: number, e: any) => {
    e.stopPropagation();
    const images = [...categoryImages];
    images.splice(index, 1);
    setCategoryImages(images);
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
    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
  };

  const handleUploadExtraField = (field: string, row: any) => {
    const extra: any = {};
    extra[field] = row.file_path;
    formRef.current?.setFieldsValue({ extra });
    if (!category.extra[field]) {
      category.extra[field] = null;
    }
    category.extra[field] = row.file_path;

    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
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
    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
  };

  const handleCleanExtraFieldItem = (field: string, index: number) => {
    category.extra[field]?.splice(index, 1);
    const extra: any = {};
    extra[field] = category.extra[field];
    formRef?.current?.setFieldsValue({ extra });

    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
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

    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
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
    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
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
    setCategory((prev: any) => ({
      ...prev,
      extra: {
        ...category.extra,
      },
    }));
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
      setCategory((prev: any) => ({
        ...prev,
        extra: {
          ...category.extra,
        },
      }));
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
      setCategory((prev: any) => ({
        ...prev,
        extra: {
          ...category.extra,
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
        if (category.extra[field].length === 1) {
          category.extra[field] = [];
        } else {
          category.extra[field].splice(idx, 1);
        }
        setCategory((prev: any) => ({
          ...prev,
          extra: {
            ...category.extra,
          },
        }));
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
                  name="module_id"
                  label={intl.formatMessage({ id: 'content.module.name' })}
                  width="lg"
                  request={async () => {
                    return modules;
                  }}
                  readonly={
                    category.id || category.module_id > 0 ? true : false
                  }
                  fieldProps={{
                    fieldNames: {
                      label: 'name',
                      value: 'id',
                    },
                    onChange: (e) => {
                      changeModule(e);
                    },
                  }}
                />
                <ProFormSelect
                  label={intl.formatMessage({ id: 'content.category.parent' })}
                  name="parent_id"
                  width="lg"
                  options={[
                    {
                      id: 0,
                      title: intl.formatMessage({
                        id: 'content.category.top',
                      }),
                      status: 1,
                    },
                  ]
                    .concat(
                      categories.filter((item) =>
                        category.id > 0
                          ? item.module_id === category.module_id &&
                            item.id !== category.id &&
                            item.parent_id !== category.id
                          : true,
                      ),
                    )
                    .map((cat: any) => ({
                      title: cat.title,
                      label: (
                        <div title={cat.title}>
                          {cat.parents?.length > 0 ? (
                            <span className="text-muted">
                              {cat.parents
                                .map((parent: any) => parent.title)
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
                <ProFormText
                  name="title"
                  label={intl.formatMessage({ id: 'content.category.title' })}
                />
                <ProFormTextArea
                  name="description"
                  label={intl.formatMessage({
                    id: 'content.category.description',
                  })}
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
                <ProFormText
                  name="seo_title"
                  label={intl.formatMessage({ id: 'content.seo-title.name' })}
                  placeholder={intl.formatMessage({
                    id: 'content.category.seo-title.placeholder',
                  })}
                  extra={
                    <div>
                      {intl.formatMessage({
                        id: 'content.category.seo-title.description',
                      })}
                      {', '}
                      <FormattedMessage id="setting.index.title.tips" />
                      <Popover
                        content={
                          <div style={{ whiteSpace: 'pre-wrap' }}>
                            {intl.formatMessage({
                              id: 'setting.index.params.tips',
                            })}
                          </div>
                        }
                        title={intl.formatMessage({
                          id: 'setting.index.title.tips',
                        })}
                      >
                        {' '}
                        <QuestionCircleOutlined />
                      </Popover>
                    </div>
                  }
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
                            <Suspense
                              fallback={<div style={{ height: 500 }} />}
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
                            </Suspense>
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
                      <Suspense fallback={<div style={{ height: 500 }} />}>
                        <MarkdownEditor
                          className="mb-normal"
                          setContent={async (html: string) => {
                            setContent(html);
                          }}
                          content={content}
                          ref={editorRef}
                        />
                      </Suspense>
                    ) : contentSetting.editor === 'simple' ? (
                      <Suspense fallback={<div style={{ height: 500 }} />}>
                        <SimpleEditor
                          className="mb-normal"
                          setContent={async (html: string) => {
                            setContent(html);
                          }}
                          content={content}
                          ref={editorRef}
                        />
                      </Suspense>
                    ) : (
                      <Suspense fallback={<div style={{ height: 500 }} />}>
                        <NewAiEditor
                          className="mb-normal"
                          setContent={async (html: string) => setContent(html)}
                          key="content"
                          field="content"
                          content={content}
                          ref={editorRef}
                        />
                      </Suspense>
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
                  title={intl.formatMessage({ id: 'content.category.banner' })}
                >
                  {categoryImages.length
                    ? categoryImages.map((item: string, index: number) => (
                        <div
                          className={
                            'ant-upload-item' +
                            (dragImageIndex === index ? ' drag-over' : '')
                          }
                          key={index}
                          draggable
                          onDragStart={() => handleDragStart(index)}
                          onDragOver={(e) => handleDragOver(e, index)}
                          onDragEnd={() => handleDragEnd}
                          onDrop={(e) => handleDrop(e)}
                        >
                          <Image
                            preview={{
                              src: item,
                            }}
                            src={item}
                          />
                          <div className="ant-upload-item-action">
                            <Tag
                              onClick={(e) => handleMoveImages(index, 'up', e)}
                            >
                              <LeftOutlined />
                            </Tag>
                            <Tag
                              color="red"
                              onClick={(e) => handleCleanImages(index, e)}
                            >
                              <DeleteOutlined />
                            </Tag>
                            <Tag
                              onClick={(e) =>
                                handleMoveImages(index, 'down', e)
                              }
                            >
                              <RightOutlined />
                            </Tag>
                          </div>
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

export default ArchiveCategoryDetail;
