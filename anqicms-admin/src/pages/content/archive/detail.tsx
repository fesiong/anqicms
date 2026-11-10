import NewContainer from '@/components/NewContainer';
import AiGenerate from '@/components/aiGenerate';
import AiGetTdk from '@/components/aitdk';
import ArchiveSearch from '@/components/archiveSearch';
import AttachmentSelect from '@/components/attachment';
import ImageItem from '@/components/attachment/image';
import CollapseItem from '@/components/collaspeItem';
import Keywords from '@/components/keywords';
import SelectColor from '@/components/selectColor';
import {
  anqiExtractDescription,
  anqiExtractKeywords,
  deleteArchive,
  deleteArchiveImage,
  getArchiveInfo,
  getArchives,
  getCategories,
  getCategoryInfo,
  getDesignTemplateFiles,
  getModules,
  getSettingContent,
  pluginGetUserGroups,
  pluginGetUserInfo,
  pluginGetUsers,
  saveArchive,
} from '@/services';
import { getPlaces, getPlaceSetting } from '@/services/place';
import { getTags } from '@/services/tag';
import { getStore, removeStore, setStore } from '@/utils/store';
import {
  CloseOutlined,
  DeleteOutlined,
  DownOutlined,
  LeftOutlined,
  MoreOutlined,
  PlusOutlined,
  QuestionCircleOutlined,
  RightOutlined,
  UpOutlined,
} from '@ant-design/icons';
import {
  ModalForm,
  ProForm,
  ProFormCheckbox,
  ProFormDatePicker,
  ProFormDateTimePicker,
  ProFormDigit,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProFormTimePicker,
} from '@ant-design/pro-components';
import { FormattedMessage, history, injectIntl } from '@umijs/max';
import {
  Badge,
  Button,
  Card,
  Col,
  Dropdown,
  message,
  Modal,
  Popover,
  Row,
  Space,
  Tag,
} from 'antd';
import dayjs from 'dayjs';
import React from 'react';
import { IntlShape } from 'react-intl';
import { getRuntimeShopMode } from '@/utils/shopMode';
import './index.less';
const MarkdownEditor = React.lazy(() => import('@/components/markdown'));
const NewAiEditor = React.lazy(() => import('@/components/newAiEditor'));
const SimpleEditor = React.lazy(() => import('@/components/simpleEditor'));

export type intlProps = {
  intl: IntlShape;
};

class ArchiveForm extends React.Component<intlProps> {
  state: { [key: string]: any } = {
    fetched: false,
    archive: { extra: {}, content: '', flag: [] },
    relations: [],
    extraContent: {},
    extraTexts: {},
    extraTimelines: {},
    content: '',
    modules: [],
    module: { fields: [] },
    contentSetting: {},
    timelineExtraVisible: false,
    timelineField: '',
    timelineItemIndex: -1,
    dragImageIndex: null,

    archiveSearchVisible: false,
    keywordsVisible: false,
    searchedTags: [],
    searchArchives: [
      {
        id: 0,
        title: this.props.intl.formatMessage({
          id: 'content.parent_id.empty',
        }),
      },
    ],
    selectedArchives: [],
    searchUsers: [
      {
        id: 0,
        title: this.props.intl.formatMessage({
          id: 'content.parent_id.empty',
        }),
      },
    ],
    selectedUser: {},
    categories: [],
    placeSetting: {},
    places: [],

    aiVisible: false,
    aiTitle: '',
    aiTdkVisible: false,
    newKey: '',
  };

  loading = false;
  submitted = false;
  defaultContent = '';

  formRef = React.createRef<ProFormInstance>();

  editorRef = React.createRef<any>();

  initArchive = async () => {
    try {
      const setting = await getSettingContent();
      this.setState({
        contentSetting: setting.data || {},
      });
    } catch (err) {
      message.error(
        this.props.intl.formatMessage({ id: 'content.networt.error' }),
      );
      return;
    }
    const res = await getModules();
    this.setState(
      {
        // 商城模式产品走 /product 独立管理，这里排除；普通模式产品仍是内容模型
        modules: getRuntimeShopMode()
          ? (res.data || []).filter((item: any) => item.id !== 2)
          : res.data || [],
      },
      () => {
        const searchParams = new URLSearchParams(window.location.search);
        const moduleId = searchParams.get('module_id') || 1;
        let categoryId = Number(searchParams.get('category_id') || 0);
        const copyId = Number(searchParams.get('copyid') || 0);
        const lastCategoryId = getStore('last_category_id') || 0;
        if (categoryId === 0 && lastCategoryId > 0) {
          categoryId = lastCategoryId;
        }
        let id: any = searchParams.get('id') || 0;
        if (id === 'new') {
          id = 0;
        }
        if (id > 0) {
          this.getArchive(Number(id));
        } else {
          if (copyId > 0) {
            this.getArchive(Number(copyId), true);
          } else {
            const archive = getStore('unsaveArchive');
            if (archive) {
              console.log('load store');
              categoryId = archive.category_id;
              if (archive.category_ids?.length > 0) {
                categoryId = archive.category_ids[0];
              }
              let arcIds = [];
              if (archive.parent_id > 0) {
                arcIds.push(archive.parent_id);
              }
              this.getSelectedArchives(arcIds);
              this.setState({
                archive,
              });
            } else {
              this.setState({
                archive: {
                  extra: {},
                  content: '',
                  flag: [],
                  category_id: categoryId,
                },
              });
            }
            this.defaultContent = archive?.content || '';
            this.setState({
              fetched: true,
              content: archive?.content || '',
            });
          }
        }

        if (categoryId > 0) {
          this.getArchiveCategory(Number(categoryId));
        } else {
          // 先默认是文章
          this.getModule(Number(moduleId));
        }
      },
    );
    // 获取分类
    getCategories().then((res) => {
      const list = res.data || [];
      let categories = getRuntimeShopMode()
        ? list.filter((item: any) => item.module_id !== 2)
        : list;
      this.setState({
        categories,
      });
      if (categories.length === 0) {
        Modal.error({
          title: this.props.intl.formatMessage({
            id: 'content.category.error',
          }),
          onOk: () => {
            history.push('/archive/category');
          },
        });
      }
    });
    // placeSetting
    getPlaceSetting().then((res) => {
      this.setState({
        placeSetting: res.data || {},
      });
    });
  };

  componentDidMount = async () => {
    this.initArchive();

    window.addEventListener('beforeunload', this.beforeunload);
  };

  onTabChange = (key: string) => {
    this.setState({
      fetched: false,
    });
    this.initArchive().then(() => {
      this.setState({
        newKey: key,
      });
    });
  };

  beforeunload = (e: any) => {
    let archive = this.state.archive;
    if (!archive.id && !this.submitted) {
      console.log('save-store');
      const values = this.formRef.current?.getFieldsFormatValue?.();
      archive.content = this.state.content;
      archive = Object.assign(archive, values);
      if (typeof archive.flag === 'object') {
        archive.flag = archive.flag.join(',');
      }
      setStore('unsaveArchive', archive);
    }
    if (
      this.state.content !== '' &&
      this.state.content !== this.defaultContent
    ) {
      const confirmationMessage = this.props.intl.formatMessage({
        id: 'content.confirm-giveup',
      });
      (e || window.event).returnValue = confirmationMessage;
      return confirmationMessage;
    }

    return null;
  };

  componentWillUnmount() {
    let archive = this.state.archive;
    if (!archive.id && !this.submitted) {
      console.log('save-store');
      const values = this.formRef.current?.getFieldsFormatValue?.();
      archive.content = this.state.content;
      archive = Object.assign(archive, values);
      if (typeof archive.flag === 'object') {
        archive.flag = archive.flag.join(',');
      }
      setStore('unsaveArchive', archive);
    }
    window.removeEventListener('beforeunload', this.beforeunload);
  }

