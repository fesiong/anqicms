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
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import MultiCategory from '../components/multiCategory';
import '../index.less';

let lastParams: any = {};

const ProductCategory: React.FC = () => {
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
    setModules(res.data || []);
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
      lastParams.module_id = 2;
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

  const handleEditCategory = async (record: any) => {
    history.push(
      '/product/category/detail?id=' +
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
    history.push('/product/index?category_id=' + record.id);
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
            <div
              className="spacer"
              dangerouslySetInnerHTML={{ __html: entity.spacer }}
            ></div>
            <a href={entity.link} target="_blank">
              {dom}
            </a>
          </>
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
            <FormattedMessage id="menu.product.product" />
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
                  module_id: 2,
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
                  module_id: 2,
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
        request={(params) => {
          params.type = 1;
          params.show_type = 1;
          params.module_id = 2;
          lastParams = params;
          return getCategories(params);
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

export default ProductCategory;
