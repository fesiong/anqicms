import NewContainer from '@/components/NewContainer';
import AiGenerate from '@/components/aiGenerate';
import AiGetTdk from '@/components/aitdk';
import ArchiveSearch from '@/components/archiveSearch';
import AttachmentSelect from '@/components/attachment';
import ImageItem from '@/components/attachment/image';
import CollapseItem from '@/components/collaspeItem';
import Keywords from '@/components/keywords';
import MarkdownEditor from '@/components/markdown';
import NewAiEditor from '@/components/newAiEditor';
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
  pluginGetShippingSetting,
  pluginGetUserGroups,
  pluginGetUserInfo,
  pluginGetUsers,
  saveArchive,
} from '@/services';
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
  ProFormDigitRange,
  ProFormGroup,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProFormTimePicker,
} from '@ant-design/pro-components';
import '@ant-design/pro-table/';
import { FormattedMessage, history, injectIntl } from '@umijs/max';
import {
  Avatar,
  Badge,
  Button,
  Card,
  Checkbox,
  Col,
  ColorPicker,
  Dropdown,
  Flex,
  Image,
  MenuProps,
  Modal,
  Popover,
  Row,
  Space,
  Tag,
  Tooltip,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React from 'react';
import { IntlShape } from 'react-intl';
import OrderFieldsForm from './components/orderFields';
import './index.less';

export type intlProps = {
  intl: IntlShape;
};

const OptionTypeSingle = 0;
const OptionTypeMulti = 1;
const OptionTypeAddon = 2;

class ProductForm extends React.Component<intlProps> {
  state: { [key: string]: any } = {
    fetched: false,
    archive: {
      extra: {},
      content: '',
      flag: [],
      option_type: OptionTypeSingle,
      product_type: 1, // 1普通商品 2批发商品 3定制商品 4虚拟商品 5服务商品 6订阅商品
    },
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
    options: [],
    skus: [],
    originSkus: [],
    skuFieldsVisible: false,
    skuFields: [
      { label: 'Logo', value: 'logo', checked: false },
      { label: 'Sku', value: 'sku', checked: false },
      { label: 'Barcode', value: 'barcode', checked: false },
      { label: 'Hs Code', value: 'hs_code', checked: false },
      { label: 'Original Price', value: 'origin_price', checked: false },
      { label: 'Cost Price', value: 'cost_price', checked: false },
      { label: 'Weight', value: 'weight', checked: false },
      // { label: 'Weight Unit', value: 'weight_unit', checked: false },
    ],
    shippingSettings: {},

    wholesaleRules: [],

    showOrderFields: false,

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
    // 读取sippping units
    pluginGetShippingSetting({}).then((res) => {
      this.setState({
        shippingSettings: res.data || {},
      });
    });
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
        modules: res.data || [],
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
            const archive = getStore('unsaveProduct');
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
                options: archive.options || [],
                skus: archive.skus || [],
              });
            } else {
              this.setState({
                archive: {
                  extra: {},
                  content: '',
                  flag: [],
                  category_ids: [categoryId],
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
    getCategories({ type: 1, module_id: 2 }).then((res) => {
      let categories = res.data || [];
      this.setState({
        categories,
      });
      if (categories.length === 0) {
        Modal.error({
          title: this.props.intl.formatMessage({
            id: 'content.category.error',
          }),
          onOk: () => {
            history.push('/product/category');
          },
        });
      }
    });
  };

  componentDidMount = async () => {
    this.initArchive();

    let cachedFields = getStore('product-sku-fields');
    if (cachedFields) {
      let skuFields = this.state.skuFields;
      for (let i = 0; i < cachedFields.length; i++) {
        const field = cachedFields[i];
        const index = skuFields.findIndex((item: any) => item.value === field);
        if (index >= 0) {
          skuFields[index].checked = true;
        }
      }
      this.setState({
        skuFields,
      });
    }

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
      archive.options = this.state.options;
      archive.skus = this.state.skus;
      setStore('unsaveProduct', archive);
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
      archive.options = this.state.options;
      archive.skus = this.state.skus;
      setStore('unsaveProduct', archive);
    }
    window.removeEventListener('beforeunload', this.beforeunload);
  }

  getArchive = async (id: number, copy?: boolean) => {
    const res = await getArchiveInfo({
      id: id,
    });
    const archive = res.data || { extra: {}, flag: null };
    if (copy) {
      archive.id = 0;
      archive.parent_id = 0;
      archive.url_token = '';
      archive.created_time = 0;
      archive.updated_time = 0;
    }
    // 价格转换为元
    archive.price = archive.price > 0 ? archive.price / 100 : 0;
    archive.origin_price =
      archive.origin_price > 0 ? archive.origin_price / 100 : 0;
    archive.cost_price = archive.cost_price > 0 ? archive.cost_price / 100 : 0;
    // 计算利润
    archive.profit =
      archive.price > 0 && archive.cost_price > 0
        ? archive.price - archive.cost_price
        : 0;
    archive.profit_percent =
      archive.profit > 0 && archive.price > 0
        ? (archive.profit / archive.price) * 100
        : 0;
    //end
    const options =
      archive.options?.map((item: any) => {
        item.values = item.values?.map((value: any) => {
          value.price = value.price / 100 || 0;
          if (copy) {
            value.id = 0;
            value.archive_id = 0;
          }
          return value;
        });
        if (copy) {
          item.id = 0;
          item.archive_id = 0;
        }
        return item;
      }) || [];
    // skus
    const skus = archive.skus?.map((item: any) => {
      item.price = item.price > 0 ? item.price / 100 : 0;
      item.origin_price = item.origin_price > 0 ? item.origin_price / 100 : 0;
      item.cost_price = item.cost_price > 0 ? item.cost_price / 100 : 0;
      item.profit = item.price - item.cost_price;
      item.profit_percent =
        item.profit > 0 && item.cost_price > 0
          ? (item.profit / item.price) * 100
          : 0;
      if (copy) {
        item.values?.map((value: any) => {
          value.id = 0;
          value.archive_id = 0;
          value.option_id = 0;
          return value;
        });
      }
      return item;
    });
    const originSkus = Object.assign([], skus);

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
    let wholesaleRules: any[] = [];
    if (archive.wholesale_rules) {
      wholesaleRules = archive.wholesale_rules.map((rule: any) => {
        return {
          id: rule.id,
          range: [rule.min_quantity, rule.max_quantity],
          discount_rate: rule.discount_rate,
        };
      });
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

    this.getSelectedArchives(arcIds);
    this.getArchiveCategory(archive.category_id);
    this.setState(
      {
        fetched: true,
        archive: archive,
        content: content,
        extraContent: extraContent,
        extraTexts: extraTexts,
        extraTimelines: extraTimelines,
        relations: archive.relations || [],
        options: options || [],
        originSkus: originSkus || [],
        wholesaleRules: wholesaleRules || [],
      },
      () => {
        this.buildSku(options, archive.option_type);
      },
    );
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

  onChangeProductType = (e: any) => {
    const { archive } = this.state;
    archive.product_type = Number(e.target.value);
    this.setState({
      archive,
    });
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

  handleSelectVideo = (row: any) => {
    const { archive } = this.state;
    archive.video_url = row.file_location || '';
    this.setState({
      archive,
    });
    message.success(
      this.props.intl.formatMessage({ id: 'setting.system.upload-success' }),
    );
  };

  handleCleanVideo = () => {
    const { archive } = this.state;
    archive.video_url = '';
    this.setState({
      archive,
    });
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
      skus,
      wholesaleRules,
      options,
    } = this.state;
    this.loading = true;
    const postData = Object.assign(archive, values);
    postData.relation_ids = relations.map((item: any) => item.id);
    delete postData.relations;
    postData.product_type = Number(values.product_type);
    postData.price = Number((values.price * 100).toFixed(0));
    postData.origin_price = Number((values.origin_price * 100).toFixed(0));
    postData.cost_price = Number((values.cost_price * 100).toFixed(0));
    postData.stock = Number(values.stock);
    postData.skus = skus.map((item: any) => {
      const newItem = { ...item };
      newItem.price = Number((newItem.price * 100).toFixed(0));
      newItem.origin_price = Number((newItem.origin_price * 100).toFixed(0));
      newItem.cost_price = Number((newItem.cost_price * 100).toFixed(0));
      newItem.weight_unit = postData.weight_unit;
      return newItem;
    });
    postData.wholesale_rules = [];
    let lastMaxQuantity = 0;
    wholesaleRules.forEach((item: any) => {
      item.min_quantity = Number(item.range[0]);
      item.max_quantity = Number(item.range[1]);
      if (lastMaxQuantity >= item.min_quantity) {
        item.min_quantity = lastMaxQuantity + 1;
      }
      lastMaxQuantity = item.max_quantity;
      if (item.max_quantity > 0 && item.max_quantity <= item.min_quantity) {
        return;
      }
      if (item.min_quantity > 0 && item.discount_rate > 0) {
        postData.wholesale_rules.push(item);
      }
    });
    postData.options = options.map((item: any) => {
      item.values = item.values?.map((value: any) => {
        value.price = Number((value.price * 100).toFixed(0));
        return value;
      });
      return item;
    });
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
    let categoryId = 0;
    if (typeof values.category_ids === 'number') {
      // 单分类
      categoryId = Number(values.category_ids);
    } else {
      for (let i in values.category_ids) {
        if (values.category_ids[i] > 0) {
          categoryIds.push(values.category_ids[i]);
        }
      }
      if (categoryIds.length > 0) {
        categoryId = categoryIds[0];
      }
    }
    if (categoryId === 0) {
      this.loading = false;
      message.error(
        this.props.intl.formatMessage({ id: 'content.category.required' }),
      );
      return;
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
      removeStore('unsaveProduct');
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

  handleUpdateProfit = (price: number | null, costPrice: number | null) => {
    if (price && price > 0 && costPrice && costPrice > 0) {
      const profit = price - costPrice;
      const profitPercent = (profit / price) * 100;
      this.formRef.current?.setFieldsValue({
        profit: profit,
        profit_percent: profitPercent,
      });
    } else {
      this.formRef.current?.setFieldsValue({
        profit: null,
        profit_percent: null,
      });
    }
  };

  handleChangeOptionType = (value: number) => {
    const { archive, options } = this.state;
    archive.option_type = value;
    this.setState({
      [`archive.option_type`]: value,
    });
    if (value !== OptionTypeSingle && options.length === 0) {
      this.handleAddOption();
    }
    this.buildSku(options, archive.option_type);
  };

  handleAddOption = () => {
    const { options } = this.state;
    options.push({
      title: '',
      values: [],
    });
    this.setState({
      options,
    });
  };

  handleAddOptionValue = (index: number) => {
    const { options } = this.state;
    if (typeof options[index].values === 'undefined') {
      options[index].values = [];
    }
    options[index].values.push({
      title: '',
      price: 0,
      color: '',
    });
    this.setState({
      options,
    });
  };

  handleChangeOption = (index: number, key: string, value: any) => {
    const { options } = this.state;
    options[index][key] = value;
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  onMoveUpOption = (index: number) => {
    const { options } = this.state;
    if (index <= 0) {
      return;
    }
    const temp = options[index];
    options[index] = options[index - 1];
    options[index - 1] = temp;

    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  onMoveDownOption = (index: number) => {
    const { options } = this.state;
    if (index >= options.length - 1) {
      return;
    }
    const temp = options[index];
    options[index] = options[index + 1];
    options[index + 1] = temp;
    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  onRemoveOption = (index: number) => {
    const { options } = this.state;
    options.splice(index, 1);
    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  handleChangeOptionValue = (
    index: number,
    idx: number,
    field: string,
    value: any,
  ) => {
    const { options } = this.state;
    options[index].values[idx][field] = value;
    this.setState({
      options,
    });
    if (field === 'is_default') {
      if (value === true) {
        options[index].values.forEach((item: any, idx2: number) => {
          if (idx !== idx2) {
            item.is_default = false;
          }
        });
      } else {
        options[index].values[0] = true;
      }
      this.setState({
        options,
      });
      this.formRef?.current?.setFieldsValue({ options });
    } else {
      this.buildSku(options, this.state.archive.option_type);
    }
  };

  onMoveUpOptionValue = (index: number, idx: number) => {
    const { options } = this.state;
    if (idx <= 0) {
      return;
    }
    const temp = options[index].values[idx];
    options[index].values[idx] = options[index].values[idx - 1];
    options[index].values[idx - 1] = temp;
    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  onMoveDownOptionValue = (index: number, idx: number) => {
    const { options } = this.state;
    if (idx >= options[index].values.length - 1) {
      return;
    }
    const temp = options[index].values[idx];
    options[index].values[idx] = options[index].values[idx + 1];
    options[index].values[idx + 1] = temp;
    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  onRemoveOptionValue = (index: number, idx: number) => {
    const { options } = this.state;
    options[index].values.splice(idx, 1);
    this.formRef?.current?.setFieldsValue({ options });
    this.setState({
      options,
    });
    this.buildSku(options, this.state.archive.option_type);
  };

  buildSku = (options: any[], optionType: number) => {
    /**
     * optionType = multi|addon, multi 是多规格，addon 是多规格加价
     * options = [
     *   {title: '颜色', is_attribute: true, values: [{title: '黑色', logo: '', color: '', price: 100}, {title: '白色', price: 100}]},
     *   {title: '内存', is_attribute: false, values: [{title: '128G', logo: '', price: 100}, {title: '256G', price: 100}]}，
     *   {title: '版本', is_attribute: false, values: [{title: '标准版', logo: '', price: 100}, {title: '专业版', price: 100}]}
     * ]
     */
    const skus: any = [];
    if (options.length === 0) {
      this.setState({
        skus,
      });
      return;
    }
    // 多属性加价不需要生成sku
    if (optionType === OptionTypeAddon) {
      this.setState({
        skus,
      });
      return;
    }

    // 递归构建SKU组合
    const buildSkuRecursive = (
      options: any[],
      index: number,
      current: any[],
    ) => {
      if (index >= options.length) {
        // 创建SKU项目
        const titles = current.map((item) => item.title);
        const skuItem = {
          title: titles.join(' '),
          price: 0,
          stock: null,
          values: [...current],
        };

        // 计算价格（将所有选项的价格相加）
        if (optionType === OptionTypeAddon) {
          // addon 模式下，只有第一个选项是主选项，其他选项是加价
          skuItem.price = current[0]?.price || 0;
          for (let i = 1; i < current.length; i++) {
            skuItem.price += current[i]?.price || 0;
          }
        } else {
          // multi 模式下，所有选项价格相加
          skuItem.price = current.reduce(
            (sum, item) => sum + (item.price || 0),
            0,
          );
        }

        skus.push(skuItem);
        return;
      }

      // 遍历当前选项的所有值
      if (!options[index].is_attribute && options[index].values?.length > 0) {
        for (let i = 0; i < options[index].values.length; i++) {
          const newValue = [];
          for (let j = 0; j < current.length; j++) {
            newValue.push({ ...current[j] });
          }
          if (options[index].values[i].title) {
            newValue.push({
              valueSize: options[index].values.length,
              rowSpan: 1,
              show: true,
              ...options[index].values[i],
              option_title: options[index].title,
            });
          }
          buildSkuRecursive(options, index + 1, newValue);
        }
      } else {
        buildSkuRecursive(options, index + 1, current);
      }
    };

    // 从第一个选项开始构建
    buildSkuRecursive(options, 0, []);

    // 构建 originSkus 的映射表，以便通过组合键快速查找
    const originSkuMap = new Map<string, any>();
    if (this.state.originSkus && this.state.originSkus.length > 0) {
      this.state.originSkus.forEach((originSku: any) => {
        if (originSku.values && originSku.values.length > 0) {
          // 使用选项标题组合作为唯一键
          const key = originSku.values.map((v: any) => v.title).join('||');
          originSkuMap.set(key, originSku);
        }
      });
    }

    // 计算每个SKU选项的rowSpan值并合并原有数据
    for (let k = 0; k < skus.length; k++) {
      const sku = skus[k];
      const values = sku.values || [];

      // 生成当前SKU的唯一键
      const currentKey = values.map((v: any) => v.title).join('||');
      const originSku = originSkuMap.get(currentKey);

      // 如果存在原有的SKU数据，则进行合并保留
      if (originSku) {
        skus[k] = Object.assign({}, originSku, skus[k]);
        // 确保某些关键字段不被新计算的默认值覆盖（如果原值存在）
        if (originSku.id) {
          skus[k].id = originSku.id;
        }
        if (originSku.sku && !skus[k].sku) {
          skus[k].sku = originSku.sku;
        }
        // 注意：价格、库存等如果用户之前手动修改过，应该保留原值，除非新计算的值有特定业务逻辑要求覆盖
        // 这里通常保留原值，因为用户可能已经设置了特定SKU的价格/库存
        if (originSku.price !== undefined && originSku.price !== null) {
          // 如果是重新生成，通常希望保留用户设置的特定价格，而不是重置为选项价格和
          // 但如果需求是每次生成都重算价格，则注释掉下面这行。
          // 根据常见电商逻辑，修改规格后，通常保留已设置的价格/库存，或者重置。
          // 此处假设保留原有设置更合理，若需强制重算价格可删除此块。
          skus[k].price = originSku.price;
        }
        if (originSku.stock !== undefined && originSku.stock !== null) {
          skus[k].stock = originSku.stock;
        }
        if (originSku.images) {
          skus[k].images = originSku.images;
        }
        if (originSku.barcode) {
          skus[k].barcode = originSku.barcode;
        }
        if (originSku.hs_code) {
          skus[k].hs_code = originSku.hs_code;
        }
        if (originSku.origin_price) {
          skus[k].origin_price = originSku.origin_price;
        }
        if (originSku.cost_price) {
          skus[k].cost_price = originSku.cost_price;
        }
        if (originSku.weight) {
          skus[k].weight = originSku.weight;
        }
      } else {
        // 如果是新增的SKU，且之前有skus状态，尝试保留之前的状态（非originSkus，可能是临时编辑未保存的）
        // 但这部分逻辑较复杂，通常以originSkus为准。
        // 如果 originSkuMap 中找不到，说明是新组合，使用默认值（已在 buildSkuRecursive 中初始化）
        skus[k].status = 1;
      }

      const prevOpts = skus[k - 1]?.values || [];
      for (let i = 0; i < values.length; i++) {
        if (k > 0 && values[i].title === prevOpts[i]?.title) {
          values[i].show = false;
        }
        values[i].rowSpan = 1;
        for (let j = i + 1; j < values.length; j++) {
          values[i].rowSpan *= values[j].valueSize;
        }
      }
    }

    this.formRef?.current?.setFieldsValue({ skus });
    this.setState({
      skus: skus,
    });
  };

  handleChangeSkuValue = (index: number, field: string, value: any) => {
    const { skus } = this.state;
    skus[index][field] = value;
    this.setState({
      skus,
    });
  };

  onSkuMenuClick = (e: any) => {
    if (e.key === 'price') {
      // 批量设置价格
      let fieldValue = 0.0;
      Modal.confirm({
        icon: null,
        title: this.props.intl.formatMessage({
          id: 'content.product.batch-price',
        }),
        content: (
          <ProFormDigit
            fieldProps={{
              precision: 2,
              onChange: (value: any) => {
                fieldValue = value;
              },
            }}
          />
        ),
        onOk: () => {
          // 批量设置价格
          const { skus } = this.state;
          for (let i = 0; i < skus.length; i++) {
            skus[i].price = fieldValue;
          }
          this.formRef?.current?.setFieldsValue({ skus });
          this.setState({
            skus,
          });
        },
      });
    } else if (e.key === 'stock') {
      // 批量设置库存
      let fieldValue = 0;
      Modal.confirm({
        icon: null,
        title: this.props.intl.formatMessage({
          id: 'content.product.batch-stock',
        }),
        content: (
          <ProFormDigit
            fieldProps={{
              precision: 0,
              onChange: (value: any) => {
                fieldValue = value;
              },
            }}
          />
        ),
        onOk: () => {
          // 批量设置价格
          const { skus } = this.state;
          for (let i = 0; i < skus.length; i++) {
            skus[i].stock = fieldValue;
          }
          this.formRef?.current?.setFieldsValue({ skus });
          this.setState({
            skus,
          });
        },
      });
    } else if (e.key === 'fields') {
      this.setState({
        skuFieldsVisible: true,
      });
    }
  };

  handleUpdateSkuActive = (checked: boolean) => {
    const { skus } = this.state;
    if (checked) {
      for (let i = 0; i < skus.length; i++) {
        skus[i].status = 1;
      }
    } else {
      for (let i = 0; i < skus.length; i++) {
        skus[i].status = 0;
      }
    }
    this.formRef?.current?.setFieldsValue({ skus });
    this.setState({
      skus,
    });
  };

  onChangeWholesaleValue = (idx: number, field: string, values: any) => {
    const { wholesaleRules } = this.state;
    wholesaleRules[idx][field] = values;
    this.setState({
      wholesaleRules,
    });
  };

  handleAddWholesaleRule = () => {
    const { wholesaleRules } = this.state;
    wholesaleRules.push({
      id: 0,
      range: [null, null],
      discount_rate: 100,
    });
    this.setState({
      wholesaleRules,
    });
  };

  handleRemoveWholesaleRule = (index: number) => {
    const { wholesaleRules } = this.state;
    wholesaleRules.splice(index, 1);
    this.setState({
      wholesaleRules,
    });
  };

  handleManageOrderFields = () => {
    this.setState({
      showOrderFields: true,
    });
  };

  handleSkuFieldsFinish = async (values: any) => {
    const { skuFields } = this.state;
    const fieldValues = values.sku_fields || [];
    for (let i = 0; i < fieldValues.length; i++) {
      const index = skuFields.findIndex(
        (item: any) => item.value === fieldValues[i],
      );
      if (index >= 0) {
        skuFields[index].checked = true;
      }
    }
    setStore('product-sku-fields', fieldValues);
    this.setState({
      skuFields: [...skuFields],
      skuFieldsVisible: false,
    });
  };

  handleRemove = (id: number) => {
    Modal.confirm({
      title: this.props.intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          this.props.intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        deleteArchive({
          id: id,
        })
          .then(() => {
            message.success(
              this.props.intl.formatMessage({ id: 'content.delete.success' }),
            );
          })
          .catch(() => {
            message.error(
              this.props.intl.formatMessage({ id: 'content.delete.failure' }),
            );
          })
          .finally(() => {
            hide();
          });
      },
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
      options,
      skus,
      wholesaleRules,
      newKey,
      showOrderFields,
      skuFieldsVisible,
      skuFields,
      shippingSettings,
    } = this.state;

    const skuMenu: MenuProps = {
      onClick: (e) => {
        this.onSkuMenuClick(e);
      },
      items: [
        {
          key: 'price',
          label: this.props.intl.formatMessage({
            id: 'content.product.batch-price',
          }),
        },
        {
          key: 'stock',
          label: this.props.intl.formatMessage({
            id: 'content.product.batch-stock',
          }),
        },
        {
          key: 'fields',
          label: this.props.intl.formatMessage({
            id: 'content.product.sku-fields',
          }),
        },
      ],
    };

    return (
      <NewContainer
        title={
          <div>
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
              <FormattedMessage id="content.product.add" />
            )}
          </div>
        }
        extra={
          archive.id > 0 ? (
            <Space size={8}>
              <Button
                onClick={() => {
                  history.push('/product/detail?copyid=' + archive.id);
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
                    label={
                      module.title_name ||
                      this.props.intl.formatMessage({
                        id: 'content.title.name',
                      })
                    }
                  />
                  <ProFormRadio.Group
                    name="product_type"
                    label={this.props.intl.formatMessage({
                      id: 'content.product.type',
                    })}
                    options={[
                      {
                        value: 1,
                        label: this.props.intl.formatMessage({
                          id: 'content.product.type.normal',
                        }),
                      },
                      {
                        value: 2,
                        label: this.props.intl.formatMessage({
                          id: 'content.product.type.wholesale',
                        }),
                      },
                      {
                        value: 3,
                        label: this.props.intl.formatMessage({
                          id: 'content.product.type.custom',
                        }),
                      },
                    ]}
                    fieldProps={{
                      onChange: this.onChangeProductType,
                    }}
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
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.media.name',
                    })}
                  >
                    <div className="media-flex">
                      <div className="video-item">
                        {archive.video_url?.length ? (
                          <div className="ant-upload-item">
                            <a
                              href={archive.video_url}
                              target="_blank"
                              rel="noreferrer"
                            >
                              <Avatar className="default-img" size={216}>
                                {archive.video_url.substring(
                                  archive.video_url.lastIndexOf('.'),
                                )}
                              </Avatar>
                            </a>
                            <span
                              className="delete"
                              onClick={this.handleCleanVideo.bind(this)}
                            >
                              <DeleteOutlined />
                            </span>
                          </div>
                        ) : (
                          <AttachmentSelect
                            onSelect={this.handleSelectVideo}
                            open={false}
                            multiple={false}
                          >
                            <div className="ant-upload-item">
                              <div className="add">
                                <PlusOutlined />
                                <div style={{ marginTop: 8 }}>
                                  <FormattedMessage id="content.media.video.add" />
                                </div>
                              </div>
                            </div>
                          </AttachmentSelect>
                        )}
                      </div>
                      <div className="images-item">
                        <ProFormText>
                          {archive.images?.length
                            ? archive.images.map(
                                (item: string, index: number) => (
                                  <div className="ant-upload-item" key={index}>
                                    <Image
                                      preview={{
                                        src: item,
                                      }}
                                      src={item}
                                    />
                                    <span
                                      className="delete"
                                      onClick={this.handleCleanLogo.bind(
                                        this,
                                        index,
                                      )}
                                    >
                                      <DeleteOutlined />
                                    </span>
                                  </div>
                                ),
                              )
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
                                  <FormattedMessage id="content.media.image.add" />
                                </div>
                              </div>
                            </div>
                          </AttachmentSelect>
                        </ProFormText>
                      </div>
                    </div>
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.product.price',
                    })}
                  >
                    <Row gutter={[16, 16]}>
                      <Col span={12}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.price.name',
                          })}
                          name="price"
                          fieldProps={{
                            precision: 2,
                            onChange: (value) => {
                              const costPrice =
                                this.formRef.current?.getFieldValue(
                                  'cost_price',
                                );
                              this.handleUpdateProfit(value, costPrice);
                            },
                          }}
                        />
                      </Col>
                      <Col span={12}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.origin_price.name',
                          })}
                          name="origin_price"
                          fieldProps={{
                            precision: 2,
                          }}
                          extra={this.props.intl.formatMessage({
                            id: 'content.origin_price.description',
                          })}
                        />
                      </Col>
                      <Col span={8}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.cost_price.name',
                          })}
                          name="cost_price"
                          fieldProps={{
                            precision: 2,
                            onChange: (value) => {
                              const price =
                                this.formRef.current?.getFieldValue('price');
                              this.handleUpdateProfit(price, value);
                            },
                          }}
                        />
                      </Col>
                      <Col span={8}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.profit.name',
                          })}
                          name="profit"
                          fieldProps={{
                            precision: 2,
                            readOnly: true,
                          }}
                        />
                      </Col>
                      <Col span={8}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.profit_percent.name',
                          })}
                          name="profit_percent"
                          fieldProps={{
                            precision: 0,
                            addonAfter: '%',
                            readOnly: true,
                          }}
                        />
                      </Col>
                    </Row>
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.product.stock',
                    })}
                  >
                    <Row gutter={[16, 16]}>
                      <Col sm={12} xs={24}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.product.stock',
                          })}
                          name="stock"
                        />
                      </Col>
                      <Col sm={12} xs={24}>
                        <ProFormText
                          label={this.props.intl.formatMessage({
                            id: 'content.product.oversold',
                          })}
                        >
                          <Space size={16}>
                            <ProFormCheckbox name={'allow_oversold'}>
                              <FormattedMessage id="content.product.allow-oversold" />
                            </ProFormCheckbox>
                          </Space>
                        </ProFormText>
                      </Col>
                      <Col sm={24} xs={24}>
                        <ProFormText
                          label={this.props.intl.formatMessage({
                            id: 'content.product.user-limit',
                          })}
                          extra={this.props.intl.formatMessage({
                            id: 'content.product.user-limit.description',
                          })}
                        >
                          <Space size={16} style={{ width: '100%' }}>
                            <ProFormDigit
                              label={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.min-quantity',
                              })}
                              placeholder={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.description',
                              })}
                              name="min_quantity"
                            />
                            <ProFormDigit
                              label={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.max-quantity',
                              })}
                              placeholder={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.description',
                              })}
                              name="max_quantity"
                            />
                            <ProFormDigit
                              label={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.step-quantity',
                              })}
                              placeholder={this.props.intl.formatMessage({
                                id: 'content.product.user-limit.step-quantity.placeholder',
                              })}
                              name="step_quantity"
                            />
                          </Space>
                        </ProFormText>
                      </Col>
                    </Row>
                  </Card>
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.product.shipping',
                    })}
                  >
                    <ProFormText
                      label={this.props.intl.formatMessage({
                        id: 'content.product.logistics',
                      })}
                    >
                      <Space size={16}>
                        <ProFormCheckbox name={'need_logistics'}>
                          <FormattedMessage id="content.product.need-logistics" />
                        </ProFormCheckbox>
                        <ProFormCheckbox name={'is_free_shipping'}>
                          <FormattedMessage id="content.product.free-shipping" />
                        </ProFormCheckbox>
                      </Space>
                    </ProFormText>
                    <ProFormText>
                      <ProFormGroup size={8}>
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.product.weight',
                          })}
                          name="weight"
                          fieldProps={{
                            precision: 1,
                          }}
                        />
                        <ProFormSelect
                          name="weight_unit"
                          options={shippingSettings.weight_units?.map(
                            (item: any) => ({
                              label: item.name,
                              value: item.name,
                            }),
                          )}
                        />
                      </ProFormGroup>
                    </ProFormText>
                    <ProFormText
                      label={this.props.intl.formatMessage({
                        id: 'content.product.size',
                      })}
                    >
                      <ProFormGroup size={8}>
                        <ProFormDigit
                          addonBefore={this.props.intl.formatMessage({
                            id: 'content.product.size.length',
                          })}
                          width={100}
                          name="length"
                          fieldProps={{
                            precision: 1,
                          }}
                        />
                        <ProFormDigit
                          addonBefore={this.props.intl.formatMessage({
                            id: 'content.product.size.width',
                          })}
                          width={100}
                          name="width"
                          fieldProps={{
                            precision: 1,
                          }}
                        />
                        <ProFormDigit
                          addonBefore={this.props.intl.formatMessage({
                            id: 'content.product.size.height',
                          })}
                          width={100}
                          name="height"
                          fieldProps={{
                            precision: 1,
                          }}
                        />
                        <ProFormSelect
                          name="length_unit"
                          options={shippingSettings.units?.map((item: any) => ({
                            label: item.name,
                            value: item.name,
                          }))}
                        />
                        <ProFormDigit
                          label={this.props.intl.formatMessage({
                            id: 'content.product.size.or',
                          })}
                          addonBefore={this.props.intl.formatMessage({
                            id: 'content.product.size.volume',
                          })}
                          addonAfter="m³"
                          name="volume"
                          fieldProps={{}}
                        />
                      </ProFormGroup>
                    </ProFormText>
                  </Card>
                  {archive.product_type === 2 && (
                    <Card
                      className="aside-card"
                      size="small"
                      title={this.props.intl.formatMessage({
                        id: 'content.product.wholesale',
                      })}
                    >
                      <ProFormGroup size={8}>
                        <div className="wholesale-group">
                          {wholesaleRules.map((item: any, idx: number) => (
                            <div className="wholesale-item" key={idx}>
                              <ProFormDigitRange
                                label={
                                  this.props.intl.formatMessage({
                                    id: 'content.product.wholesale.range',
                                  }) + ` ${idx + 1}`
                                }
                                name={['wholesale_rules', idx, 'range']}
                                fieldProps={{
                                  value: item.range,
                                  onChange: (value) => {
                                    this.onChangeWholesaleValue(
                                      idx,
                                      'range',
                                      value,
                                    );
                                  },
                                }}
                                separator="-"
                                placeholder={['Min', 'Max']}
                                separatorWidth={50}
                              />
                              <ProFormDigit
                                label={this.props.intl.formatMessage({
                                  id: 'content.product.wholesale.discount',
                                })}
                                name="discount_rate"
                                fieldProps={{
                                  suffix: '%',
                                  value: item.discount_rate,
                                  onChange: (value) => {
                                    this.onChangeWholesaleValue(
                                      idx,
                                      'discount_rate',
                                      value,
                                    );
                                  },
                                }}
                                placeholder={this.props.intl.formatMessage({
                                  id: 'content.product.wholesale.discount.tips',
                                })}
                              />
                              <Tag
                                className="wholesale-remove"
                                color="red"
                                onClick={() =>
                                  this.handleRemoveWholesaleRule(idx)
                                }
                              >
                                <DeleteOutlined />
                              </Tag>
                            </div>
                          ))}

                          <div className="wholesale-item wholesale-item-add">
                            <Button onClick={this.handleAddWholesaleRule}>
                              <FormattedMessage id="content.product.wholesale.add" />
                            </Button>
                          </div>
                        </div>
                      </ProFormGroup>
                    </Card>
                  )}
                  {archive.product_type === 3 && (
                    <Card
                      className="aside-card"
                      size="small"
                      title={this.props.intl.formatMessage({
                        id: 'content.product.order-fields',
                      })}
                    >
                      <Button onClick={() => this.handleManageOrderFields()}>
                        <FormattedMessage id="content.product.order-fields.manage" />
                        {archive.order_fields?.length > 0 && (
                          <span>({archive.order_fields?.length})</span>
                        )}
                      </Button>
                    </Card>
                  )}
                  <Card
                    className="aside-card"
                    size="small"
                    title={this.props.intl.formatMessage({
                      id: 'content.product.option.type',
                    })}
                  >
                    <ProFormRadio.Group
                      name="option_type"
                      label={this.props.intl.formatMessage({
                        id: 'content.product.option.type',
                      })}
                      options={[
                        {
                          label: this.props.intl.formatMessage({
                            id: 'content.product.option.type.single',
                          }),
                          value: OptionTypeSingle,
                        },
                        {
                          label: this.props.intl.formatMessage({
                            id: 'content.product.option.type.multiple',
                          }),
                          value: OptionTypeMulti,
                        },
                        {
                          label: this.props.intl.formatMessage({
                            id: 'content.product.option.type.addon',
                          }),
                          value: OptionTypeAddon,
                        },
                      ]}
                      fieldProps={{
                        onChange: (e) => {
                          this.handleChangeOptionType(e.target.value);
                        },
                      }}
                    />
                    {(archive.option_type === OptionTypeMulti ||
                      archive.option_type === OptionTypeAddon) && (
                      <>
                        <Card
                          bordered={false}
                          className="aside-card"
                          size="small"
                          title={this.props.intl.formatMessage({
                            id: 'content.product.option.type.multiple',
                          })}
                          extra={
                            <Button
                              type="link"
                              size="small"
                              onClick={() => this.handleAddOption()}
                            >
                              <FormattedMessage id="content.product.option.type.add" />
                            </Button>
                          }
                        >
                          <div className="options">
                            {options &&
                              options.map((item: any, index: number) => (
                                <div className="option-item" key={index}>
                                  <Flex
                                    gap={16}
                                    align="center"
                                    className="mb-normal"
                                  >
                                    <div className="flex-1">
                                      <ProFormText
                                        name={['options', index, 'title']}
                                        label={this.props.intl.formatMessage({
                                          id: 'content.product.option.type.name',
                                        })}
                                        rules={[
                                          {
                                            required: true,
                                            message:
                                              this.props.intl.formatMessage({
                                                id: 'content.product.option.type.placeholder',
                                              }),
                                          },
                                        ]}
                                        fieldProps={{
                                          onChange: (e) => {
                                            this.handleChangeOption(
                                              index,
                                              'title',
                                              e.target.value,
                                            );
                                          },
                                          suffix: (
                                            <div className="text-action">
                                              <Tag
                                                onClick={() =>
                                                  this.onMoveUpOption(index)
                                                }
                                              >
                                                <UpOutlined />
                                              </Tag>
                                              <Tag
                                                onClick={() =>
                                                  this.onMoveDownOption(index)
                                                }
                                              >
                                                <DownOutlined />
                                              </Tag>
                                              <Tag
                                                color="red"
                                                onClick={() =>
                                                  this.onRemoveOption(index)
                                                }
                                              >
                                                <DeleteOutlined />
                                              </Tag>
                                            </div>
                                          ),
                                        }}
                                      />
                                    </div>
                                    <ProFormCheckbox
                                      name={['options', index, 'is_attribute']}
                                      fieldProps={{
                                        onChange: (e) => {
                                          this.handleChangeOption(
                                            index,
                                            'is_attribute',
                                            e.target.checked,
                                          );
                                        },
                                      }}
                                    >
                                      {this.props.intl.formatMessage({
                                        id: 'content.product.option.is_attribute',
                                      })}{' '}
                                      <Tooltip
                                        title={this.props.intl.formatMessage({
                                          id: 'content.product.option.is_attribute.description',
                                        })}
                                      >
                                        <QuestionCircleOutlined />
                                      </Tooltip>
                                    </ProFormCheckbox>
                                  </Flex>
                                  <div className="option-values">
                                    <ProFormText
                                      label={this.props.intl.formatMessage({
                                        id: 'content.product.option.value',
                                      })}
                                    >
                                      {item.values?.map(
                                        (val: any, idx: number) => (
                                          <Flex
                                            key={idx}
                                            gap={16}
                                            align="center"
                                            className="mb-normal"
                                          >
                                            <ProFormRadio
                                              name={[
                                                'options',
                                                index,
                                                'values',
                                                idx,
                                                'is_default',
                                              ]}
                                              rules={[
                                                {
                                                  required: false,
                                                },
                                              ]}
                                              placeholder={this.props.intl.formatMessage(
                                                {
                                                  id: 'content.product.option.value.placeholder',
                                                },
                                              )}
                                              fieldProps={{
                                                onChange: (e) => {
                                                  this.handleChangeOptionValue(
                                                    index,
                                                    idx,
                                                    'is_default',
                                                    e.target.checked,
                                                  );
                                                },
                                              }}
                                            />
                                            <div className="flex-1">
                                              <ProFormText
                                                name={[
                                                  'options',
                                                  index,
                                                  'values',
                                                  idx,
                                                  'title',
                                                ]}
                                                rules={[
                                                  {
                                                    required: true,
                                                    message:
                                                      this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.product.option.value.placeholder',
                                                        },
                                                      ),
                                                  },
                                                ]}
                                                fieldProps={{
                                                  prefix:
                                                    this.props.intl.formatMessage(
                                                      {
                                                        id: 'content.product.option.value.name',
                                                      },
                                                    ),
                                                  onChange: (e) => {
                                                    this.handleChangeOptionValue(
                                                      index,
                                                      idx,
                                                      'title',
                                                      e.target.value,
                                                    );
                                                  },
                                                }}
                                              />
                                            </div>
                                            {val.images && (
                                              <div>
                                                {val.images.map(
                                                  (iimg: any, iidx: number) => (
                                                    <div
                                                      className="ant-upload-item sku-logo-item"
                                                      key={iidx}
                                                    >
                                                      <Image
                                                        preview={{
                                                          src: iimg,
                                                        }}
                                                        src={iimg}
                                                      />
                                                      <span
                                                        className="delete"
                                                        onClick={() => {
                                                          let newVal =
                                                            val.images.filter(
                                                              (img: any) =>
                                                                img !== iimg,
                                                            );
                                                          this.handleChangeOptionValue(
                                                            index,
                                                            idx,
                                                            'images',
                                                            newVal,
                                                          );
                                                        }}
                                                      >
                                                        <DeleteOutlined />
                                                      </span>
                                                    </div>
                                                  ),
                                                )}
                                              </div>
                                            )}
                                            <AttachmentSelect
                                              onSelect={(rows) => {
                                                let logoValues =
                                                  val.images || [];
                                                for (
                                                  let i = 0;
                                                  i < rows.length;
                                                  i++
                                                ) {
                                                  if (
                                                    logoValues.includes(
                                                      rows[i].file_path,
                                                    )
                                                  ) {
                                                    continue;
                                                  }
                                                  logoValues.push(
                                                    rows[i].file_path,
                                                  );
                                                }
                                                this.handleChangeOptionValue(
                                                  index,
                                                  idx,
                                                  'images',
                                                  logoValues,
                                                );
                                              }}
                                              multiple={true}
                                              open={false}
                                            >
                                              <div className="ant-upload-item sku-logo-item">
                                                <div className="add">
                                                  <PlusOutlined />
                                                </div>
                                              </div>
                                            </AttachmentSelect>
                                            <ColorPicker
                                              disabledAlpha
                                              defaultValue={val.color}
                                              allowClear
                                              onChangeComplete={(e) => {
                                                this.handleChangeOptionValue(
                                                  index,
                                                  idx,
                                                  'color',
                                                  e.toHexString(),
                                                );
                                              }}
                                            />
                                            {archive.option_type ===
                                              OptionTypeAddon && (
                                              <>
                                                <ProFormDigit
                                                  name={[
                                                    'options',
                                                    index,
                                                    'values',
                                                    idx,
                                                    'price',
                                                  ]}
                                                  rules={[
                                                    {
                                                      required: false,
                                                      message:
                                                        this.props.intl.formatMessage(
                                                          {
                                                            id: 'content.product.option.value.price.placeholder',
                                                          },
                                                        ),
                                                    },
                                                  ]}
                                                  fieldProps={{
                                                    prefix:
                                                      this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.product.option.value.price',
                                                        },
                                                      ),
                                                    precision: 2,
                                                    onChange: (e) => {
                                                      this.handleChangeOptionValue(
                                                        index,
                                                        idx,
                                                        'price',
                                                        e,
                                                      );
                                                    },
                                                  }}
                                                />
                                                <ProFormDigit
                                                  name={[
                                                    'options',
                                                    index,
                                                    'values',
                                                    idx,
                                                    'stock',
                                                  ]}
                                                  rules={[
                                                    {
                                                      required: false,
                                                      message:
                                                        this.props.intl.formatMessage(
                                                          {
                                                            id: 'content.product.option.sku.stock.placeholder',
                                                          },
                                                        ),
                                                    },
                                                  ]}
                                                  fieldProps={{
                                                    prefix:
                                                      this.props.intl.formatMessage(
                                                        {
                                                          id: 'content.product.option.sku.stock',
                                                        },
                                                      ),
                                                    precision: 0,
                                                    onChange: (e) => {
                                                      this.handleChangeOptionValue(
                                                        index,
                                                        idx,
                                                        'stock',
                                                        e,
                                                      );
                                                    },
                                                  }}
                                                />
                                              </>
                                            )}
                                            <div className="text-action">
                                              <Tag
                                                onClick={() =>
                                                  this.onMoveUpOptionValue(
                                                    index,
                                                    idx,
                                                  )
                                                }
                                              >
                                                <UpOutlined />
                                              </Tag>
                                              <Tag
                                                onClick={() =>
                                                  this.onMoveDownOptionValue(
                                                    index,
                                                    idx,
                                                  )
                                                }
                                              >
                                                <DownOutlined />
                                              </Tag>
                                              <Tag
                                                color="red"
                                                onClick={() =>
                                                  this.onRemoveOptionValue(
                                                    index,
                                                    idx,
                                                  )
                                                }
                                              >
                                                <DeleteOutlined />
                                              </Tag>
                                            </div>
                                          </Flex>
                                        ),
                                      )}
                                      <Button
                                        style={{ width: '100%' }}
                                        onClick={() =>
                                          this.handleAddOptionValue(index)
                                        }
                                      >
                                        <FormattedMessage id="content.product.option.value.add" />
                                      </Button>
                                    </ProFormText>
                                  </div>
                                </div>
                              ))}
                          </div>
                        </Card>
                        {archive.option_type === OptionTypeMulti && (
                          <Card
                            bordered={false}
                            className="aside-card"
                            size="small"
                            title={this.props.intl.formatMessage({
                              id: 'content.product.option.skus',
                            })}
                            extra={
                              <Dropdown menu={skuMenu}>
                                <a>
                                  <FormattedMessage id="content.product.option.skus.extra" />
                                </a>
                              </Dropdown>
                            }
                          >
                            <div className="ant-table">
                              <div className="ant-table-container">
                                <div className="ant-table-content">
                                  <table className="archive-skus">
                                    <colgroup>
                                      {options.map(
                                        (item: any, index: number) => (
                                          <col key={index} />
                                        ),
                                      )}
                                      {skuFields.map(
                                        (field: any, idx: number) =>
                                          field.checked && <col key={idx} />,
                                      )}
                                      <col />
                                      <col />
                                    </colgroup>
                                    <thead className="ant-table-thead">
                                      <tr>
                                        <th
                                          className="ant-table-cell"
                                          colSpan={options.length || 1}
                                        >
                                          <FormattedMessage id="content.product.option.sku.name" />
                                        </th>
                                        {skuFields.map(
                                          (field: any, idx: number) =>
                                            field.checked && (
                                              <th
                                                className="ant-table-cell"
                                                key={idx}
                                              >
                                                {field.value === 'logo'
                                                  ? this.props.intl.formatMessage(
                                                      {
                                                        id: 'content.product.option.sku.image',
                                                      },
                                                    )
                                                  : field.label}
                                              </th>
                                            ),
                                        )}
                                        <th className="ant-table-cell">
                                          <FormattedMessage id="content.product.option.sku.price" />
                                        </th>
                                        <th className="ant-table-cell">
                                          <FormattedMessage id="content.product.option.sku.stock" />
                                        </th>
                                        <th className="ant-table-cell">
                                          <FormattedMessage id="content.product.option.sku.active" />
                                          <Checkbox
                                            checked={skus?.find(
                                              (sku: any) => sku.status === 1,
                                            )}
                                            indeterminate={
                                              skus?.find(
                                                (sku: any) => sku.status === 1,
                                              ) &&
                                              skus?.find(
                                                (sku: any) => sku.status !== 1,
                                              )
                                            }
                                            onChange={(e) =>
                                              this.handleUpdateSkuActive(
                                                e.target.checked,
                                              )
                                            }
                                          />
                                        </th>
                                      </tr>
                                    </thead>
                                    <tbody className="ant-table-tbody">
                                      {skus &&
                                        skus.map((item: any, index: number) => (
                                          <tr className="sku-item" key={index}>
                                            {index === 0 &&
                                              options.map(
                                                (opt: any, optIdx: number) =>
                                                  opt.is_attribute && (
                                                    <td
                                                      className="sku-item-value sku-item-value-border"
                                                      key={optIdx}
                                                      rowSpan={skus.length}
                                                    >
                                                      {opt.values.map(
                                                        (optVal: any) => (
                                                          <div
                                                            key={optVal.title}
                                                          >
                                                            {optVal.title}
                                                          </div>
                                                        ),
                                                      )}
                                                    </td>
                                                  ),
                                              )}
                                            {item.values.map(
                                              (value: any, idx: number) =>
                                                value.show && (
                                                  <td
                                                    className={
                                                      'sku-item-value ' +
                                                      (idx + 1 <
                                                        item.values.length &&
                                                        'sku-item-value-border')
                                                    }
                                                    key={idx}
                                                    rowSpan={value.rowSpan}
                                                  >
                                                    {value.title}
                                                    {value.color && (
                                                      <span
                                                        className="sku-item-value-color"
                                                        style={{
                                                          background:
                                                            value.color,
                                                        }}
                                                      ></span>
                                                    )}
                                                  </td>
                                                ),
                                            )}
                                            {skuFields.map(
                                              (field: any, fieldId: number) =>
                                                field.checked &&
                                                (field.value === 'logo' ? (
                                                  <td key={fieldId}>
                                                    <Space>
                                                      {item.images && (
                                                        <div>
                                                          {item.images.map(
                                                            (
                                                              iimg: any,
                                                              iidx: number,
                                                            ) => (
                                                              <div
                                                                key={iidx}
                                                                className="ant-upload-item sku-logo-item"
                                                              >
                                                                <Image
                                                                  preview={{
                                                                    src: iimg,
                                                                  }}
                                                                  src={iimg}
                                                                />
                                                                <span
                                                                  className="delete"
                                                                  onClick={() => {
                                                                    let newVal =
                                                                      item.images.filter(
                                                                        (
                                                                          img: any,
                                                                        ) =>
                                                                          img !==
                                                                          iimg,
                                                                      );
                                                                    this.handleChangeSkuValue(
                                                                      index,
                                                                      'images',
                                                                      newVal,
                                                                    );
                                                                  }}
                                                                >
                                                                  <DeleteOutlined />
                                                                </span>
                                                              </div>
                                                            ),
                                                          )}
                                                        </div>
                                                      )}
                                                      <AttachmentSelect
                                                        onSelect={(rows) => {
                                                          let logoValues =
                                                            item.images || [];
                                                          for (
                                                            let i = 0;
                                                            i < rows.length;
                                                            i++
                                                          ) {
                                                            if (
                                                              logoValues.includes(
                                                                rows[i]
                                                                  .file_path,
                                                              )
                                                            ) {
                                                              continue;
                                                            }
                                                            logoValues.push(
                                                              rows[i].file_path,
                                                            );
                                                          }
                                                          this.handleChangeSkuValue(
                                                            index,
                                                            'images',
                                                            logoValues,
                                                          );
                                                        }}
                                                        multiple={true}
                                                        open={false}
                                                      >
                                                        <div className="ant-upload-item sku-logo-item">
                                                          <div className="add">
                                                            <PlusOutlined />
                                                          </div>
                                                        </div>
                                                      </AttachmentSelect>
                                                    </Space>
                                                  </td>
                                                ) : (
                                                  <td key={fieldId}>
                                                    {field.value ===
                                                    'weight' ? (
                                                      <ProFormDigit
                                                        label={this.props.intl.formatMessage(
                                                          {
                                                            id: 'content.product.weight',
                                                          },
                                                        )}
                                                        name={[
                                                          'skus',
                                                          index,
                                                          'weight',
                                                        ]}
                                                        fieldProps={{
                                                          precision: 1,
                                                          onChange: (e) => {
                                                            this.handleChangeSkuValue(
                                                              index,
                                                              field.value,
                                                              e,
                                                            );
                                                          },
                                                        }}
                                                      />
                                                    ) : field.value ===
                                                        'cost_price' ||
                                                      field.value ===
                                                        'origin_price' ? (
                                                      <ProFormDigit
                                                        name={[
                                                          'skus',
                                                          index,
                                                          field.value,
                                                        ]}
                                                        fieldProps={{
                                                          precision: 2,
                                                          onChange: (e) => {
                                                            this.handleChangeSkuValue(
                                                              index,
                                                              field.value,
                                                              e,
                                                            );
                                                          },
                                                        }}
                                                      />
                                                    ) : (
                                                      <ProFormText
                                                        name={[
                                                          'skus',
                                                          index,
                                                          field.value,
                                                        ]}
                                                        fieldProps={{
                                                          onChange: (e) => {
                                                            this.handleChangeSkuValue(
                                                              index,
                                                              field.value,
                                                              e.target.value,
                                                            );
                                                          },
                                                        }}
                                                      />
                                                    )}
                                                  </td>
                                                )),
                                            )}
                                            <td>
                                              <ProFormDigit
                                                name={['skus', index, 'price']}
                                                fieldProps={{
                                                  precision: 2,
                                                  onChange: (e) => {
                                                    this.handleChangeSkuValue(
                                                      index,
                                                      'price',
                                                      e,
                                                    );
                                                  },
                                                }}
                                              />
                                            </td>
                                            <td>
                                              <ProFormDigit
                                                name={['skus', index, 'stock']}
                                                fieldProps={{
                                                  precision: 0,
                                                  onChange: (e) => {
                                                    this.handleChangeSkuValue(
                                                      index,
                                                      'stock',
                                                      e,
                                                    );
                                                  },
                                                }}
                                              />
                                            </td>
                                            <td>
                                              <ProFormCheckbox
                                                name={['skus', index, 'status']}
                                                fieldProps={{
                                                  checked: item.status === 1,
                                                  onChange: (e) => {
                                                    this.handleChangeSkuValue(
                                                      index,
                                                      'status',
                                                      e.target.checked ? 1 : 0,
                                                    );
                                                  },
                                                }}
                                              />
                                            </td>
                                          </tr>
                                        ))}
                                    </tbody>
                                  </table>
                                </div>
                              </div>
                            </div>
                          </Card>
                        )}
                      </>
                    )}
                  </Card>
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
                      name="category_ids"
                      width="lg"
                      mode={
                        contentSetting.multi_category === 1
                          ? 'multiple'
                          : 'single'
                      }
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
                          history.push('/plugin/user');
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
        {showOrderFields && (
          <OrderFieldsForm
            open={showOrderFields}
            onCancel={() => {
              this.setState({
                showOrderFields: false,
              });
            }}
            values={archive.order_fields || []}
            onSubmit={async (values) => {
              archive.order_fields = values;
              this.setState({
                showOrderFields: false,
                archive,
              });
            }}
          />
        )}
        {skuFieldsVisible && (
          <ModalForm
            open={skuFieldsVisible}
            onFinish={this.handleSkuFieldsFinish}
            onOpenChange={(flag) => {
              this.setState({
                skuFieldsVisible: flag,
              });
            }}
          >
            <ProFormCheckbox.Group
              name="sku_fields"
              label={this.props.intl.formatMessage({
                id: 'content.product.sku-fields',
              })}
              initialValue={skuFields.reduce((pre: any, cur: any) => {
                if (cur.checked) {
                  pre.push(cur.value);
                }
                return pre;
              }, [])}
              options={skuFields}
            />
          </ModalForm>
        )}
      </NewContainer>
    );
  }
}

export default injectIntl(ProductForm);
