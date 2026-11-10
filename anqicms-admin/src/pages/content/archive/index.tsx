import NewContainer from '@/components/NewContainer';
import ReplaceKeywords from '@/components/replaceKeywords';
import {
  addTitleToAnchor,
  anqiAiPseudoArchive,
  anqiTranslateArchive,
  deleteArchive,
  getArchives,
  getModules,
  getSettingContent,
  getTags,
  updateArchivesCategory,
  updateArchivesFlag,
  updateArchivesReleasePlan,
  updateArchivesSort,
  updateArchivesStatus,
  updateArchivesTags,
  updateArchivesTime,
} from '@/services';
import { getCategories } from '@/services/category';
import { getRuntimeShopMode } from '@/utils/shopMode';
import { supportLanguages } from '@/utils';
import { getStore } from '@/utils/store';
import { PlusOutlined, StarOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  ProColumnType,
  ProColumns,
  ProFormCheckbox,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Dropdown,
  Input,
  Modal,
  Select,
  Space,
  Tag,
  Tooltip,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import ChildrenArchivesModal from './components/childrenArchives';
import QuickImportModal from './components/quickImport';
import './index.less';
import QuickEditForm from './quickEdit';

let toLanguage = '';
let updating = false;

let lastParams: any = {
  module_id: 0,
  category_id: 0,
  status: 'ok',
  flag: '',
  exact: false,
};

const ArchiveList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [replaceVisible, setReplaceVisible] = useState<boolean>(false);
  const [flagVisible, setFlagVisible] = useState<boolean>(false);
  const [tagVisible, setTagVisible] = useState<boolean>(false);
  const [statusVisible, setStatusVisible] = useState<boolean>(false);
  const [categoryVisible, setCategoryVisible] = useState<boolean>(false);
  const [timeVisible, setTimeVisible] = useState<boolean>(false);
  const [releaseVisible, setReleaseVisible] = useState<boolean>(false);
  const [modules, setModules] = useState<any[]>([]);
  const [moduleId, setModuleId] = useState<number>(0);
  const [contentSetting, setContentSetting] = useState<any>({});
  const [currentArchive, setCurrentArchive] = useState<any>({});
  const [quickVisible, setQuickVisible] = useState<boolean>(false);
  const [firstFetch, setFirstFetch] = useState<boolean>(false);
  const [latestUpdateId, setLatestUpdateId] = useState<number>(0);
  const [importVisible, setImportVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const [childrenOpen, setChildrenOpen] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [searchedTags, setSearchedTags] = useState<any>({});
  const intl = useIntl();

  const flagEnum: any = {
    h: intl.formatMessage({ id: 'content.flag.h' }),
    c: intl.formatMessage({ id: 'content.flag.c' }),
    f: intl.formatMessage({ id: 'content.flag.f' }),
    a: intl.formatMessage({ id: 'content.flag.a' }),
    s: intl.formatMessage({ id: 'content.flag.s' }),
    b: intl.formatMessage({ id: 'content.flag.b' }),
    p: intl.formatMessage({ id: 'content.flag.p' }),
    j: intl.formatMessage({ id: 'content.flag.j' }),
  };

  const loadContentSetting = () => {
    getSettingContent()
      .then((res) => {
        setContentSetting(res?.data || {});
      })
      .catch();
  };

  const loadModules = async () => {
    // 商城模式产品独立到 /product 管理，文档列表排除产品模块（module_id=2）；
    // 普通内容模式下产品仍是内容模型，必须出现在模块切换里。
    const shopMode = getRuntimeShopMode();
    let res = await getModules(shopMode ? { exclude_id: 2 } : {});
    setModules(
      [
        { title: intl.formatMessage({ id: 'content.archive.all' }), id: 0 },
      ].concat(res.data || []),
    );
    getCategories()
      .then((res) => {
        const list = res.data || [];
        setCategories(
          shopMode
            ? list.filter((item: any) => item.module_id !== 2)
            : list,
        );
      })
      .catch();
  };

  const loadLatestUpdate = () => {
    let latestUpdate = getStore('latest_update') || {};
    // 最近7天更新过的文档
    if (
      latestUpdate.timestamp >
      new Date().getTime() / 1000 - 60 * 60 * 24 * 7
    ) {
      setLatestUpdateId(latestUpdate.id || 0);
    }
  };

  const onTabChange = (key: string, isSubSite: boolean) => {
    setModuleId(lastParams.module_id);
    loadModules();
    loadContentSetting();
    loadLatestUpdate();

    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  useEffect(() => {
    setModuleId(lastParams.module_id);
    loadModules();
    loadContentSetting();
    loadLatestUpdate();
  }, []);

  const onSelectModule = (id: number) => {
    lastParams.module_id = id;
    setModuleId(id);
    formRef.current?.setFieldsValue({
      module_id: id,
      category_id: 0,
    });
    history.replace('/archive/list?module_id=' + id);
    formRef.current?.submit();
  };

  const beforeSearch = (searchParams: any) => {
    let params = searchParams;
    if (!firstFetch) {
      setFirstFetch(true);
      const searchParams = new URLSearchParams(window.location.search);
      lastParams.module_id = Number(searchParams.get('module_id') || 0);
      lastParams.category_id = Number(searchParams.get('category_id') || 0);
      lastParams.status = searchParams.get('status') || 'ok';
      formRef.current?.setFieldsValue(lastParams);
      params = lastParams;
    } else {
      lastParams = params;
      if (params.module_id !== moduleId) {
        onSelectModule(params.module_id);
      }
    }

    return params;
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteArchive({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleSetFlag = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesFlag({
      flag: values.flag.join(','),
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setFlagVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reload?.();
      })
      .finally(() => {
        hide();
      });
  };

  const handleSetStatus = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesStatus({
      status: Number(values.status),
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setStatusVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reloadAndRest?.();
      })
      .finally(() => {
        hide();
      });
  };

  const handleSetTime = async (values: any) => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesTime({
      time: Number(values.time),
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setTimeVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reloadAndRest?.();
      })
      .finally(() => {
        hide();
      });
  };

  const handleSetCategory = async (values: any) => {
    let categoryIds = [];
    let categoryId = values.category_id;
    if (categoryId === 0) {
      message.error(intl.formatMessage({ id: 'content.category.required' }));
      return;
    }
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
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesCategory({
      category_id: categoryId,
      category_ids: categoryIds,
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setCategoryVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reload?.();
      })
      .finally(() => {
        hide();
      });
  };

  const handleEditArchive = async (record: any) => {
    history.push('/archive/detail?id=' + record.id);
  };

  const handleCopyArchive = async (record: any) => {
    history.push('/archive/detail?copyid=' + record.id);
  };

  const handleShowChildren = (record: any) => {
    setCurrentArchive(record);
    setChildrenOpen(true);
  };

  const handleTranslate = (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.translate.confirm' }),
      content: (
        <div>
          <Alert
            className="mb-normal"
            message={intl.formatMessage({ id: 'content.translate.tips' })}
          ></Alert>
          <div className="">
            {intl.formatMessage({ id: 'content.translate.select-language' })}
          </div>
          <Select
            style={{ width: '100%' }}
            onChange={(e) => {
              toLanguage = e;
            }}
            options={supportLanguages.map((item) => {
              return {
                label: item.label,
                value: item.value,
              };
            })}
          />
        </div>
      ),
      onOk: async () => {
        if (!selectedRowKeys) return true;
        const hide = message.loading(
          intl.formatMessage({ id: 'content.translate.translating' }),
          0,
        );
        try {
          let msg = '';
          for (let item of selectedRowKeys) {
            const res = await anqiTranslateArchive({
              id: item,
              to_language: toLanguage,
            });
            msg = res.msg;
          }
          hide();
          message.info(msg);
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          return true;
        }
      },
    });
  };

  const handleTranslateArchive = async (record: any) => {
    handleTranslate([record.id]);
  };

  const handleAiPseudo = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.pseudo.confirm' }),
      content: intl.formatMessage({ id: 'content.pseudo.content' }),
      onOk: async () => {
        if (!selectedRowKeys) return true;
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        try {
          let msg = '';
          for (let item of selectedRowKeys) {
            const res = await anqiAiPseudoArchive({
              id: item,
            });
            msg = res.msg;
          }
          hide();
          message.info(msg);
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();

          return true;
        } catch (error) {
          hide();
          return true;
        }
      },
    });
  };
  const handleAiPseudoArchive = async (record: any) => {
    handleAiPseudo([record.id]);
  };

  const parseFlag = (flag: string) => {
    let flags = flag.split(',');
    return (
      <span>
        {flags.map((f) => (
          <Tag key={f}>{flagEnum[f] || f}</Tag>
        ))}
      </span>
    );
  };

  const updateSort = (index: number, record: any, value: any) => {
    if (updating) {
      return;
    }
    let valueNum = parseInt(value);
    if (isNaN(valueNum)) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    if (valueNum === record.sort) {
      return;
    }
    if (valueNum < 0) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    updating = true;
    updateArchivesSort({
      sort: valueNum,
      id: record.id,
    })
      .then((res) => {
        message.success(res.msg);
        actionRef.current?.reload?.();
      })
      .finally(() => {
        updating = false;
      });
  };

  const handleSetReleasePlan = async (values: any) => {
    if (updating) {
      return;
    }
    updating = true;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesReleasePlan({
      daily_limit: Number(values.daily_limit),
      start_hour: Number(values.start_hour),
      end_hour: Number(values.end_hour),
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setReleaseVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reloadAndRest?.();
      })
      .finally(() => {
        hide();
        updating = false;
      });
  };

  const handleQuickEdit = (record: any) => {
    setCurrentArchive(record);
    setQuickVisible(true);
  };

  const getExactCount = () => {
    message.loading(intl.formatMessage({ id: 'content.count.loading' }));
    lastParams.exact = true;
    actionRef.current?.reload?.();
  };

  const handleAddAnchor = (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.option.batch-add-anchor.confirm',
      }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          await addTitleToAnchor({
            type: 'category',
            ids: selectedRowKeys,
          });
          hide();
          message.success(intl.formatMessage({ id: 'content.submit.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.submit.failure' }));
          return true;
        }
      },
    });
  };

  const onChangeTagInput = (e: any) => {
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
      setSearchedTags(result);
    });
  };

  const handleSetTag = async (values: any) => {
    let tags = values.tags;
    if (tags.length === 0) {
      message.error(intl.formatMessage({ id: 'content.tag.required' }));
      return;
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    updateArchivesTags({
      tags: tags,
      ids: selectedRowKeys,
    })
      .then((res) => {
        message.success(res.msg);
        setTagVisible(false);
        setSelectedRowKeys([]);
        actionRef.current?.reload?.();
      })
      .finally(() => {
        hide();
      });
  };

  const sortColumn: ProColumnType = {
    title: intl.formatMessage({ id: 'content.sort.name' }),
    dataIndex: 'sort',
    hideInSearch: true,
    sorter: true,
    tooltip: intl.formatMessage({ id: 'content.sort.tips' }),
    width: 90,
    render: (_, entity: any, index) => {
      return (
        <div>
          <Input
            onBlur={(e: any) => {
              updateSort(index, entity, e.target.value);
            }}
            onPressEnter={(e: any) => {
              updateSort(index, entity, e.target?.value);
            }}
            defaultValue={entity.sort}
          ></Input>
        </div>
      );
    },
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            <a
              href={entity.link + (entity.status !== 1 ? '?preview=true' : '')}
              rel="noopener noreferrer"
              target="_blank"
            >
              {latestUpdateId === entity.id && (
                <Tooltip
                  title={intl.formatMessage({ id: 'content.latest-update' })}
                >
                  <StarOutlined className="update-tag" />
                </Tooltip>
              )}
              {dom} {entity.flag && parseFlag(entity.flag)}
            </a>
          </div>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      width: 70,
      render: (text, record) => {
        return text ? <img src={record.thumb} className="list-thumb" /> : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_id',
      render: (_: any, entity) => {
        return entity.module_name;
      },
      renderFormItem: () => {
        return (
          <ProFormSelect
            name="module_id"
            request={async () => {
              let res = await getModules({});
              const tmpModules = [
                {
                  name: intl.formatMessage({ id: 'content.archive.all' }),
                  id: 0,
                },
              ]
                .concat(res.data || [])
                // 仅商城模式排除产品模块
                .filter((item: any) => !(getRuntimeShopMode() && item.id === 2))
                .map((item: any) => ({
                  label: item.name,
                  value: item.id,
                }));
              return tmpModules;
            }}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.category.name' }),
      dataIndex: 'category_id',
      render: (_: any, entity) => {
        return (
          <div>
            {entity.category_titles?.map((item: string) => (
              <div key={item}>{item}</div>
            ))}
          </div>
        );
      },
      request: async () => {
        let res = await getCategories({ type: 1 });
        const categories = [
          {
            parents: [],
            title: intl.formatMessage({ id: 'content.category.all' }),
            id: 0,
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
        return categories;
      },
      fieldProps: {
        showSearch: true,
        filterOption: (input: string, option: any) =>
          (option?.title ?? option?.label)
            .toLowerCase()
            .includes(input.toLowerCase()),
      },
    },
    {
      title: intl.formatMessage({ id: 'content.views.name' }),
      dataIndex: 'views',
      hideInSearch: true,
      sorter: true,
    },
    {
      title: 'Flag',
      dataIndex: 'flag',
      hideInTable: true,
      renderFormItem: () => {
        return (
          <ProFormSelect
            name="flag"
            valueEnum={Object.assign(
              { '': intl.formatMessage({ id: 'content.unlimit' }) },
              flagEnum,
            )}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.status.draft' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.status.normal' }),
          status: 'Success',
        },
        2: {
          text: intl.formatMessage({ id: 'content.status.plan' }),
          status: 'Warning',
        },
      },
      renderFormItem: () => {
        return (
          <ProFormRadio.Group
            name="status"
            options={[
              {
                label: intl.formatMessage({ id: 'content.status.normal' }),
                value: 'ok',
              },
              {
                label: intl.formatMessage({ id: 'content.status.draft' }),
                value: 'draft',
              },
              {
                label: intl.formatMessage({ id: 'content.status.plan' }),
                value: 'plan',
              },
            ]}
          />
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.create-update-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'created_time',
      render: (_, record) => {
        return (
          <div>
            <div>
              {dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm')}
            </div>
            {record.created_time !== record.updated_time && (
              <div className="update-color">
                {dayjs(record.updated_time * 1000).format('YYYY-MM-DD HH:mm')}
              </div>
            )}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditArchive(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a onClick={() => handleQuickEdit(record)}>
            <FormattedMessage id="content.action.quick-edit" />
          </a>

          <Dropdown
            menu={{
              items: [
                {
                  key: '1',
                  label: (
                    <a
                      onClick={() => {
                        handleTranslateArchive(record);
                      }}
                      title={intl.formatMessage({
                        id: 'content.action.translate.tips',
                      })}
                    >
                      <FormattedMessage id="content.action.translate" />
                    </a>
                  ),
                },
                {
                  key: '2',
                  label: (
                    <a
                      onClick={() => {
                        handleAiPseudoArchive(record);
                      }}
                      title={intl.formatMessage({
                        id: 'content.action.aipseudo.tips',
                      })}
                    >
                      <FormattedMessage id="content.action.aipseudo" />
                    </a>
                  ),
                },
                {
                  key: '3',
                  label: (
                    <a
                      onClick={() => {
                        handleShowChildren(record);
                      }}
                    >
                      <FormattedMessage id="content.children.btn" />
                    </a>
                  ),
                },
                {
                  key: '4',
                  label: (
                    <a
                      onClick={() => {
                        handleCopyArchive(record);
                      }}
                      title={intl.formatMessage({
                        id: 'content.action.copy.tips',
                      })}
                    >
                      <FormattedMessage id="content.action.copy" />
                    </a>
                  ),
                },
                {
                  key: '5',
                  label: (
                    <a
                      className="text-red"
                      onClick={async () => {
                        await handleRemove([record.id]);
                      }}
                    >
                      <FormattedMessage id="setting.system.delete" />
                    </a>
                  ),
                },
              ],
            }}
            key="more"
          >
            <a>
              <FormattedMessage id="content.action.more" />
            </a>
          </Dropdown>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={onTabChange}>
      <ProTable<any>
        key={newKey}
        headerTitle={
          <div className="module-tags">
            {modules.map((item: any) => (
              <div
                className={
                  'module-tag ' + (item.id === moduleId ? 'active' : '')
                }
                key={item.id}
                onClick={() => {
                  onSelectModule(item.id);
                }}
              >
                {item.id === 0
                  ? intl.formatMessage({
                      id: 'content.archive.all',
                    })
                  : item.name}
              </div>
            ))}
          </div>
        }
        actionRef={actionRef}
        rowKey="id"
        search={{
          span: { xs: 24, sm: 12, md: 8, lg: 8, xl: 8, xxl: 8 },
          defaultCollapsed: false,
        }}
        formRef={formRef}
        form={{
          initialValues: lastParams,
        }}
        beforeSearchSubmit={beforeSearch}
        toolBarRender={() => [
          <Button
            key="recycle"
            onClick={() => {
              history.push('/archive/recycle');
            }}
          >
            <FormattedMessage id="content.recycle.name" />
          </Button>,
          <Button
            key="import"
            onClick={() => {
              setImportVisible(true);
            }}
          >
            <FormattedMessage id="content.quick-import.name" />
          </Button>,
          <Button
            key="replace"
            onClick={() => {
              setReplaceVisible(true);
            }}
          >
            <FormattedMessage id="content.replace.name" />
          </Button>,
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                if (moduleId > 0) {
                  history.push('/archive/detail?module_id=' + moduleId);
                } else {
                  history.push('/archive/detail');
                }
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.archive.add" />
            </Button>
          ),
        ]}
        columnsState={{
          persistenceKey: 'archive-table',
          persistenceType: 'localStorage',
        }}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space wrap>
            <Button
              size={'small'}
              onClick={async () => {
                await setTagVisible(true);
              }}
            >
              <FormattedMessage id="content.option.batch-add-tag" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await setFlagVisible(true);
              }}
            >
              <FormattedMessage id="content.option.change-flag" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await setCategoryVisible(true);
              }}
            >
              <FormattedMessage id="content.option.change-category" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await setTimeVisible(true);
              }}
            >
              <FormattedMessage id="content.option.reflash-time" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await setStatusVisible(true);
              }}
            >
              <FormattedMessage id="content.option.update-status" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await setReleaseVisible(true);
              }}
            >
              <FormattedMessage id="content.option.plan-send" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleTranslate(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-translate" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleAiPseudo(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-pseudo" />
            </Button>
            <Button
              size={'small'}
              onClick={() => handleAddAnchor(selectedRowKeys)}
            >
              <FormattedMessage id="content.option.batch-add-anchor" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={async (params, sort) => {
          // eslint-disable-next-line guard-for-in
          for (let i in sort) {
            params.sort = i;
            params.order = sort[i] === 'ascend' ? 'asc' : 'desc';
          }
          params.exact = lastParams.exact;
          lastParams = params;
          const res = await getArchives(params);
          lastParams.exact = res.exact || false;
          return res;
        }}
        columns={
          contentSetting.use_sort
            ? (columns.splice(1, 0, sortColumn), columns)
            : columns
        }
        rowSelection={{
          preserveSelectedRowKeys: true,
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showQuickJumper: true,
          showSizeChanger: true,
          defaultCurrent: lastParams.current,
          defaultPageSize: lastParams.pageSize,
          pageSizeOptions: ['10', '20', '50', '100', '200', '500'],
          showTotal: (total, range) => (
            <div>
              {lastParams.exact === false && (
                <Tooltip
                  title={intl.formatMessage({
                    id: 'content.count.tips.tooltip',
                  })}
                >
                  <Tag className="link" color="orange" onClick={getExactCount}>
                    <FormattedMessage id="content.count.tips" />
                  </Tag>
                </Tooltip>
              )}
              <FormattedMessage
                id="content.archive.total"
                values={{ total: total, range0: range[0], range1: range[1] }}
              />
            </div>
          ),
        }}
      />
      {replaceVisible && (
        <ReplaceKeywords
          open={replaceVisible}
          onCancel={() => {
            setReplaceVisible(false);
          }}
        />
      )}
      {flagVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.flag.select' })}
          open={flagVisible}
          onFinish={handleSetFlag}
          onOpenChange={(e) => setFlagVisible(e)}
        >
          <ProFormCheckbox.Group
            name="flag"
            valueEnum={{
              h: intl.formatMessage({ id: 'content.flag.h' }),
              c: intl.formatMessage({ id: 'content.flag.c' }),
              f: intl.formatMessage({ id: 'content.flag.f' }),
              a: intl.formatMessage({ id: 'content.flag.a' }),
              s: intl.formatMessage({ id: 'content.flag.s' }),
              b: intl.formatMessage({ id: 'content.flag.b' }),
              p: intl.formatMessage({ id: 'content.flag.p' }),
              j: intl.formatMessage({ id: 'content.flag.j' }),
            }}
          />
        </ModalForm>
      )}
      {statusVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.status.select' })}
          open={statusVisible}
          onFinish={handleSetStatus}
          onOpenChange={(e) => setStatusVisible(e)}
        >
          <ProFormRadio.Group
            name="status"
            valueEnum={{
              0: intl.formatMessage({ id: 'content.status.draft' }),
              1: intl.formatMessage({ id: 'content.status.normal' }),
            }}
          />
        </ModalForm>
      )}
      {timeVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.time.select' })}
          open={timeVisible}
          onFinish={handleSetTime}
          onOpenChange={(e) => setTimeVisible(e)}
        >
          <ProFormRadio.Group
            name="time"
            initialValue={1}
            valueEnum={{
              1: intl.formatMessage({ id: 'content.time.create-time' }),
              2: intl.formatMessage({ id: 'content.time.update-time' }),
              3: intl.formatMessage({ id: 'content.time.create-time-all' }),
              4: intl.formatMessage({ id: 'content.time.update-time-all' }),
            }}
          />
        </ModalForm>
      )}
      {categoryVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.category.select' })}
          open={categoryVisible}
          onFinish={handleSetCategory}
          onOpenChange={(e) => setCategoryVisible(e)}
        >
          <ProFormSelect
            name="category_id"
            label={intl.formatMessage({
              id: 'content.category.name',
            })}
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
          />
          {contentSetting.multi_category === 1 && (
            <ProFormSelect
              name="category_ids"
              mode="multiple"
              label={intl.formatMessage({ id: 'content.archive.related-category' })}
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
        </ModalForm>
      )}
      {releaseVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.plan.name' })}
          open={releaseVisible}
          onFinish={handleSetReleasePlan}
          onOpenChange={(e) => setReleaseVisible(e)}
        >
          <Alert
            className="mb-normal"
            message={
              <div>
                <div>
                  <FormattedMessage id="content.plan.tips" />
                </div>
                <div>
                  <FormattedMessage id="content.plan.tips.before" />{' '}
                  <span className="text-red">{selectedRowKeys.length}</span>{' '}
                  <FormattedMessage id="content.plan.tips.after" />
                </div>
              </div>
            }
          ></Alert>
          <ProFormText
            name="daily_limit"
            label={intl.formatMessage({ id: 'content.plan.daily-limit' })}
            placeholder={intl.formatMessage({
              id: 'content.plan.daily-limit.placeholder',
            })}
            addonAfter={intl.formatMessage({
              id: 'content.plan.daily-limit.suffix',
            })}
          />
          <ProFormText
            name="start_hour"
            initialValue={8}
            label={intl.formatMessage({ id: 'content.plan.start-hour' })}
            placeholder={intl.formatMessage({
              id: 'content.plan.start-hour.placeholder',
            })}
            addonAfter={intl.formatMessage({
              id: 'content.plan.start-hour.suffix',
            })}
          />
          <ProFormText
            name="end_hour"
            initialValue={20}
            label={intl.formatMessage({ id: 'content.plan.end-hour' })}
            placeholder={intl.formatMessage({
              id: 'content.plan.end-hour.placeholder',
            })}
            addonAfter={intl.formatMessage({
              id: 'content.plan.start-hour.suffix',
            })}
          />
        </ModalForm>
      )}
      {quickVisible && (
        <QuickEditForm
          archive={currentArchive}
          open={quickVisible}
          onSubmit={async () => {
            actionRef.current?.reload?.();
            setQuickVisible(false);
            loadLatestUpdate();
          }}
          onCancel={() => setQuickVisible(false)}
        />
      )}
      {importVisible && (
        <QuickImportModal
          open={importVisible}
          onOpenChange={(flag) => {
            setImportVisible(flag);
            if (flag === false) {
              actionRef.current?.reload?.();
            }
          }}
        />
      )}
      {childrenOpen && (
        <ChildrenArchivesModal
          open={childrenOpen}
          parent={currentArchive}
          onOpenChange={(flag) => {
            setChildrenOpen(flag);
          }}
        />
      )}
      {tagVisible && (
        <ModalForm
          width={480}
          title={intl.formatMessage({ id: 'content.tag.name' })}
          open={tagVisible}
          onFinish={handleSetTag}
          onOpenChange={(e) => setTagVisible(e)}
        >
          <ProFormSelect
            mode="tags"
            name="tags"
            valueEnum={searchedTags}
            placeholder={intl.formatMessage({
              id: 'content.tag.placeholder',
            })}
            fieldProps={{
              tokenSeparators: [',', '，'],
              onInputKeyDown: onChangeTagInput,
              onFocus: onChangeTagInput,
            }}
            extra={intl.formatMessage({ id: 'content.tag.placeholder' })}
          />
        </ModalForm>
      )}
    </NewContainer>
  );
};

export default ArchiveList;
