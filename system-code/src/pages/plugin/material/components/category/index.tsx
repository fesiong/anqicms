import React, { useRef, useState } from 'react';
import { Button, Input, message, Modal, Space } from 'antd';
import {
  pluginDeleteMaterialCategory,
  pluginGetMaterialCategories,
  pluginSaveMaterialCategory,
} from '@/services/plugin/material';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';

export type MaterialCategoryProps = {
  onCancel: (flag?: boolean) => void;
};

const MaterialCategory: React.FC<MaterialCategoryProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisbile, setEditVisible] = useState<boolean>(false);
  const [editingCategory, setEditingCategory] = useState<any>({});
  const [editingInput, setEditingInput] = useState<string>('');

  const handleAddCategory = () => {
    setEditingCategory({});
    setEditingInput('');
    setEditVisible(true);
  };

  const handleEditCategory = (record: any) => {
    setEditingCategory(record);
    setEditingInput(record.title);
    setEditVisible(true);
  };

  const handleRemove = async (record: any) => {
    let res = await pluginDeleteMaterialCategory(record);

    message.info(res.msg);
    actionRef.current?.reloadAndRest?.();
  };

  const handleSaveCategory = () => {
    const hide = message.loading('正在提交中', 0);
    pluginSaveMaterialCategory({
      id: editingCategory.id,
      title: editingInput,
    })
      .then((res) => {
        if (res.code === 0) {
          setEditVisible(false);

          actionRef.current?.reloadAndRest?.();
        } else {
          message.error(res.msg);
        }
      })
      .catch((err) => {
        console.log(err);
      }).finally(() => {
        hide();
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
    },
    {
      title: '板块名称',
      dataIndex: 'title',
    },
    {
      title: '素材数量',
      dataIndex: 'material_count',
      width: 80,
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      width: 120,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditCategory(record);
            }}
          >
            编辑
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove(record);
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
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        visible={visible}
        title={
          <Button
            type="primary"
            onClick={() => {
              handleAddCategory();
            }}
          >
            新增板块
          </Button>
        }
        width={600}
        onCancel={() => {
          setVisible(false);
          props.onCancel();
        }}
        footer={false}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <ProTable<any>
            headerTitle="内容素材类别管理"
            actionRef={actionRef}
            rowKey="id"
            search={false}
            pagination={false}
            toolBarRender={false}
            request={(params, sort) => {
              return pluginGetMaterialCategories(params);
            }}
            columns={columns}
          />
        </div>
      </Modal>
      <Modal
        visible={editVisbile}
        title={editingCategory.id ? '重命名板块：' + editingCategory.title : '新增板块'}
        width={480}
        zIndex={2000}
        okText="确认"
        cancelText="取消"
        maskClosable={false}
        onOk={handleSaveCategory}
        onCancel={() => {
          setEditVisible(false);
        }}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <p>请填写板块名称: </p>
          <Input
            size="large"
            value={editingInput}
            onChange={(e) => {
              setEditingInput(e.target.value);
            }}
          />
        </div>
      </Modal>
    </>
  );
};

export default MaterialCategory;
