import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Modal, Space, Select } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';

import { removeHtmlTag } from '@/utils';
import MaterialImport from './components/import';
import { pluginDeleteMaterial, pluginGetMaterialCategories, pluginGetMaterials } from '@/services/plugin/material';
import MaterialForm from './components/materialForm';
import MaterialCategory from './components/category';

const PluginMaterial: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentMaterial, setCurrentMaterial] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [previewVisible, setPreviewVisible] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [categoryId, setCategoryId] = useState<number>(0);

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetMaterialCategories();
    let categories = res.data || [];
    setCategories(categories);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的素材吗？',
      onOk: async () => {
        const hide = message.loading('正在删除', 0);
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteMaterial({
              id: item,
            });
          }
          hide();
          message.success('删除成功');
          if (actionRef.current) {
            actionRef.current.reload();
          }
          return true;
        } catch (error) {
          hide();
          message.error('删除失败');
          return true;
        }
      },
    });
  };

  const handleChangeCategory = async (e: any) => {
    setCategoryId(e)
    actionRef.current?.reloadAndRest?.();
  }

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
      title: '内容',
      dataIndex: 'content',
      render: (text) => <div style={{ wordBreak: 'break-all' }}>{removeHtmlTag(text)}</div>,
    },
    {
      title: '板块',
      dataIndex: 'category_title',
      width: 150,
    },
    {
      title: '引用数量',
      dataIndex: 'use_count',
      width: 80,
    },
    {
      title: '操作',
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
            预览
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditMaterial(record);
            }}
          >
            编辑
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
              setSelectedRowKeys([]);
              actionRef.current?.reloadAndRest?.();
            }}
          >
            删除
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="内容素材管理"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <span>分类筛选</span>,
                <Select
                  defaultValue={categoryId}
                  style={{ width: 120 }}
                  onChange={handleChangeCategory}
                >
                  <Select.Option value={0}>全部资源</Select.Option>
                  {categories.map((item: any, index) => (
                    <Select.Option key={item.id} value={item.id}>
                      {item.title}
                    </Select.Option>
                  ))}
                </Select>,
          <MaterialImport
            onCancel={() => {
              getSetting();
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              key="import"
              onClick={() => {
                //todo
              }}
            >
              <PlusOutlined /> 添加素材
            </Button>
          </MaterialImport>,
          <MaterialCategory
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
              板块管理
            </Button>
          </MaterialCategory>,
        ]}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
                setSelectedRowKeys([]);
                actionRef.current?.reloadAndRest?.();
              }}
            >
              批量删除
            </Button>
            <Button type="link" size={'small'} onClick={onCleanSelected}>
              取消选择
            </Button>
          </Space>
        )}
        request={(params, sort) => {
          params.category_id = categoryId;
          return pluginGetMaterials(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <MaterialForm
          visible={editVisible}
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
        title="内容预览"
        visible={previewVisible}
        width={800}
        cancelText="关闭"
        okText={false}
        onCancel={() => {
          setPreviewVisible(false);
        }}
        onOk={() => {
          setPreviewVisible(false);
        }}
      >
        <div dangerouslySetInnerHTML={{ __html: currentMaterial.content }}></div>
      </Modal>
    </PageContainer>
  );
};

export default PluginMaterial;
