import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import { pluginCheckLink, pluginDeleteLink, pluginGetLinks } from '@/services/plugin/link';
import moment from 'moment';
import LinkForm from './components/linkForm';

const PluginLink: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentLink, setCurrentLink] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的友情链接吗？',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteLink({
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

  const handleEditLink = async (record: any) => {
    setCurrentLink(record);
    setEditVisible(true);
  };

  const handleCheckLink = async (record: any) => {
    let res = await pluginCheckLink(record);
    message.info(res.msg);
    if (actionRef.current) {
      actionRef.current.reload();
    }
  };

  const getStatusText = (status: any) => {
    if (status === 0) {
      return '待检测';
    } else if (status === 1) {
      return '正常';
    } else if (status === 2) {
      return 'NOFOLLOW';
    } else if (status === 3) {
      return '关键词不一致';
    } else if (status === 4) {
      return '对方无反链';
    }

    return status;
  };

  const columns: ProColumns<any>[] = [
    {
      title: '编号',
      dataIndex: 'sort',
    },
    {
      title: '对方关键词/链接',
      dataIndex: 'title',
      render: (text, record) => {
        return (
          <div>
            <span>{record.title}</span>
            <span> / </span>
            <a href={record.link} target="_blank">
              {record.link}
            </a>
          </div>
        );
      },
    },
    {
      title: '对方联系方式/备注',
      dataIndex: 'contact',
      render: (text, record) => {
        return (
          <div>
            <span>{record.contact}</span>
            <span> / </span>
            <span>{record.remark}</span>
          </div>
        );
      },
    },
    {
      title: '状态/检查时间',
      dataIndex: 'status',
      render: (text, record) => {
        return (
          <div>
            <span>{getStatusText(text)}</span>
            <span> / </span>
            <span>{moment(record.checked_time * 1000).format('YYYY-MM-DD HH:mm')}</span>
          </div>
        );
      },
    },
    {
      title: '添加时间',
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handleCheckLink(record);
            }}
          >
            检查
          </a>
          <a
            key="edit"
            onClick={() => {
              handleEditLink(record);
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
        headerTitle="友情链接管理"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditLink({});
            }}
          >
            <PlusOutlined /> 添加友情链接
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
          return pluginGetLinks(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <LinkForm
          visible={editVisible}
          editingLink={currentLink}
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

export default PluginLink;
