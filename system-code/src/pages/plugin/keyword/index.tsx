import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import {
  pluginDeleteKeyword,
  pluginExportKeyword,
  pluginGetKeywords,
} from '@/services/plugin/keyword';
import { exportFile } from '@/utils';
import KeywordImport from './components/import';
import { digCollectorKeyword } from '@/services/collector';
import KeywordForm from './components/keywordForm';

const PluginKeyword: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentKeyword, setCurrentKeyword] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的关键词吗？',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteKeyword({
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

  const handleEditKeyword = async (record: any) => {
    setCurrentKeyword(record);
    setEditVisible(true);
  };

  const handleExportKeyword = async () => {
    let res = await pluginExportKeyword();

    exportFile(res.data?.header, res.data?.content, 'csv');
  };

  const handleDigKeyword = async () => {
    let res = await digCollectorKeyword();
    message.info(res.msg);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: '关键词',
      dataIndex: 'title',
    },
    {
      title: '层级',
      dataIndex: 'level',
    },
    {
      title: '文章分类ID',
      dataIndex: 'category_id',
    },
    {
      title: '已采集文章',
      dataIndex: 'article_count',
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
              handleEditKeyword(record);
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
        headerTitle="关键词库管理"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditKeyword({});
            }}
          >
            <PlusOutlined /> 添加关键词
          </Button>,
          <Button
            key="export"
            onClick={() => {
              handleExportKeyword();
            }}
          >
            导出关键词
          </Button>,
          <KeywordImport
            onCancel={() => {
              actionRef.current?.reloadAndRest?.();
            }}
          >
            <Button
              key="import"
              onClick={() => {
                //todo
              }}
            >
              导入关键词
            </Button>
          </KeywordImport>,
          <Button
            key="update"
            onClick={() => {
              handleDigKeyword();
            }}
          >
            手动拓词
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
          return pluginGetKeywords(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <KeywordForm
          visible={editVisible}
          editingKeyword={currentKeyword}
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
    </PageContainer>
  );
};

export default PluginKeyword;
