import NewContainer from '@/components/NewContainer';
import ReplaceKeywords from '@/components/replaceKeywords';
import {
  addTitleToAnchor,
  anqiAiPseudoArchive,
  anqiTranslateArchive,
  deleteArchive,
  getArchives,
  getSettingContent,
  updateArchivesCategory,
  updateArchivesFlag,
  updateArchivesReleasePlan,
  updateArchivesSort,
  updateArchivesStatus,
  updateArchivesTime,
} from '@/services';
import { getCategories } from '@/services/category';
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
  Menu,
  Modal,
  Select,
  Space,
  Tag,
  Tooltip,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
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

const ProductList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [replaceVisible, setReplaceVisible] = useState<boolean>(false);
  const [flagVisible, setFlagVisible] = useState<boolean>(false);
  const [statusVisible, setStatusVisible] = useState<boolean>(false);
  const [categoryVisible, setCategoryVisible] = useState<boolean>(false);
  const [timeVisible, setTimeVisible] = useState<boolean>(false);
  const [releaseVisible, setReleaseVisible] = useState<boolean>(false);
  const [moduleId, setModuleId] = useState<number>(2);
  const [contentSetting, setContentSetting] = useState<any>({});
  const [currentArchive, setCurrentArchive] = useState<any>({});
  const [quickVisible, setQuickVisible] = useState<boolean>(false);
  const [firstFetch, setFirstFetch] = useState<boolean>(false);
  const [latestUpdateId, setLatestUpdateId] = useState<number>(0);
  const [importVisible, setImportVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
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
    setModuleId(2);
    loadContentSetting();
    loadLatestUpdate();

    setNewKey(key);
    setIsSubSite(isSubSite);
  };

  useEffect(() => {
    setModuleId(2);
    loadContentSetting();
    loadLatestUpdate();
  }, []);

  const beforeSearch = (searchParams: any) => {
    let params = searchParams;
    if (!firstFetch) {
      setFirstFetch(true);
      const searchParams = new URLSearchParams(window.location.search);
      lastParams.module_id = 2;
      lastParams.category_id = Number(searchParams.get('category_id') || 0);
      lastParams.status = searchParams.get('status') || 'ok';
      formRef.current?.setFieldsValue(lastParams);
      params = lastParams;
    } else {
      lastParams = params;
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
      message.error(intl.formatMessage({ id: 'content.category.required' }));
      return;
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
    history.push('/product/detail?id=' + record.id);
  };

  const handleCopyArchive = async (record: any) => {
    history.push('/product/detail?copyid=' + record.id);
  };

  const handleShowChildren = (record: any) => {
    setCurrentArchive(record);
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
    value = parseInt(value);
    if (isNaN(value)) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    if (value === record.sort) {
      return;
    }
    if (value < 0) {
      message.error(intl.formatMessage({ id: 'content.sort.required' }));
      return;
    }
    updating = true;
    updateArchivesSort({
      sort: value,
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
            <a href={entity.link} target="_blank">
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
      title: intl.formatMessage({ id: 'content.product.type' }),
      dataIndex: 'product_type',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.product.type.all' }),
        },
        1: {
          text: intl.formatMessage({ id: 'content.product.type.normal' }),
        },
        2: {
          text: intl.formatMessage({ id: 'content.product.type.wholesale' }),
        },
        3: {
          text: intl.formatMessage({ id: 'content.product.type.custom' }),
        },
        // 4: {
        //   text: intl.formatMessage({ id: 'content.product.type.virtual' }),
        // },
        // 5: {
        //   text: intl.formatMessage({ id: 'content.product.type.service' }),
        // },
        // 6: {
        //   text: intl.formatMessage({ id: 'content.product.type.subscription' }),
        // },
      },
    },
    {
      title: intl.formatMessage({ id: 'content.price.name' }),
      dataIndex: 'price',
      hideInSearch: true,
      width: 70,
      render: (_, record) => {
        return record.price > 0 ? (record.price / 100).toFixed(2) : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'content.stock.name' }),
      dataIndex: 'stock',
      hideInSearch: true,
      width: 70,
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
      renderFormItem: (_, { fieldProps }) => {
        return (
          <ProFormSelect
            name="category_id"
            request={async () => {
              let res = await getCategories({ type: 1, module_id: 2 });
              const categories = [
                {
                  spacer: '',
                  title: intl.formatMessage({ id: 'content.category.all' }),
                  id: 0,
                  status: 1,
                },
              ]
                .concat(res.data || [])
                .map((cat: any) => ({
                  spacer: cat.spacer,
                  label:
                    cat.title +
                    (cat.status === 1
                      ? ''
                      : intl.formatMessage({ id: 'setting.nav.hide' })),
                  value: cat.id,
                }));
              return categories;
            }}
            fieldProps={{
              ...fieldProps,
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
        );
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
            overlay={
              <Menu>
                <Menu.Item>
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
                </Menu.Item>
                <Menu.Item>
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
                </Menu.Item>
                <Menu.Item>
                  <a
                    onClick={() => {
                      handleShowChildren(record);
                    }}
                  >
                    <FormattedMessage id="content.children.btn" />
                  </a>
                </Menu.Item>
                <Menu.Item>
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
                </Menu.Item>
                <Menu.Item danger>
                  <a
                    className="text-red"
                    onClick={async () => {
                      await handleRemove([record.id]);
                    }}
                  >
                    <FormattedMessage id="setting.system.delete" />
                  </a>
                </Menu.Item>
              </Menu>
            }
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
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                history.push('/product/detail');
              }}
            >
              <PlusOutlined /> <FormattedMessage id="content.product.add" />
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
            name="category_ids"
            request={async () => {
              let res = await getCategories({ type: 1, module_id: 2 });
              return [
                {
                  spacer: '',
                  title: intl.formatMessage({ id: 'content.please-select' }),
                  id: 0,
                },
              ].concat(res.data || []);
            }}
            fieldProps={{
              mode:
                contentSetting.multi_category === 1 ? 'multiple' : undefined,
              fieldNames: {
                label: 'title',
                value: 'id',
              },
              optionItemRender(item: any) {
                return (
                  <div
                    dangerouslySetInnerHTML={{
                      __html: item.spacer + item.title,
                    }}
                  ></div>
                );
              },
            }}
          />
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
    </NewContainer>
  );
};

export default ProductList;