  getArchive = async (id: number, copy?: boolean) => {
    const res = await getArchiveInfo({
      id: id,
    });
    const archive = res.data || { extra: {}, flag: null };
    // 商城模式下产品有独立编辑页；普通模式下产品就是普通文档，留在本页编辑
    if (archive.module_id === 2 && getRuntimeShopMode()) {
      history.replace('/product/detail?id=' + archive.id);
    }
    if (copy) {
      archive.id = 0;
      archive.parent_id = 0;
      archive.url_token = '';
      archive.created_time = 0;
      archive.updated_time = 0;
    }
    // 价格转换为元
    archive.price = archive.price > 0 ? archive.price / 100 : 0;
    if (typeof archive.extra === 'undefined' || archive.extra === null) {
      archive.extra = {};
    }
    let content = archive.data?.content || '';
    archive.flag = archive.flag?.split(',') || [];
    archive.created_moment = dayjs(archive.created_time * 1000);
    archive.tags = archive.tags?.map((tag: any) => tag.title);
    this.defaultContent = content;
    const module = await this.getModule(archive.module_id);
    let extraContent: any = {};
    for (let i in module.fields) {
      if (module.fields[i].type === 'editor') {
        extraContent[module.fields[i].field_name] =
          archive.extra[module.fields[i].field_name]?.value || '';
      } else if (
        module.fields[i].type === 'date' ||
        module.fields[i].type === 'time' ||
        module.fields[i].type === 'datetime'
      ) {
        archive.extra[module.fields[i].field_name].value =
          archive.extra[module.fields[i].field_name]?.value || null;
      }
    }
    let arcIds = [];
    if (archive.parent_id > 0) {
      arcIds.push(archive.parent_id);
    }
    let extraTimelines: any = {};
    let extraTexts: any = {};
    for (let i in module.fields) {
      if (module.fields[i].type === 'texts') {
        extraTexts[module.fields[i].field_name] =
          archive.extra[module.fields[i].field_name]?.value || [];
      } else if (module.fields[i].type === 'timeline') {
        extraTimelines[module.fields[i].field_name] = archive.extra[
          module.fields[i].field_name
        ]?.value || {
          title: '',
          content: '',
          status: '',
          images: [],
          extra: {},
          items: [],
        };
      } else if (
        module.fields[i].type === 'archive' &&
        archive.extra[module.fields[i].field_name]?.value?.length > 0
      ) {
        arcIds.push(...archive.extra[module.fields[i].field_name]?.value);
      }
    }
    if (archive.user_id > 0) {
      pluginGetUserInfo({
        id: archive.user_id,
      }).then((res) => {
        if (res.data) {
          this.setState({
            selectedUser: res.data,
            searchUsers: [res.data],
          });
        }
      });
    }
    // category_ids 问题
    if (archive.category_id > 0) {
      archive.category_ids = archive.category_ids.filter(
        (catid: number) => catid !== archive.category_id,
      );
    }
    this.getSelectedArchives(arcIds);
    this.getArchiveCategory(archive.category_id);
    this.setState({
      fetched: true,
      archive: archive,
      content: content,
      extraContent: extraContent,
      extraTexts: extraTexts,
      extraTimelines: extraTimelines,
      relations: archive.relations || [],
    });
  };

