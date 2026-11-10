import NewContainer from '@/components/NewContainer';
import AiGenerate from '@/components/aiGenerate';
import AiGetTdk from '@/components/aitdk';
import AttachmentSelect from '@/components/attachment';
import {
  anqiExtractDescription,
  getCategories,
  getCategoryInfo,
  getDesignTemplateFiles,
  getModules,
  getSettingContent,
  saveCategory,
} from '@/services';
import {
  DeleteOutlined,
  LeftOutlined,
  PlusOutlined,
  RightOutlined,
} from '@ant-design/icons';
import {
  ProForm,
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
import React, { lazy, Suspense, useEffect, useRef, useState } from 'react';
import '../index.less';
const MarkdownEditor = lazy(() => import('@/components/markdown'));
const NewAiEditor = lazy(() => import('@/components/newAiEditor'));
const SimpleEditor = lazy(() => import('@/components/simpleEditor'));

const categoryType = 3;

const PageCategoryDetail: React.FC = () => {
  const formRef = useRef<ProFormInstance>();
  const editorRef = useRef<any>(null);
  const [content, setContent] = useState<string>('');
  const [categoryImages, setCategoryImages] = useState<string[]>([]);
  const [categoryLogo, setCategoryLogo] = useState<string>('');
  const [currentModule, setCurrentModule] = useState<any>({});
  const [contentSetting, setContentSetting] = useState<any>({});
  const [loaded, setLoaded] = useState<boolean>(false);
  const [category, setCategory] = useState<any>({ status: 1 });
  const [modules, setModules] = useState<any[]>([]);
  const [newKey, setNewKey] = useState<string>('');
  const [aiTitle, setAiTitle] = useState<string>('');
  const [aiVisible, setAiVisible] = useState<boolean>(false);
  const [aiTdkVisible, setAiTdkVisible] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [dragImageIndex, setDragImageIndex] = useState<number>(-1);
  const intl = useIntl();

  const changeModule = (e: any) => {
    for (let item of modules) {
      if (item.id === e) {
        setCurrentModule(item);
        break;
      }
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
    let cat = catRes.data || { status: 1, parent_id: parent_id };
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
    changeModule(moduleId);

    setCategory(cat);

    setContent(cat.content || '');
    setCategoryImages(cat.images || []);
    setCategoryLogo(cat.logo || '');

    getSettingContent().then((res) => {
      setContentSetting(res.data || {});
      setLoaded(true);
    });
    getCategories({ type: 3 }).then((res) => {
      setCategories(res.data || []);
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
      message.error(intl.formatMessage({ id: 'content.page.input.required' }));
      return;
    }
    let res = await saveCategory(cat);
    if (res.code === 0) {
      message.info(res.msg);
      history.back();
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
          ? intl.formatMessage({ id: 'content.page.edit' })
          : intl.formatMessage({ id: 'content.page.new' })
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
                  label={intl.formatMessage({ id: 'content.page.parent' })}
                  name="parent_id"
                  width="lg"
                  options={[
                    {
                      id: 0,
                      title: intl.formatMessage({
                        id: 'content.page.top',
                      }),
                      status: 1,
                    },
                  ]
                    .concat(
                      categories.filter((item) =>
                        category.id > 0
                          ? item.id !== category.id &&
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
                  label={intl.formatMessage({ id: 'content.page.name' })}
                />
                <ProFormTextArea
                  name="description"
                  label={intl.formatMessage({ id: 'content.page.description' })}
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
                    id: 'content.page.seo-title.placeholder',
                  })}
                  extra={intl.formatMessage({
                    id: 'content.page.seo-title.description',
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
                  extra={intl.formatMessage({ id: 'content.page.status.tips' })}
                />
                {loaded && (
                  <Suspense fallback={<div style={{ height: 500 }} />}>
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
                  </Suspense>
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
                    label=""
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
                  title={intl.formatMessage({ id: 'content.page.template' })}
                >
                  <ProFormSelect
                    label=""
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
                        <FormattedMessage id="content.category.default" />:
                        page/detail.html
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

export default PageCategoryDetail;
