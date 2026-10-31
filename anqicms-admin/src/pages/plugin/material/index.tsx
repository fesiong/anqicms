import {
  pluginDeleteMaterial,
  pluginGetMaterialCategories,
  pluginGetMaterials,
} from '@/services/plugin/material';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Select, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import MaterialCategory from './components/category';
import MaterialImport from './components/import';
import MaterialForm from './components/materialForm';

const PluginMaterial: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentMaterial, setCurrentMaterial] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [previewVisible, setPreviewVisible] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [categoryId, setCategoryId] = useState<number>(0);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetMaterialCategories();
    let categories = res.data || [];
    setCategories(categories);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.material.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteMaterial({
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

  const handleChangeCategory = async (e: any) => {
    setCategoryId(e);
    actionRef.current?.reloadAndRest?.();
  };

  const handleEditMaterial = async (record: any) => {
    setCurrentMaterial(record);
    setEditVisible(true);
  };

  const handlePreviewMaterial = async (record: any) => {
    setCurrentMaterial(record);
    setPreviewVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
    },
    {
      title: intl.formatMessage({ id: 'plugin.material.content' }),
      dataIndex: 'content',
      render: (text) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.material.category.title' }),
      dataIndex: 'category_title',
      width: 150,
    },
    {
      title: intl.formatMessage({ id: 'plugin.material.user-count' }),
      dataIndex: 'use_count',
      width: 80,
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 180,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handlePreviewMaterial(record);
            }}
          >
            <FormattedMessage id="plugin.material.preview" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditMaterial(record);
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
    <PageContainer>
      <ProTable<any>
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <span key="filter">
            <FormattedMessage id="plugin.material.category-filter" />
          </span>,
          <Select
            key="select"
            defaultValue={categoryId}
            style={{ width: 120 }}
            onChange={handleChangeCategory}
          >
            <Select.Option value={0}>
              <FormattedMessage id="plugin.material.all" />
            </Select.Option>
            {categories.map((item: any) => (
              <Select.Option key={item.id} value={item.id}>
                {item.title}
              </Select.Option>
            ))}
          </Select>,
          <MaterialImport
            key="import"
            onCancel={() => {
              getSetting();
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              onClick={() => {
                //todo
              }}
            >
              <PlusOutlined /> <FormattedMessage id="plugin.material.add" />
            </Button>
          </MaterialImport>,
          <MaterialCategory
            key="category"
            onCancel={() => {
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              key="category"
              onClick={() => {
                //todo
              }}
            >
              <FormattedMessage id="plugin.material.category.manage" />
            </Button>
          </MaterialCategory>,
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
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
          params.category_id = categoryId;
          return pluginGetMaterials(params);
        }}
        columnsState={{
          persistenceKey: 'material-table',
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
        }}
      />
      {editVisible && (
        <MaterialForm
          open={editVisible}
          editingMaterial={currentMaterial}
          onCancel={() => {
            setEditVisible(false);
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
      <Modal
        title={intl.formatMessage({ id: 'plugin.material.preview' })}
        open={previewVisible}
        width={800}
        cancelText={intl.formatMessage({ id: 'setting.action.close' })}
        okText={false}
        onCancel={() => {
          setPreviewVisible(false);
        }}
        onOk={() => {
          setPreviewVisible(false);
        }}
      >
        <div className="article-content">
          <div
            dangerouslySetInnerHTML={{ __html: currentMaterial.content }}
          ></div>
        </div>
      </Modal>
    </PageContainer>
  );
};

export default PluginMaterial;