  handleRemove = async (id: number) => {
    if (!id) {
      return;
    }
    Modal.confirm({
      title: this.props.intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          this.props.intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        try {
          await deleteArchive({
            id: id,
          });
          hide();
          message.success(
            this.props.intl.formatMessage({ id: 'content.delete.success' }),
          );
          // 跳回上一步
          history.back();

          return true;
        } catch (error) {
          hide();
          message.error(
            this.props.intl.formatMessage({ id: 'content.delete.failure' }),
          );
          return true;
        }
      },
    });
  };

  getSelectedArchives = (arcIds: number[]) => {
    if (arcIds.length > 0) {
      // 存在了再处理
      getArchives({
        id: arcIds.join(','),
        limit: 20,
      }).then((res) => {
        if (res.data) {
          this.setState({
            selectedArchives: res.data,
            searchArchives: res.data,
          });
        }
      });
    }
  };

  getArchiveCategory = async (categoryId: number) => {
    const res = await getCategoryInfo({
      id: categoryId,
    });
    const category = res.data || {};
    if (category.module_id) {
      this.getModule(category.module_id);
    }
  };

  onChangeSelectCategory = (e: any) => {
    let categoryId = 0;
    if (typeof e === 'number') {
      // 单分类
      categoryId = Number(e);
    } else {
      for (let i in e) {
        if (e[i] > 0) {
          categoryId = e[i];
          break;
        }
      }
    }
    setStore('last_category_id', categoryId);
    this.getArchiveCategory(categoryId);
  };

  getModule = async (moduleId: number) => {
    if (this.state.module.id === moduleId) {
      return this.state.module;
    }
    let module = { fields: [] };
    for (const item of this.state.modules) {
      if (item.id === moduleId) {
        module = item;
        break;
      }
    }
    this.setState({
      module: module,
    });
    return module;
  };

  setContent = async (html: string) => {
    this.setState({
      content: html,
    });
  };

  setExtraContent = async (field: string, html: string) => {
    const { extraContent } = this.state;
    extraContent[field] = html;
    this.setState({
      extraContent,
    });
  };

  handleSelectImages = (rows: any) => {
    const { archive } = this.state;
    if (!archive.images) {
      archive.images = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in archive.images) {
        if (archive.images[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        archive.images.push(row.file_path);
      }
    }
    this.setState({
      archive,
    });
    message.success(
      this.props.intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  handleDragStart = (index: number) => {
    this.setState({ dragImageIndex: index });
  };

  handleDragOver = (e: any, index: number) => {
    e.preventDefault();
    const { dragImageIndex, archive } = this.state;
    if (dragImageIndex === -1 || dragImageIndex === index) {
      return;
    }
    const images = [...(archive.images || [])];
    const [moved] = images.splice(dragImageIndex, 1);
    images.splice(index, 0, moved);
    this.setState({
      dragImageIndex: index,
      archive: { ...archive, images },
    });
  };

  handleDragEnd = () => {
    this.setState({ dragImageIndex: -1 });
  };

  handleDrop = (e: any) => {
    e.preventDefault();
    this.handleDragEnd();
  };

  handleMoveImages = (index: number, direction: string, e: any) => {
    e.stopPropagation();
    const { archive } = this.state;
    const images = [...(archive.images || [])];
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
    this.setState({
      archive: { ...archive, images },
    });
  };

  handleCleanLogo = (index: number, e: any) => {
    e.stopPropagation();
    const { archive } = this.state;
    // 请求接口删除
    deleteArchiveImage({ id: archive.id, image_index: index })
      .then(() => {
        archive.logo = '';
        archive.images.splice(index, 1);
        this.setState({
          archive,
        });
      })
      .catch(() => {});
  };

  handleExtractDescription = () => {
    if (this.state.content.length < 100) {
      message.error(
        this.props.intl.formatMessage({
          id: 'content.archive.content.length.error',
        }),
      );
      return false;
    }
    Modal.confirm({
      title: this.props.intl.formatMessage({
        id: 'content.archive.extract.description',
      }),
      content: this.props.intl.formatMessage({
        id: 'content.archive.extract.description.content',
      }),
      onOk: () => {
        anqiExtractDescription({
          text: this.state.content,
        })
          .then((res) => {
            if (res.code === 0) {
              this.formRef?.current?.setFieldsValue({
                description: res.data,
              });
            } else {
              message.info(res.msg);
            }
          })
          .catch((err) => {
            message.error(
              err.msg ||
                this.props.intl.formatMessage({
                  id: 'content.submit.failure',
                }),
            );
          });
      },
    });
  };

  handleExtractKeywords = () => {
    if (this.state.content.length < 100) {
      message.error(
        this.props.intl.formatMessage({
          id: 'content.archive.content.length.error',
        }),
      );
      return false;
    }
    Modal.confirm({
      title: this.props.intl.formatMessage({
        id: 'content.archive.extract.keywords',
      }),
      content: this.props.intl.formatMessage({
        id: 'content.archive.extract.keywords.content',
      }),
      onOk: () => {
        anqiExtractKeywords({
          text: this.state.content,
        })
          .then((res) => {
            if (res.code === 0) {
              this.formRef?.current?.setFieldsValue({
                keywords: res.data?.join(','),
              });
            } else {
              message.info(res.msg);
            }
          })
          .catch((err) => {
            message.error(
              err.msg ||
                this.props.intl.formatMessage({
                  id: 'content.submit.failure',
                }),
            );
          });
      },
    });
  };

  handleChooseKeywords = () => {
    this.setState({
      keywordsVisible: true,
    });
  };

  handleHideKeywords = () => {
    this.setState({
      keywordsVisible: false,
    });
  };

  handleShowArchiveSearchs = () => {
    this.setState({
      archiveSearchVisible: true,
    });
  };

  handleHideArchiveSearchs = () => {
    this.setState({
      archiveSearchVisible: false,
    });
  };

  handleSelectedArchives = async (values: any[]) => {
    const { relations } = this.state;
    for (let i in values) {
      if (!values.hasOwnProperty(i)) continue;
      let exist = false;
      for (let j in relations) {
        if (!relations.hasOwnProperty(j)) continue;
        if (relations[j].id === values[i].id) {
          exist = true;
          break;
        }
      }
      if (!exist) {
        relations.push(values[i]);
      }
    }
    this.setState({
      relations,
    });
    this.handleHideArchiveSearchs();
  };

  handleRemoveRelation = (index: number) => {
    const { relations } = this.state;
    relations.splice(index, 1);
    this.setState({
      relations,
    });
  };

  handleSelectedKeywords = async (values: string[]) => {
    const keywords = (
      this.formRef?.current?.getFieldValue('keywords') || ''
    ).split(',');
    for (const item of values) {
      if (keywords.indexOf(item) === -1) {
        keywords.push(item);
      }
    }
    this.formRef?.current?.setFieldsValue({
      keywords: keywords.join(',').replace(/^,+/, '').replace(/,+$/, ''),
    });
    this.handleHideKeywords();
  };

  onChangeTagInput = (e: any) => {
    const value = e.target?.value || '';
    getTags({
      type: 1,
      title: value,
      pageSize: 10,
    }).then((res) => {
      const data = res.data || [];
      const result: any = {};
      for (const item of data) {
        result[item.title] = item.title;
      }
      this.setState({
        searchedTags: result,
      });
    });
  };

  onSubmit = async (values: any) => {
    if (this.loading) {
      return;
    }
    const {
      archive,
      content,
      extraContent,
      extraTexts,
      extraTimelines,
      relations,
    } = this.state;
    this.loading = true;
    const postData = Object.assign(archive, values);
    postData.relation_ids = relations.map((item: any) => item.id);
    delete postData.relations;
    postData.price = Number((values.price * 100).toFixed(0));
    postData.stock = Number(values.stock);
    // eslint-disable-next-line guard-for-in
    for (let field in extraContent) {
      if (!postData.extra[field]) {
        postData.extra[field] = {};
      }
      postData.extra[field].value = extraContent[field];
    }
    // eslint-disable-next-line guard-for-in
    for (let field in extraTexts) {
      if (!postData.extra[field]) {
        postData.extra[field] = {};
      }
      postData.extra[field].value = extraTexts[field]
        .map((item: any) => {
          return {
            key: item.key || '',
            value: (item.values || [''])[0],
            values: item.values || [''],
          };
        })
        .filter((item: any) => item.key);
    }
    // eslint-disable-next-line guard-for-in
    for (let field in extraTimelines) {
      if (!postData.extra[field]) {
        postData.extra[field] = {};
      }
      postData.extra[field].value = extraTimelines[field];
    }
    // 必须选择分类
    let categoryIds = [];
    let categoryId = values.category_id;
    if (categoryId === 0) {
      this.loading = false;
      message.error(
        this.props.intl.formatMessage({ id: 'content.category.required' }),
      );
      return;
    }
    // 优先
    categoryIds.push(categoryId);
    if (values.category_ids) {
      for (let i in values.category_ids) {
        if (
          values.category_ids[i] > 0 &&
          values.category_ids[i] !== categoryId
        ) {
          categoryIds.push(values.category_ids[i]);
        }
      }
    }
    postData.category_id = categoryId;
    postData.category_ids = categoryIds;
    const hide = message.loading(
      this.props.intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    postData.content = content;
    if (typeof postData.flag === 'object') {
      postData.flag = postData.flag.join(',');
    }
    const res = await saveArchive(postData);
    this.loading = false;
    hide();
    if (res.code !== 0) {
      if (res.data && res.data.id) {
        // 提示
        Modal.confirm({
          title: res.msg,
          content: this.props.intl.formatMessage({
            id: 'content.submit.confirm',
          }),
          cancelText: this.props.intl.formatMessage({
            id: 'content.submit.cancel',
          }),
          okText: this.props.intl.formatMessage({
            id: 'content.submit.ok-force',
          }),
          onOk: () => {
            values.force_save = true;
            this.onSubmit(values);
          },
        });
        return;
      }
      message.error(res.msg);
    } else {
      // 设置最近更新过的文档
      setStore('latest_update', {
        id: res.data.id,
        timestamp: new Date().getTime() / 1000,
      });
      removeStore('unsaveArchive');
      this.submitted = true;
      message.success(res.msg);
      history.back();
    }
  };

  handleCleanExtraField = (field: string) => {
    const extra: any = {};
    extra[field] = { value: '' };
    this.formRef?.current?.setFieldsValue({ extra });

    const { archive } = this.state;
    delete archive.extra[field];
    this.setState({
      archive,
    });
  };

  handleChangeExtraField = (field: string, value: any) => {
    const extra: any = {};
    extra[field] = { value: value };
    this.formRef?.current?.setFieldsValue({ extra });
    const { archive } = this.state;
    if (!archive.extra[field]) {
      archive.extra[field] = {};
    }
    archive.extra[field].value = value;

    this.setState({
      archive,
    });
  };

  handleUploadExtraField = (field: string, row: any) => {
    const extra: any = {};
    extra[field] = { value: row.file_path };
    this.formRef?.current?.setFieldsValue({ extra });
    const { archive } = this.state;
    if (!archive.extra[field]) {
      archive.extra[field] = {};
    }
    archive.extra[field].value = row.file_path;

    this.setState({
      archive,
    });
  };

  handleCleanExtraFieldItem = (field: string, index: number) => {
    const { archive } = this.state;
    archive.extra[field].value?.splice(index, 1);
    const extra: any = {};
    extra[field] = { value: archive.extra[field].value };
    this.formRef?.current?.setFieldsValue({ extra });

    this.setState({
      archive,
    });
  };

  handleMoveExtraFieldItem = (
    field: string,
    index: number,
    direction: 'up' | 'down',
  ) => {
    const { archive } = this.state;
    if (direction === 'up') {
      if (index <= 0) {
        return;
      }
      const temp = archive.extra[field].value[index];
      archive.extra[field].value[index] = archive.extra[field].value[index - 1];
      archive.extra[field].value[index - 1] = temp;
    } else {
      if (index >= archive.extra[field].value.length - 1) {
        return;
      }
      const temp = archive.extra[field].value[index];
      archive.extra[field].value[index] = archive.extra[field].value[index + 1];
      archive.extra[field].value[index + 1] = temp;
    }
    this.setState({
      archive,
    });
  };

  handleUploadExtraFieldItem = (field: string, rows: any) => {
    const { archive } = this.state;
    if (!archive.extra[field]) {
      archive.extra[field] = {
        value: [],
      };
    }
    if (!archive.extra[field].value?.length) {
      archive.extra[field].value = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in archive.extra[field].value) {
        if (archive.extra[field].value[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        archive.extra[field].value.push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = { value: archive.extra[field].value };
    this.formRef?.current?.setFieldsValue({ extra });

    this.setState({
      archive,
    });
  };

  onAddExtraTextsField = (field: string) => {
    const { extraTexts } = this.state;
    if (!extraTexts[field]) {
      extraTexts[field] = [];
    }
    extraTexts[field].push({
      key: '',
      value: '',
      values: [''],
    });
    const extra: any = {};
    extra[field] = { value: extraTexts[field] };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTexts,
    });
  };

  onChangeExtraTextsField = (
    field: string,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    const { extraTexts } = this.state;
    if (!extraTexts[field][idx]) {
      extraTexts[field][idx] = {};
    }
    extraTexts[field][idx][keyName] = value;

    const extra: any = {};
    extra[field] = { value: { idx: { keyName: value } } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTexts,
    });
  };

  onChangeExtraTextsFieldValue = (
    field: string,
    idx: number,
    valueIdx: number,
    value: any,
  ) => {
    const { extraTexts } = this.state;
    if (!extraTexts[field][idx]) {
      extraTexts[field][idx] = {};
    }
    extraTexts[field][idx].values[valueIdx] = value;

    const extra: any = {};
    extra[field] = {
      value: { idx: { values: extraTexts[field][idx].values } },
    };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTexts,
    });
  };

  onMoveUpExtraTextsField = (field: string, idx: number) => {
    const { extraTexts } = this.state;
    // 移动
    if (idx > 0) {
      const tmp = extraTexts[field][idx];
      extraTexts[field][idx] = extraTexts[field][idx - 1];
      extraTexts[field][idx - 1] = tmp;
      const extra: any = {};
      extra[field] = { value: extraTexts[field] };
      this.formRef?.current?.setFieldsValue({ extra });
      this.setState({
        extraTexts,
      });
    }
  };

  onMoveDownExtraTextsField = (field: string, idx: number) => {
    const { extraTexts } = this.state;
    // 移动
    if (idx < extraTexts[field].length - 1) {
      const tmp = extraTexts[field][idx];
      extraTexts[field][idx] = extraTexts[field][idx + 1];
      extraTexts[field][idx + 1] = tmp;
      const extra: any = {};
      extra[field] = { value: extraTexts[field] };
      this.formRef?.current?.setFieldsValue({ extra });
      this.setState({
        extraTexts,
      });
    }
  };

  onRemoveExtraTextsField = (field: string, idx: number) => {
    const { extraTexts } = this.state;
    Modal.confirm({
      title: this.props.intl.formatMessage({
        id: 'content.module.field.delete.confirm',
      }),
      content: this.props.intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: () => {
        if (extraTexts[field].length === 1) {
          extraTexts[field] = [];
        } else {
          extraTexts[field].splice(idx, 1);
        }
        const extra: any = {};
        extra[field] = { value: extraTexts[field] };
        this.formRef?.current?.setFieldsValue({ extra });
        this.setState({
          extraTexts,
        });
      },
    });
  };

  onAddExtraTextsFieldValue = (field: string, idx: number) => {
    const { extraTexts } = this.state;
    extraTexts[field][idx].values.push('');
    const extra: any = {};
    extra[field] = { value: extraTexts[field] };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTexts,
    });
  };

  onReduceExtraTextsFieldValue = (
    field: string,
    idx: number,
    valueIdx: number,
  ) => {
    const { extraTexts } = this.state;
    if (extraTexts[field][idx].values.length <= 1) {
      return;
    }
    extraTexts[field][idx].values.splice(valueIdx, 1);
    const extra: any = {};
    extra[field] = { value: extraTexts[field] };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTexts,
    });
  };

  //////////
  onChangeExtraTimelineField = (field: string, keyName: any, value: any) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field]) {
      extraTimelines[field] = {
        title: '',
        content: '',
        status: '',
        images: [],
        extra: {},
        items: [],
      };
    }
    extraTimelines[field][keyName] = value;

    const extra: any = {};
    extra[field] = { value: { idx: { keyName: value } } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onChangeExtraTimelineExtra = (
    field: string,
    extraKey: string,
    extraValue: any,
  ) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].extra) {
      extraTimelines[field].extra = {};
    }
    extraTimelines[field].extra[extraKey] = extraValue;
    const extra: any = {};
    extra[field] = {
      value: { extra: extraTimelines[field].extra },
    };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  handleUploadExtraTimelineImage = (field: string, rows: any) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].images) {
      extraTimelines[field].images = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in extraTimelines[field].images) {
        if (extraTimelines[field].images[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        extraTimelines[field].images.push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = { value: { images: extraTimelines[field].images } };
    this.formRef?.current?.setFieldsValue({ extra });

    this.setState({
      extraTimelines,
    });
  };

  onAddExtraTimelineItem = (field: string) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].items) {
      extraTimelines[field].items = [];
    }
    extraTimelines[field].items.push({
      title: '',
      content: '',
      status: '',
      images: [],
      extra: {},
      items: [],
    });

    const extra: any = {};
    extra[field] = {
      value: { items: extraTimelines[field].items },
    };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onChangeExtraTimelineItemField = (
    field: string,
    idx: number,
    keyName: any,
    value: any,
  ) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].items[idx]) {
      extraTimelines[field].items[idx] = {
        title: '',
        content: '',
        status: '',
        images: [],
        extra: {},
        items: [],
      };
    }
    extraTimelines[field].items[idx][keyName] = value;

    const extra: any = {};
    extra[field] = {
      value: { items: extraTimelines[field].items },
    };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onChangeExtraTimelineItemExtra = (
    field: string,
    idx: number,
    extraKey: string,
    extraValue: any,
  ) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].items[idx].extra) {
      extraTimelines[field].items[idx].extra = {};
    }
    extraTimelines[field].items[idx].extra[extraKey] = extraValue;

    const extra: any = {};
    extra[field] = {
      value: { items: extraTimelines[field].items },
    };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  handleUploadExtraTimelineItemImage = (
    field: string,
    idx: number,
    rows: any,
  ) => {
    const { extraTimelines } = this.state;
    if (!extraTimelines[field].items[idx].images) {
      extraTimelines[field].items[idx].images = [];
    }
    for (const row of rows) {
      let exists = false;
      for (const i in extraTimelines[field].items[idx].images) {
        if (extraTimelines[field].items[idx].images[i] === row.file_path) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        extraTimelines[field].items[idx].images.push(row.file_path);
      }
    }
    const extra: any = {};
    extra[field] = {
      value: { items: extraTimelines[field].items },
    };
    this.formRef?.current?.setFieldsValue({ extra });

    this.setState({
      extraTimelines,
    });
  };

  onMoveUpExtraTimelineItem = (field: string, idx: number) => {
    const { extraTimelines } = this.state;
    // 移动
    if (idx > 0) {
      const tmp = extraTimelines[field].items[idx];
      extraTimelines[field].items[idx] = extraTimelines[field].items[idx - 1];
      extraTimelines[field].items[idx - 1] = tmp;
      const extra: any = {};
      extra[field] = { value: { items: extraTimelines[field].items } };
      this.formRef?.current?.setFieldsValue({ extra });
      this.setState({
        extraTimelines,
      });
    }
  };

  onMoveDownExtraTimelineItem = (field: string, idx: number) => {
    const { extraTimelines } = this.state;
    // 移动
    if (idx < extraTimelines[field].items.length - 1) {
      const tmp = extraTimelines[field].items[idx];
      extraTimelines[field].items[idx] = extraTimelines[field].items[idx + 1];
      extraTimelines[field].items[idx + 1] = tmp;
      const extra: any = {};
      extra[field] = { value: { items: extraTimelines[field].items } };
      this.formRef?.current?.setFieldsValue({ extra });
      this.setState({
        extraTimelines,
      });
    }
  };

  onRemoveExtraTimelineItem = (field: string, idx: number) => {
    const { extraTimelines } = this.state;
    Modal.confirm({
      title: this.props.intl.formatMessage({
        id: 'content.module.field.delete.confirm',
      }),
      content: this.props.intl.formatMessage({
        id: 'content.module.field.delete.content',
      }),
      onOk: () => {
        if (extraTimelines[field].items.length === 1) {
          extraTimelines[field] = [];
        } else {
          extraTimelines[field].items.splice(idx, 1);
        }
        const extra: any = {};
        extra[field] = { value: { items: extraTimelines[field].items } };
        this.formRef?.current?.setFieldsValue({ extra });
        this.setState({
          extraTimelines,
        });
      },
    });
  };

  onRemoveExtraTimelineImage = (field: string, imageIdx: number) => {
    const { extraTimelines } = this.state;
    if (extraTimelines[field].images.length <= 1) {
      extraTimelines[field] = [];
    } else {
      extraTimelines[field].images.splice(imageIdx, 1);
    }
    const extra: any = {};
    extra[field] = { value: { images: extraTimelines[field].images } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onRemoveExtraTimelineItemImage = (
    field: string,
    idx: number,
    imageIdx: number,
  ) => {
    const { extraTimelines } = this.state;
    if (extraTimelines[field].items[idx].images.length <= 1) {
      extraTimelines[field] = [];
    } else {
      extraTimelines[field].items[idx].images.splice(imageIdx, 1);
    }
    const extra: any = {};
    extra[field] = { value: { items: extraTimelines[field].items } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onReduceExtraTimelineExtra = (field: string, extraKey: string) => {
    const { extraTimelines } = this.state;
    if (extraTimelines[field].extra.length < 1) {
      return;
    }
    delete extraTimelines[field].extra[extraKey];
    const extra: any = {};
    extra[field] = { value: { extra: extraTimelines[field].extra } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  onReduceExtraTimelineItemExtra = (
    field: string,
    idx: number,
    extraKey: string,
  ) => {
    const { extraTimelines } = this.state;
    if (extraTimelines[field].items[idx].extra.length < 1) {
      return;
    }
    delete extraTimelines[field].items[idx].extra[extraKey];
    const extra: any = {};
    extra[field] = { value: { items: extraTimelines[field].items } };
    this.formRef?.current?.setFieldsValue({ extra });
    this.setState({
      extraTimelines,
    });
  };

  handleAddTimelineExtra = (field: string) => {
    this.setState({
      timelineField: field,
      timelineExtraVisible: true,
      timelineItemIndex: -1,
    });
  };

  handleAddTimelineItemExtra = (field: string, idx: number) => {
    this.setState({
      timelineField: field,
      timelineExtraVisible: true,
      timelineItemIndex: idx,
    });
  };
  //////////

  handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === 's' && (event.ctrlKey || event.metaKey)) {
      const values = this.formRef.current?.getFieldsFormatValue?.();
      // 自动保存
      this.onSubmit(values);

      event.preventDefault();
    }
  };

  aiGenerateArticle = () => {
    const values = this.formRef.current?.getFieldsFormatValue?.() || {};
    this.setState({
      aiTitle: values.title,
      aiVisible: true,
    });
  };

  onHideAiGenerate = () => {
    this.setState({
      aiVisible: false,
    });
  };

  aiTdkGenerate = () => {
    this.setState({
      aiTdkVisible: true,
    });
  };

  onHideAiTdkVisible = () => {
    this.setState({
      aiTdkVisible: false,
    });
  };

  onFinishAiGenerate = async (values: any) => {
    this.onHideAiGenerate();
    this.formRef.current?.setFieldsValue({ title: values.title });
    //
    let content = values.content.trim();

    this.setContent(content);
    this.editorRef.current?.setInnerContent(content);
  };

  onFinishAiTdk = async (values: any) => {
    this.onHideAiTdkVisible();
    let data: any = {};
    if (values.title?.length > 0) {
      data.title = values.title;
    }
    if (values.keywords?.length > 0) {
      data.keywords = values.keywords;
    }
    if (values.description?.length > 0) {
      data.description = values.description;
    }
    this.formRef.current?.setFieldsValue(data);
  };

  onSearchArchives = (e: any) => {
    getArchives({ title: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (this.state.selectedArchives) {
        searchItems.push(...this.state.selectedArchives);
      } else {
        searchItems.push({
          id: 0,
          title: this.props.intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      this.setState({
        searchArchives: searchItems.concat(res.data || []),
      });
    });
  };

  onSearchUsers = (e: any) => {
    pluginGetUsers({ q: e, pageSize: 10 }).then((res) => {
      // 如果是已经有选择的 ParentId,则把它加入到开头
      const searchItems: any[] = [];
      if (this.state.selectedUser.id) {
        searchItems.push(this.state.selectedUser);
      } else {
        searchItems.push({
          id: 0,
          title: this.props.intl.formatMessage({
            id: 'content.parent_id.empty',
          }),
        });
      }
      this.setState({
        searchUsers: searchItems.concat(res.data || []),
      });
    });
  };

  render() {
    const {
      archive,
      content,
      extraContent,
      extraTexts,
      extraTimelines,
      module,
      fetched,
      keywordsVisible,
      searchedTags,
      aiTitle,
      aiVisible,
      aiTdkVisible,
      contentSetting,
      archiveSearchVisible,
      timelineExtraVisible,
      timelineField,
      timelineItemIndex,
      relations,
      searchArchives,
      searchUsers,
      categories,
      placeSetting,
      newKey,
    } = this.state;
    return (
      <NewContainer
        title={
          <div className="heading-title">
            {archive.id > 0 ? (
              <div>
                {archive.title}{' '}
                {archive.status === 1 ? (
                  <Badge
                    className="site-badge-count-109"
                    count={this.props.intl.formatMessage({
                      id: 'content.status.normal',
                    })}
                    style={{ backgroundColor: '#52c41a' }}
                  />
                ) : (
                  <Badge
                    className="site-badge-count-109"
                    count={this.props.intl.formatMessage({
                      id: 'content.status.draft',
                    })}
                    style={{ backgroundColor: '#999999' }}
                  />
                )}
              </div>
            ) : (
              <FormattedMessage id="content.archive.add" />
            )}
          </div>
        }
        extra={
          archive.id > 0 ? (
            <Space size={8}>
              <Button
                onClick={() => {
                  history.push('/archive/detail?copyid=' + archive.id);
                }}
              >
                <FormattedMessage id="content.action.copy" />
              </Button>
              <Button
                onClick={() => {
                  window.open(archive.link);
                }}
              >
                <FormattedMessage id="menu.preview" />
              </Button>
              <Dropdown
                menu={{
                  items: [
                    {
                      key: 'delete',
                      label: this.props.intl.formatMessage({
                        id: 'setting.system.delete',
                      }),
                      onClick: () => {
                        this.handleRemove(archive.id);
                      },
                    },
                  ],
                }}
                key="more"
              >
                <Button>
                  <FormattedMessage id="content.action.more" />
                  <MoreOutlined />
                </Button>
              </Dropdown>
            </Space>
          ) : null
        }
        onTabChange={(key) => this.onTabChange(key)}
      >
        <Card key={newKey} onKeyDown={this.handleKeyDown}>
          {fetched && (
            <ProForm
              initialValues={archive}
              layout="horizontal"
              formRef={this.formRef}
              onFinish={this.onSubmit}
            >
              <Row gutter={20}>
                <Col sm={18} xs={24}>
                  <ProFormText
                    name="title"
                    label={this.props.intl.formatMessage({
                      id: 'content.title.name',
                    })}
                  />
                  <ProFormCheckbox.Group
                    name="flag"
                    label={this.props.intl.formatMessage({
                      id: 'content.flag.name',
                    })}
                    valueEnum={{
                      h: this.props.intl.formatMessage({
                        id: 'content.flag.h',
                      }),
                      c: this.props.intl.formatMessage({
                        id: 'content.flag.c',
                      }),
                      f: this.props.intl.formatMessage({
                        id: 'content.flag.f',
                      }),
                      a: this.props.intl.formatMessage({
                        id: 'content.flag.a',
                      }),
                      s: this.props.intl.formatMessage({
                        id: 'content.flag.s',
                      }),
                      b: this.props.intl.formatMessage({
                        id: 'content.flag.b',
                      }),
                      p: this.props.intl.formatMessage({
                        id: 'content.flag.p',
                      }),
                      j: this.props.intl.formatMessage({
                        id: 'content.flag.j',
                      }),
                    }}
                  />
                  <ProFormText
                    name="keywords"
                    label={this.props.intl.formatMessage({
                      id: 'content.keywords.name',
                    })}
                    fieldProps={{
                      suffix: (
                        <Space size={16}>
                          <span
                            className="link"
                            onClick={this.handleExtractKeywords}
                          >
                            <FormattedMessage id="content.keywords.extract" />
                          </span>
                          <span
                            className="link"
                            onClick={this.handleChooseKeywords}
                          >
                            <FormattedMessage id="content.keywords.select" />
                          </span>
                        </Space>
                      ),
                    }}
                  />
                  <ProFormTextArea
                    name="description"
                    label={this.props.intl.formatMessage({
                      id: 'content.description.name',
                    })}
                    extra={
                      <div className="mt-small">
                        <Space size={20}>
                          <span
                            className="link extract-tag"
                            onClick={this.aiTdkGenerate}
                          >
                            <FormattedMessage id="component.aitdk.btn.generate" />
                          </span>
                          <span
                            className="link extract-tag"
                            onClick={this.handleExtractDescription}
                          >
                            <FormattedMessage id="content.description.extract" />
                          </span>
                        </Space>
                      </div>
                    }
                  />

                  <CollapseItem
                    header={this.props.intl.formatMessage({
                      id: 'content.param.other',
                    })}
                    showArrow
                    key="1"
                  >
                    <Row gutter={20}>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          label={this.props.intl.formatMessage({
                            id: 'content.origin-url.name',
                          })}
                          name="origin_url"
                          placeholder={this.props.intl.formatMessage({
                            id: 'content.field.default',
                          })}
                          extra={this.props.intl.formatMessage({
                            id: 'content.origin-url.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          name="seo_title"
                          label={this.props.intl.formatMessage({
                            id: 'content.seo-title.name',
                          })}
                          placeholder={this.props.intl.formatMessage({
                            id: 'content.seo-title.placeholder',
                          })}
                          extra={
                            <div>
                              {this.props.intl.formatMessage({
                                id: 'content.seo-title.description',
                              })}
                              {', '}
                              <FormattedMessage id="setting.index.title.tips" />
                              <Popover
                                content={
                                  <div style={{ whiteSpace: 'pre-wrap' }}>
                                    {this.props.intl.formatMessage({
                                      id: 'setting.index.params.tips',
                                    })}
                                  </div>
                                }
                                title={this.props.intl.formatMessage({
                                  id: 'setting.index.title.tips',
                                })}
                              >
                                {' '}
                                <QuestionCircleOutlined />
                              </Popover>
                            </div>
                          }
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          name="canonical_url"
                          label={this.props.intl.formatMessage({
                            id: 'content.canonical-url.name',
                          })}
                          placeholder={this.props.intl.formatMessage({
                            id: 'content.canonical-url.placeholder',
                          })}
                          extra={this.props.intl.formatMessage({
                            id: 'content.canonical-url.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          name="fixed_link"
                          label={this.props.intl.formatMessage({
                            id: 'content.fixed-link.name',
                          })}
                          placeholder={this.props.intl.formatMessage({
                            id: 'content.canonical-url.placeholder',
                          })}
                          extra={this.props.intl.formatMessage({
                            id: 'content.fixed-link.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormSelect
                          label={this.props.intl.formatMessage({
                            id: 'content.archive-template.name',
                          })}
                          showSearch
                          name="template"
                          request={async () => {
                            const res = await getDesignTemplateFiles({});
                            const data = [
                              {
                                path: '',
                                remark: this.props.intl.formatMessage({
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
                          extra={this.props.intl.formatMessage({
                            id: 'content.archive-template.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.price.name',
                          })}
                          name="price"
                          fieldProps={{
                            precision: 0,
                            addonAfter: this.props.intl.formatMessage({
                              id: 'content.price.suffix',
                            }),
                          }}
                          extra={this.props.intl.formatMessage({
                            id: 'content.papriceram.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.stock.name',
                          })}
                          name="stock"
                          fieldProps={{
                            precision: 0,
                            addonAfter: this.props.intl.formatMessage({
                              id: 'content.stock.suffix',
                            }),
                          }}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormSelect
                          name="read_level"
                          label={this.props.intl.formatMessage({
                            id: 'content.read-level.name',
                          })}
                          request={async () => {
                            const res = await pluginGetUserGroups({});
                            return [
                              {
                                level: 0,
                                title: this.props.intl.formatMessage({
                                  id: 'content.unlimit',
                                }),
                                id: 0,
                              },
                            ].concat(res.data || []);
                          }}
                          fieldProps={{
                            fieldNames: {
                              label: 'title',
                              value: 'level',
                            },
                            optionItemRender(item: any) {
                              return (
                                <div
                                  dangerouslySetInnerHTML={{
                                    __html: 'L' + item.level + item.title,
                                  }}
                                ></div>
                              );
                            },
                          }}
                          extra={this.props.intl.formatMessage({
                            id: 'content.read-level.description',
                          })}
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          name="password"
                          label={this.props.intl.formatMessage({
                            id: 'content.password.name',
                          })}
                          placeholder={this.props.intl.formatMessage({
                            id: 'content.password.placeholder',
                          })}
                          extra={this.props.intl.formatMessage({
                            id: 'content.password.description',
                          })}
                        />
                      </Col>
                    </Row>
                  </CollapseItem>
                  {module.fields && (
                    <CollapseItem
                      header={this.props.intl.formatMessage({
                        id: 'content.param.extra-fields',
                      })}
                      open
                      showArrow
                      key="2"
                    >
                      <Row gutter={20}>
                        {module.fields?.map(
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
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'number' ? (
                                  <ProFormDigit
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'textarea' ? (
                                  <ProFormTextArea
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'editor' ? (
                                  ''
                                ) : item.type === 'radio' ? (
                                  <ProFormRadio.Group
                                    name={['extra', item.field_name, 'value']}
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
                                    name={['extra', item.field_name, 'value']}
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
                                    name={['extra', item.field_name, 'value']}
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
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                  >
                                    {archive.extra?.[item.field_name]?.value ? (
                                      <div className="ant-upload-item">
                                        <ImageItem
                                          src={
                                            archive.extra?.[item.field_name]
                                              ?.value
                                          }
                                          size={100}
                                        />
                                        <span
                                          className="delete"
                                          onClick={this.handleCleanExtraField.bind(
                                            this,
                                            item.field_name,
                                          )}
                                        >
                                          <DeleteOutlined />
                                        </span>
                                      </div>
                                    ) : (
                                      <AttachmentSelect
                                        onSelect={this.handleUploadExtraField.bind(
                                          this,
                                          item.field_name,
                                        )}
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
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                  >
                                    {archive.extra?.[item.field_name]?.value
                                      ?.length
                                      ? archive.extra[
                                          item.field_name
                                        ].value.map(
                                          (inner: string, idx: number) => (
                                            <div
                                              className="ant-upload-item"
                                              key={idx}
                                            >
                                              <ImageItem
                                                src={inner}
                                                size={100}
                                              />
                                              <div className="ant-upload-item-action">
                                                <Tag
                                                  onClick={this.handleMoveExtraFieldItem.bind(
                                                    this,
                                                    item.field_name,
                                                    idx,
                                                    'up',
                                                  )}
                                                >
                                                  <LeftOutlined />
                                                </Tag>
                                                <Tag
                                                  color="red"
                                                  onClick={this.handleCleanExtraFieldItem.bind(
                                                    this,
                                                    item.field_name,
                                                    idx,
                                                  )}
                                                >
                                                  <DeleteOutlined />
                                                </Tag>
                                                <Tag
                                                  onClick={this.handleMoveExtraFieldItem.bind(
                                                    this,
                                                    item.field_name,
                                                    idx,
                                                    'down',
                                                  )}
                                                >
                                                  <RightOutlined />
                                                </Tag>
                                              </div>
                                            </div>
                                          ),
                                        )
                                      : null}
                                    <AttachmentSelect
                                      onSelect={this.handleUploadExtraFieldItem.bind(
                                        this,
                                        item.field_name,
                                      )}
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
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                  >
                                    {archive.extra?.[item.field_name]?.value ? (
                                      <div className="ant-upload-item ant-upload-file">
                                        <span>
                                          {
                                            archive.extra?.[item.field_name]
                                              ?.value
                                          }
                                        </span>
                                        <span
                                          className="delete"
                                          onClick={this.handleCleanExtraField.bind(
                                            this,
                                            item.field_name,
                                          )}
                                        >
                                          <DeleteOutlined />
                                        </span>
                                      </div>
                                    ) : (
                                      <AttachmentSelect
                                        onSelect={this.handleUploadExtraField.bind(
                                          this,
                                          item.field_name,
                                        )}
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
                                        <div className="text-value">Values</div>
                                        <div className="text-action"></div>
                                      </div>
                                      {extraTexts?.[item.field_name]?.length
                                        ? extraTexts[item.field_name].map(
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
                                                      'value',
                                                      idx,
                                                      'key',
                                                    ]}
                                                    fieldProps={{
                                                      onChange: (e: any) => {
                                                        this.onChangeExtraTextsField(
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
                                                  <Space
                                                    size={16}
                                                    direction="vertical"
                                                    style={{ width: '100%' }}
                                                  >
                                                    {inner.values?.map(
                                                      (
                                                        value: string,
                                                        valueIdx: number,
                                                      ) => (
                                                        <ProFormText
                                                          key={valueIdx}
                                                          name={[
                                                            'extra',
                                                            item.field_name,
                                                            'value',
                                                            idx,
                                                            'values',
                                                            valueIdx,
                                                          ]}
                                                          fieldProps={{
                                                            onChange: (
                                                              e: any,
                                                            ) => {
                                                              this.onChangeExtraTextsFieldValue(
                                                                item.field_name,
                                                                idx,
                                                                valueIdx,
                                                                e.target.value,
                                                              );
                                                            },
                                                            prefix: inner.values
                                                              .length > 1 && (
                                                              <span>
                                                                {valueIdx + 1}.
                                                              </span>
                                                            ),
                                                            addonAfter: (
                                                              <Tag
                                                                onClick={() =>
                                                                  this.onReduceExtraTextsFieldValue(
                                                                    item.field_name,
                                                                    idx,
                                                                    valueIdx,
                                                                  )
                                                                }
                                                              >
                                                                <DeleteOutlined />
                                                              </Tag>
                                                            ),
                                                          }}
                                                        />
                                                      ),
                                                    )}
                                                  </Space>
                                                </div>
                                                <div className="text-action">
                                                  <Tag
                                                    onClick={() =>
                                                      this.onAddExtraTextsFieldValue(
                                                        item.field_name,
                                                        idx,
                                                      )
                                                    }
                                                  >
                                                    <PlusOutlined />
                                                  </Tag>
                                                  <Tag
                                                    onClick={() =>
                                                      this.onMoveUpExtraTextsField(
                                                        item.field_name,
                                                        idx,
                                                      )
                                                    }
                                                  >
                                                    <UpOutlined />
                                                  </Tag>
                                                  <Tag
                                                    onClick={() =>
                                                      this.onMoveDownExtraTextsField(
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
                                                      this.onRemoveExtraTextsField(
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
                                              this.onAddExtraTextsField(
                                                item.field_name,
                                              )
                                            }
                                          >
                                            {this.props.intl.formatMessage({
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
                                      name={['extra', item.field_name, 'value']}
                                      showSearch
                                      mode="multiple"
                                      options={searchArchives.map((a: any) => ({
                                        title: a.title,
                                        label: a.title,
                                        value: a.id,
                                      }))}
                                      fieldProps={{
                                        onSearch: (e) => {
                                          this.onSearchArchives(e);
                                        },
                                      }}
                                    />
                                  </ProFormText>
                                ) : item.type === 'category' ? (
                                  <ProFormText label={item.name}>
                                    <ProFormSelect
                                      showSearch
                                      name={['extra', item.field_name, 'value']}
                                      mode={'single'}
                                      options={[
                                        {
                                          title: this.props.intl.formatMessage({
                                            id: 'content.please-select',
                                          }),
                                          value: 0,
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
                                ) : item.type === 'timeline' ? (
                                  <ProFormText
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    extra={
                                      <div className="mt-small">
                                        <Button
                                          onClick={() =>
                                            this.onAddExtraTimelineItem(
                                              item.field_name,
                                            )
                                          }
                                        >
                                          <FormattedMessage id="content.param.add-timeline" />
                                        </Button>
                                      </div>
                                    }
                                  >
                                    <div className="timeline-container">
                                      <div className="timeline-header">
                                        <ProFormText
                                          label={this.props.intl.formatMessage({
                                            id: 'content.module.field.timeline.title',
                                          })}
                                          name={[
                                            'extra',
                                            item.field_name,
                                            'value',
                                            'title',
                                          ]}
                                          fieldProps={{
                                            onChange: (e: any) => {
                                              this.onChangeExtraTimelineField(
                                                item.field_name,
                                                'title',
                                                e.target.value,
                                              );
                                            },
                                          }}
                                        />
                                        <ProFormTextArea
                                          label={this.props.intl.formatMessage({
                                            id: 'content.module.field.timeline.content',
                                          })}
                                          name={[
                                            'extra',
                                            item.field_name,
                                            'value',
                                            'content',
                                          ]}
                                          fieldProps={{
                                            onChange: (e: any) => {
                                              this.onChangeExtraTimelineField(
                                                item.field_name,
                                                'content',
                                                e.target.value,
                                              );
                                            },
                                          }}
                                        />
                                        <ProFormText
                                          label={this.props.intl.formatMessage({
                                            id: 'content.module.field.timeline.status',
                                          })}
                                          name={[
                                            'extra',
                                            item.field_name,
                                            'value',
                                            'status',
                                          ]}
                                          fieldProps={{
                                            onChange: (e: any) => {
                                              this.onChangeExtraTimelineField(
                                                item.field_name,
                                                'status',
                                                e.target.value,
                                              );
                                            },
                                          }}
                                        />
                                        <ProFormText
                                          name={[
                                            'extra',
                                            item.field_name,
                                            'value',
                                            'images',
                                          ]}
                                          label={this.props.intl.formatMessage({
                                            id: 'content.module.field.timeline.images',
                                          })}
                                        >
                                          {extraTimelines?.[item.field_name]
                                            ?.images?.length
                                            ? extraTimelines[
                                                item.field_name
                                              ].images.map(
                                                (
                                                  inner: string,
                                                  idx: number,
                                                ) => (
                                                  <div
                                                    className="ant-upload-item"
                                                    key={idx}
                                                  >
                                                    <ImageItem
                                                      src={inner}
                                                      size={100}
                                                    />
                                                    <div className="ant-upload-item-action">
                                                      <Tag
                                                        color="red"
                                                        onClick={this.onRemoveExtraTimelineImage.bind(
                                                          this,
                                                          item.field_name,
                                                          idx,
                                                        )}
                                                      >
                                                        <DeleteOutlined />
                                                      </Tag>
                                                    </div>
                                                  </div>
                                                ),
                                              )
                                            : null}
                                          <AttachmentSelect
                                            onSelect={this.handleUploadExtraTimelineImage.bind(
                                              this,
                                              item.field_name,
                                            )}
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
                                        <ProFormText
                                          label={this.props.intl.formatMessage({
                                            id: 'content.module.field.timeline.extra',
                                          })}
                                        >
                                          <div className="text-groups">
                                            <div className="text-group">
                                              <div className="text-key">
                                                Key
                                              </div>
                                              <div className="text-value">
                                                Value
                                              </div>
                                              <div className="text-action"></div>
                                            </div>
                                            {extraTimelines?.[item.field_name]
                                              ?.extra
                                              ? Object.entries(
                                                  extraTimelines[
                                                    item.field_name
                                                  ].extra,
                                                ).map(
                                                  ([extraKey, inner]: [
                                                    string,
                                                    any,
                                                  ]) => (
                                                    <div
                                                      className="text-group"
                                                      key={extraKey}
                                                    >
                                                      <div className="text-key">
                                                        {extraKey}
                                                      </div>
                                                      <div className="text-value">
                                                        {inner}
                                                      </div>
                                                      <div className="text-action">
                                                        <Tag
                                                          color="red"
                                                          onClick={() =>
                                                            this.onReduceExtraTimelineExtra(
                                                              item.field_name,
                                                              extraKey,
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
                                                    this.handleAddTimelineExtra(
                                                      item.field_name,
                                                    )
                                                  }
                                                >
                                                  <FormattedMessage id="content.module.field.timeline.extra.add" />
                                                </Tag>
                                              </div>
                                            </div>
                                          </div>
                                        </ProFormText>
                                      </div>
                                    </div>
                                    <div className="timeline-groups">
                                      {extraTimelines?.[item.field_name]?.items
                                        ?.length
                                        ? extraTimelines[
                                            item.field_name
                                          ].items.map(
                                            (
                                              itemInner: string,
                                              idx: number,
                                            ) => (
                                              <CollapseItem
                                                header={'Item ' + (idx + 1)}
                                                open
                                                showArrow
                                                key={'timeline-item-' + idx}
                                                extra={
                                                  <Space size={16}>
                                                    <Tag
                                                      onClick={(e) => {
                                                        e.stopPropagation();
                                                        this.onMoveUpExtraTimelineItem(
                                                          item.field_name,
                                                          idx,
                                                        );
                                                      }}
                                                    >
                                                      <UpOutlined />
                                                    </Tag>
                                                    <Tag
                                                      onClick={(e) => {
                                                        e.stopPropagation();
                                                        this.onMoveDownExtraTimelineItem(
                                                          item.field_name,
                                                          idx,
                                                        );
                                                      }}
                                                    >
                                                      <DownOutlined />
                                                    </Tag>
                                                    <Tag
                                                      color="red"
                                                      onClick={(e) => {
                                                        e.stopPropagation();
                                                        this.onRemoveExtraTimelineItem(
                                                          item.field_name,
                                                          idx,
                                                        );
                                                      }}
                                                    >
                                                      <DeleteOutlined />
                                                    </Tag>
                                                  </Space>
                                                }
                                              >
                                                <div className="timeline-container">
                                                  <div className="timeline-header">
                                                    <ProFormText
                                                      label={this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.module.field.timeline.title',
                                                        },
                                                      )}
                                                      name={[
                                                        'extra',
                                                        item.field_name,
                                                        'value',
                                                        'items',
                                                        idx,
                                                        'title',
                                                      ]}
                                                      fieldProps={{
                                                        onChange: (e: any) => {
                                                          this.onChangeExtraTimelineItemField(
                                                            item.field_name,
                                                            idx,
                                                            'title',
                                                            e.target.value,
                                                          );
                                                        },
                                                      }}
                                                    />
                                                    <ProFormTextArea
                                                      label={this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.module.field.timeline.content',
                                                        },
                                                      )}
                                                      name={[
                                                        'extra',
                                                        item.field_name,
                                                        'value',
                                                        'items',
                                                        idx,
                                                        'content',
                                                      ]}
                                                      fieldProps={{
                                                        onChange: (e: any) => {
                                                          this.onChangeExtraTimelineItemField(
                                                            item.field_name,
                                                            idx,
                                                            'content',
                                                            e.target.value,
                                                          );
                                                        },
                                                      }}
                                                    />
                                                    <ProFormText
                                                      label={this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.module.field.timeline.status',
                                                        },
                                                      )}
                                                      name={[
                                                        'extra',
                                                        item.field_name,
                                                        'value',
                                                        'items',
                                                        idx,
                                                        'status',
                                                      ]}
                                                      fieldProps={{
                                                        onChange: (e: any) => {
                                                          this.onChangeExtraTimelineItemField(
                                                            item.field_name,
                                                            idx,
                                                            'status',
                                                            e.target.value,
                                                          );
                                                        },
                                                      }}
                                                    />
                                                    <ProFormText
                                                      name={[
                                                        'extra',
                                                        item.field_name,
                                                        'value',
                                                        'items',
                                                        idx,
                                                        'images',
                                                      ]}
                                                      label={this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.module.field.timeline.images',
                                                        },
                                                      )}
                                                    >
                                                      {extraTimelines?.[
                                                        item.field_name
                                                      ]?.items[idx]?.images
                                                        ?.length
                                                        ? extraTimelines[
                                                            item.field_name
                                                          ].items[
                                                            idx
                                                          ].images.map(
                                                            (
                                                              imgInner: string,
                                                              imgIdx: number,
                                                            ) => (
                                                              <div
                                                                className="ant-upload-item"
                                                                key={imgIdx}
                                                              >
                                                                <ImageItem
                                                                  src={imgInner}
                                                                  size={100}
                                                                />
                                                                <div className="ant-upload-item-action">
                                                                  <Tag
                                                                    color="red"
                                                                    onClick={this.onRemoveExtraTimelineItemImage.bind(
                                                                      this,
                                                                      item.field_name,
                                                                      idx,
                                                                      imgIdx,
                                                                    )}
                                                                  >
                                                                    <DeleteOutlined />
                                                                  </Tag>
                                                                </div>
                                                              </div>
                                                            ),
                                                          )
                                                        : null}
                                                      <AttachmentSelect
                                                        onSelect={this.handleUploadExtraTimelineItemImage.bind(
                                                          this,
                                                          item.field_name,
                                                          idx,
                                                        )}
                                                        open={false}
                                                        multiple={true}
                                                      >
                                                        <div className="ant-upload-item">
                                                          <div className="add">
                                                            <PlusOutlined />
                                                            <div
                                                              style={{
                                                                marginTop: 8,
                                                              }}
                                                            >
                                                              <FormattedMessage id="setting.system.upload" />
                                                            </div>
                                                          </div>
                                                        </div>
                                                      </AttachmentSelect>
                                                    </ProFormText>
                                                    <ProFormText
                                                      label={this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.module.field.timeline.extra',
                                                        },
                                                      )}
                                                    >
                                                      <div className="text-groups">
                                                        <div className="text-group">
                                                          <div className="text-key">
                                                            Key
                                                          </div>
                                                          <div className="text-value">
                                                            Value
                                                          </div>
                                                          <div className="text-action"></div>
                                                        </div>
                                                        {extraTimelines?.[
                                                          item.field_name
                                                        ]?.items[idx]?.extra
                                                          ? Object.entries(
                                                              extraTimelines[
                                                                item.field_name
                                                              ].items[idx]
                                                                .extra,
                                                            ).map(
                                                              ([
                                                                extraKey,
                                                                inner,
                                                              ]: [
                                                                string,
                                                                any,
                                                              ]) => (
                                                                <div
                                                                  className="text-group"
                                                                  key={extraKey}
                                                                >
                                                                  <div className="text-key">
                                                                    {extraKey}
                                                                  </div>
                                                                  <div className="text-value">
                                                                    {inner}
                                                                  </div>
                                                                  <div className="text-action">
                                                                    <Tag
                                                                      color="red"
                                                                      onClick={() =>
                                                                        this.onReduceExtraTimelineItemExtra(
                                                                          item.field_name,
                                                                          idx,
                                                                          extraKey,
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
                                                                this.handleAddTimelineItemExtra(
                                                                  item.field_name,
                                                                  idx,
                                                                )
                                                              }
                                                            >
                                                              <FormattedMessage id="content.module.field.timeline.extra.add" />
                                                            </Tag>
                                                          </div>
                                                        </div>
                                                      </div>
                                                    </ProFormText>
                                                  </div>
                                                </div>
                                              </CollapseItem>
                                            ),
                                          )
                                        : null}
                                    </div>
                                  </ProFormText>
                                ) : item.type === 'date' ? (
                                  <ProFormDatePicker
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'time' ? (
                                  <ProFormTimePicker
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'datetime' ? (
                                  <ProFormDateTimePicker
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                    required={item.required ? true : false}
                                    placeholder={
                                      item.content &&
                                      this.props.intl.formatMessage({
                                        id: 'content.param.default',
                                      }) + item.content
                                    }
                                  />
                                ) : item.type === 'color' ? (
                                  <ProFormText
                                    name={['extra', item.field_name, 'value']}
                                    label={item.name}
                                  >
                                    <SelectColor
                                      defaultValue={
                                        archive.extra?.[item.field_name]
                                          ?.value || null
                                      }
                                      showText
                                      onChangeComplete={(value) => {
                                        this.handleChangeExtraField(
                                          item.field_name,
                                          value.toHexString(),
                                        );
                                      }}
                                    />
                                  </ProFormText>
                                ) : null}
                              </Col>
                            ),
                        )}
                      </Row>
                      {module.fields?.map(
                        (item: any, index: number) =>
                          item.type === 'editor' && (
                            <ProFormText
                              key={index}
                              label={item.name}
                              required={item.required ? true : false}
                              extra={
                                item.content &&
                                this.props.intl.formatMessage({
                                  id: 'content.param.default',
                                }) + item.content
                              }
                            >
                              {contentSetting.editor === 'markdown' ? (
                                <MarkdownEditor
                                  className="mb-normal"
                                  setContent={this.setExtraContent.bind(
                                    this,
                                    item.field_name,
                                  )}
                                  content={extraContent[item.field_name] || ''}
                                  ref={null}
                                />
                              ) : contentSetting.editor === 'simple' ? (
                                <SimpleEditor
                                  className="mb-normal"
                                  setContent={this.setExtraContent.bind(
                                    this,
                                    item.field_name,
                                  )}
                                  content={extraContent[item.field_name] || ''}
                                  ref={null}
                                />
                              ) : (
                                <NewAiEditor
                                  className="mb-normal"
                                  setContent={this.setExtraContent.bind(
                                    this,
                                    item.field_name,
                                  )}
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
                      setContent={this.setContent}
                      content={content}
                      ref={this.editorRef}
                    />
                  ) : contentSetting.editor === 'simple' ? (
                    <SimpleEditor
                      className="mb-normal"
                      setContent={this.setContent}
                      content={content}
                      ref={this.editorRef}
                    />
                  ) : (
                    <NewAiEditor
                      className="mb-normal"
                      setContent={this.setContent}
                      content={content}
                      key="content"
                      field="content"
                      ref={this.editorRef}
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
                            this.onSubmit(
                              this.formRef.current?.getFieldsFormatValue?.() ||
                                {},
                            );
                          }}
                        >
                          <FormattedMessage id="content.submit.ok" />
                        </Button>
                      </Col>
                      <Col span={12}>
                        <Button
                          block
                          onClick={() => {
                            const values =
                              this.formRef.current?.getFieldsFormatValue?.() ||
                              {};
                            values.draft = true;
                            this.onSubmit(values);
                          }}
                        >
                          <FormattedMessage id="content.submit.draft" />
                        </Button>
                      </Col>
                      <Col span={12}>
                        <Button
                          block
                          onClick={() => {
                            this.aiGenerateArticle();
                          }}
                        >
                          <FormattedMessage id="content.submit.aigenerate" />
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
                    title={this.props.intl.formatMessage({
                      id: 'content.category.name',
                    })}
                  >
                    <ProFormSelect
                      //label="所属分类"
                      name="category_id"
                      width="lg"
                      options={categories.map((cat: any) => ({
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
                        onChange: this.onChangeSelectCategory,
                      }}
                      extra={
                        <div>
                          <FormattedMessage id="content.module.name" />:{' '}
                          {module.name}
                        </div>
                      }
                    />
                    {contentSetting.multi_category === 1 && (
                      <ProFormSelect
                        name="category_ids"
                        width="lg"
                        mode="multiple"
                        options={categories.map((cat: any) => ({
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
                        extra={
                          <div>
                            <FormattedMessage id="content.archive.related-category.description" />
                          </div>
                        }
                      />
                    )}
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.images.name',
                    })}
                  >
                    <ProFormText>
                      {archive.images?.length
                        ? archive.images.map((item: string, index: number) => (
                            <div
                              className={
                                'ant-upload-item' +
                                (this.state.dragImageIndex === index
                                  ? ' drag-over'
                                  : '')
                              }
                              key={index}
                              draggable
                              onDragStart={() => this.handleDragStart(index)}
                              onDragOver={(e) => this.handleDragOver(e, index)}
                              onDragEnd={this.handleDragEnd}
                              onDrop={this.handleDrop}
                            >
                              <ImageItem src={item} size={100} />
                              <div className="ant-upload-item-action">
                                <Tag
                                  onClick={this.handleMoveImages.bind(
                                    this,
                                    index,
                                    'up',
                                  )}
                                >
                                  <LeftOutlined />
                                </Tag>
                                <Tag
                                  color="red"
                                  onClick={this.handleCleanLogo.bind(
                                    this,
                                    index,
                                  )}
                                >
                                  <DeleteOutlined />
                                </Tag>
                                <Tag
                                  onClick={this.handleMoveImages.bind(
                                    this,
                                    index,
                                    'down',
                                  )}
                                >
                                  <RightOutlined />
                                </Tag>
                              </div>
                            </div>
                          ))
                        : null}
                      <AttachmentSelect
                        onSelect={this.handleSelectImages}
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
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.url-token.name',
                    })}
                  >
                    <ProFormText
                      name="url_token"
                      placeholder={this.props.intl.formatMessage({
                        id: 'content.url-token.placeholder',
                      })}
                      extra={this.props.intl.formatMessage({
                        id: 'content.url-token.tips',
                      })}
                    />
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.create-time.name',
                    })}
                  >
                    <ProFormDateTimePicker
                      name="created_moment"
                      placeholder={this.props.intl.formatMessage({
                        id: 'content.url-token.placeholder',
                      })}
                      extra={this.props.intl.formatMessage({
                        id: 'content.create-time.description',
                      })}
                      transform={(value) => {
                        return {
                          created_time: value ? dayjs(value).unix() : 0,
                        };
                      }}
                    />
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.tag.name',
                    })}
                  >
                    <ProFormSelect
                      mode="tags"
                      name="tags"
                      valueEnum={searchedTags}
                      placeholder={this.props.intl.formatMessage({
                        id: 'content.tag.placeholder',
                      })}
                      fieldProps={{
                        tokenSeparators: [',', '，'],
                        onInputKeyDown: this.onChangeTagInput,
                        onFocus: this.onChangeTagInput,
                      }}
                      extra={this.props.intl.formatMessage({
                        id: 'content.tag.placeholder',
                      })}
                    />
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.author.name',
                    })}
                    extra={
                      <Tag
                        className="link"
                        onClick={() => {
                          history.push('/plugin/commerce/user');
                        }}
                      >
                        <FormattedMessage id="setting.action.add" />
                      </Tag>
                    }
                  >
                    <ProFormSelect
                      name="user_id"
                      showSearch
                      options={searchUsers.map((a: any) => ({
                        label: a.user_name,
                        value: a.id,
                      }))}
                      fieldProps={{
                        onSearch: (e) => {
                          this.onSearchUsers(e);
                        },
                      }}
                    />
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.parent_id.name',
                    })}
                  >
                    <ProFormSelect
                      name="parent_id"
                      showSearch
                      options={searchArchives.map((a: any) => ({
                        title: a.title,
                        label: a.title,
                        value: a.id,
                        disabled: a.id === archive.id,
                      }))}
                      fieldProps={{
                        onSearch: (e) => {
                          this.onSearchArchives(e);
                        },
                      }}
                    />
                  </Card>
                  {placeSetting.open && (
                    <Card
                      className="aside-card"
                      size="small"
                      title={this.props.intl.formatMessage({
                        id: 'content.place.title',
                      })}
                    >
                      <ProFormSelect
                        showSearch
                        name="place_id"
                        request={async () => {
                          const res = await getPlaces();
                          return [
                            {
                              title: this.props.intl.formatMessage({
                                id: 'content.parent_id.empty',
                              }),
                              value: 0,
                              status: 1,
                            },
                          ]
                            .concat(res.data || [])
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
                            }));
                        }}
                        fieldProps={{
                          showSearch: true,
                          filterOption: (input: string, option: any) =>
                            (option?.title ?? option?.label)
                              .toLowerCase()
                              .includes(input.toLowerCase()),
                        }}
                      />
                    </Card>
                  )}
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.relation.name',
                    })}
                    extra={
                      <Tag
                        className="link"
                        onClick={this.handleShowArchiveSearchs}
                      >
                        <FormattedMessage id="setting.action.add" />
                      </Tag>
                    }
                  >
                    {relations.map((item: any, index: number) => (
                      <div className="relation-item" key={item.id}>
                        <div className="name">{item.title}</div>
                        <div
                          className="close link"
                          onClick={this.handleRemoveRelation.bind(this, index)}
                        >
                          <CloseOutlined />
                        </div>
                      </div>
                    ))}
                  </Card>
                </Col>
              </Row>
            </ProForm>
          )}
        </Card>
        {keywordsVisible && (
          <Keywords
            open={keywordsVisible}
            onCancel={this.handleHideKeywords}
            onSubmit={this.handleSelectedKeywords}
          />
        )}
        {aiVisible && (
          <AiGenerate
            open={aiVisible}
            title={aiTitle}
            editor={contentSetting.editor}
            onCancel={this.onHideAiGenerate}
            onSubmit={this.onFinishAiGenerate}
          />
        )}
        {aiTdkVisible && (
          <AiGetTdk
            open={aiTdkVisible}
            content={content}
            onCancel={this.onHideAiTdkVisible}
            onSubmit={this.onFinishAiTdk}
          />
        )}
        {archiveSearchVisible && (
          <ArchiveSearch
            open={archiveSearchVisible}
            onCancel={this.handleHideArchiveSearchs}
            onSubmit={this.handleSelectedArchives}
          />
        )}
        {timelineExtraVisible && (
          <ModalForm
            width={600}
            title={this.props.intl.formatMessage({
              id: 'content.module.field.timeline.extra',
            })}
            open={timelineExtraVisible}
            layout="vertical"
            onOpenChange={(flag) => {
              if (!flag) {
                this.setState({ timelineExtraVisible: false });
              }
            }}
            onFinish={async (values) => {
              if (timelineItemIndex < 0) {
                this.onChangeExtraTimelineExtra(
                  timelineField,
                  values.key,
                  values.value,
                );
              } else {
                this.onChangeExtraTimelineItemExtra(
                  timelineField,
                  timelineItemIndex,
                  values.key,
                  values.value,
                );
              }
              this.setState({ timelineExtraVisible: false });
            }}
          >
            <ProFormText name="key" label="Key" />
            <ProFormText name="value" label="Value" />
          </ModalForm>
        )}
      </NewContainer>
    );
  }
}

export default injectIntl(ArchiveForm);
