import NewContainer from '@/components/NewContainer';
import {
  addTitleToAnchor,
  deleteCategory,
  getCategories,
  getModules,
  updateCategoryArchiveCount,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ProColumns,
  ProFormInstance,
  ProFormSelect,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import MultiCategory from '../components/multiCategory';
import { getRuntimeShopMode } from '@/utils/shopMode';
import '../index.less';

let lastParams: any = {};

const ArchiveCategory: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<ProFormInstance>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentCategory, setCurrentCategory] = useState<any>({});
  const [modules, setModules] = useState<any[]>([]);
  const [moduleId, setModuleId] = useState<number>(0);
  const [firstFetch, setFirstFetch] = useState<boolean>(false);
  const [multiVisible, setMultiVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const [isSubSite, setIsSubSite] = useState<boolean>(false);
  const intl = useIntl();

  const readModules = async () => {
    const res = await getModules();

    // 商城模式产品独立管理，分类页排除产品模块；普通模式保留
    const list = res.data || [];
    setModules(
      getRuntimeShopMode()
        ? list.filter((item: any) => item.id !== 2)
        : list,
    );
  };

  const onTabChange = (key: string, isSubSite: boolean) => {
    readModules().then(() => {
      setNewKey(key);
      setIsSubSite(isSubSite);
    });
  };

  useEffect(() => {
    setModuleId(lastParams.module_id);
    readModules();
  }, []);

  const onSelectModule = (id: number) => {
    lastParams.module_id = id;
    setModuleId(id);
    formRef.current?.setFieldsValue({
      module_id: id,
      category_id: 0,
    });
    history.replace('/archive/category?module_id=' + id);
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
      title: intl.formatMessage({ id: 'content.category.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteCategory({
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

  const handleAddAnchor = (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.option.batch-add-anchor.confirm',
      }),
      onOk: async () => {
        if (!selectedRowKeys) return true;
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
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

  const handleEditCategory = async (record: any) => {
    history.push(
      '/archive/category/detail?id=' +
        (record.id || 'new') +
        '&parent_id=' +
        record.parent_id,
    );
  };

  const handleAddMultiCategory = async (record: any) => {
    setCurrentCategory(record);
    setMultiVisible(true);
  };

  const handleShowArchives = (record: any) => {
    history.push('/archive/list?category_id=' + record.id);
  };

  const getModuleName = (moduleId: number) => {
    for (let item of modules) {
      if (moduleId === item.id) {
        return item.name;
      }
    }
    return null;
  };

  const handleUpdateArchiveCount = () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'content.category.update-count.confirm',
      }),
      content: intl.formatMessage({
        id: 'content.category.update-count.description',
      }),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        updateCategoryArchiveCount({})
          .then((res) => {
            message.info(res.msg);
            actionRef.current?.reload?.();
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.sort.name' }),
      dataIndex: 'sort',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.category.title' }),
      dataIndex: 'title',
      //hideInSearch: true,
      filters: true,
      render: (dom, entity) => {
        return (
          <>
            {entity.spacer}
            <a href={entity.link} target="_blank" rel="noreferrer">
              {dom}
            </a>
          </>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_id',
      render: (dom, entity) => {
        return getModuleName(entity.module_id);
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
      title: intl.formatMessage({ id: 'content.category.template' }),
      dataIndex: 'template',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.archive-template.name' }),
      dataIndex: 'detail_template',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.category.archive-count' }),
      dataIndex: 'archive_count',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.category.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'content.category.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'content.category.status.ok' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="list"
            onClick={() => {
              handleShowArchives(record);
            }}
          >
            <FormattedMessage id="menu.archive.list" />
          </a>
          {!isSubSite && (
            <>
              <a
                key="add-child"
                onClick={() => {
                  handleEditCategory({
                    parent_id: record.id,
                    module_id: record.module_id,
                    status: 1,
                  });
                }}
              >
                <FormattedMessage id="content.category.add-children" />
              </a>
              <a
                key="add"
                onClick={() => {
                  handleAddMultiCategory({
                    parent_id: record.id,
                    module_id: record.module_id,
                    status: 1,
                  });
                }}
              >
                <FormattedMessage id="content.category.batch-add-children" />
              </a>
            </>
          )}
          <a
            key="edit"
            onClick={() => {
              handleEditCategory(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
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
            <div
              className={'module-tag ' + (0 === moduleId ? 'active' : '')}
              onClick={() => {
                onSelectModule(0);
              }}
            >
              <FormattedMessage id="menu.archive.category" />
            </div>
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
                {item.name}
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
            key="count"
            onClick={() => {
              handleUpdateArchiveCount();
            }}
          >
            <FormattedMessage id="content.category.update-count.name" />
          </Button>,
          !isSubSite && (
            <Button
              key="add"
              onClick={() => {
                handleAddMultiCategory({
                  parent_id: 0,
                  module_id: null,
                  status: 1,
                });
              }}
            >
              <FormattedMessage id="content.category.batch-add" />
            </Button>
          ),
          !isSubSite && (
            <Button
              type="primary"
              key="add"
              onClick={() => {
                handleEditCategory({
                  parent_id: 0,
                  module_id: null,
                  status: 1,
                });
              }}
            >
              <PlusOutlined />{' '}
              <FormattedMessage id="content.category.batch-add-top" />
            </Button>
          ),
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={() => handleAddAnchor(selectedRowKeys)}
            >
              <FormattedMessage id="content.option.batch-add-anchor" />
            </Button>
            <Button
              size={'small'}
              onClick={() => {
                handleRemove(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-delete" />
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              <FormattedMessage id="content.option.cancel-select" />
            </Button>
          </Space>
        )}
        request={async (params) => {
          params.type = 1;
          params.show_type = 1;
          lastParams = params;
          let res = await getCategories(params);
          const list = res.data || [];
          return {
            // 仅商城模式排除产品分类
            data: getRuntimeShopMode()
              ? list.filter((item: any) => item.module_id !== 2)
              : list,
            total: res.total,
          };
        }}
        columnsState={{
          persistenceKey: 'category-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
          defaultCurrent: lastParams.current,
          defaultPageSize: lastParams.pageSize,
        }}
      />
      {multiVisible && (
        <MultiCategory
          open={multiVisible}
          category={currentCategory}
          modules={modules}
          type={1}
          onCancel={() => {
            setMultiVisible(false);
          }}
          onSubmit={async () => {
            setMultiVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
    </NewContainer>
  );
};

export default ArchiveCategory;
