import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import { deleteModule, getModules } from '@/services';
import ModuleForm from './components/moduleForm';
import { history } from 'umi';

const ModuleList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentModule, setCurrentModule] = useState<any>({});

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的模型吗？',
      content: '该模型下的分类、文档也会一并被删除，请谨慎操作。',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteModule({
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

  const handleEditModule = async (record: any) => {
    setCurrentModule(record);
    setEditVisible(true);
  };

  const handleShowArchive = async (record: any) => {
    history.push('/archive/list?module_id=' + record.id);
  };

  const columns: ProColumns<any>[] = [
    {
      title: '编号',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: '模型名称',
      dataIndex: 'title'
    },
    {
      title: '模型表名',
      dataIndex: 'table_name',
    },
    {
      title: '标题名称',
      dataIndex: 'title_name',
      hideInSearch: true,
    },
    {
      title: '模型',
      dataIndex: 'is_system',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: '自定义',
        },
        1: {
          text: '系统',
        },
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: '未启用',
          status: 'Default',
        },
        1: {
          text: '正常',
          status: 'Success',
        },
      },
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleShowArchive(record);
            }}
          >
            文档列表
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditModule(record);
            }}
          >
            编辑
          </a>
          {record.is_system == 0 && <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
              setSelectedRowKeys([]);
              actionRef.current?.reloadAndRest?.();
            }}
          >
            删除
          </a>}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="内容模型列表"
        actionRef={actionRef}
        rowKey="id"
        search={{}}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditModule({});
            }}
          >
            <PlusOutlined /> 添加模型
          </Button>,
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
          return getModules(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <ModuleForm
          visible={editVisible}
          module={currentModule}
          type={1}
          onCancel={() => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
          onSubmit={async () => {
            setEditVisible(false);
            if (actionRef.current) {
              actionRef.current.reload();
            }
          }}
        />
      )}
    </PageContainer>
  );
};

export default ModuleList;
