import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import CommentForm from './components/commentForm';
import { pluginDeleteComment, pluginGetComments } from '@/services/plugin/comment';

const PluginComment: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentComment, setCurrentComment] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的评论吗？',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteComment({
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

  const handleEditComment = async (record: any) => {
    setCurrentComment(record);
    setEditVisible(true);
  };

  const handlePreviewComment = async (record: any) => {
    setCurrentComment(record);
    setEditVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '类型',
      dataIndex: 'item_type',
      width: 60,
      valueEnum: {
        article: '文章',
        product: '产品',
      },
    },
    {
      title: '名称',
      width: 200,
      dataIndex: 'item_title',
      render: (text, record) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: '用户名',
      width: 100,
      dataIndex: 'user_name',
      render: (text, record) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: '评论内容',
      dataIndex: 'content',
      render: (text, record) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 100,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 60,
      valueEnum: {
        0: {
          text: '待审',
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
      width: 150,
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handlePreviewComment(record);
            }}
          >
            查看/编辑
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
        headerTitle="评论管理"
        actionRef={actionRef}
        rowKey="id"
        search={false}
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
          return pluginGetComments(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <CommentForm
          visible={editVisible}
          editingComment={currentComment}
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

export default PluginComment;
