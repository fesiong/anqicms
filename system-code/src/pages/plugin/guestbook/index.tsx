import { Button, message, Modal, Space } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import {
  pluginDeleteGuestbook,
  pluginExportGuestbook,
  pluginGetGuestbooks,
} from '@/services/plugin/guestbook';
import GuestbookForm from './components/guestbookForm';
import { exportFile } from '@/utils';
import GuestbookSetting from './components/setting';

const PluginGuestbook: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentGuestbook, setCurrentGuestbook] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的留言吗？',
      onOk: async () => {
        const hide = message.loading('正在删除', 0);
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteGuestbook({
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

  const handlePreview = async (record: any) => {
    setCurrentGuestbook(record);
    setEditVisible(true);
  };

  const handleExportGuestbook = async () => {
    let res = await pluginExportGuestbook();

    exportFile(res.data?.header, res.data?.content, 'xls');
  };

  const columns: ProColumns<any>[] = [
    {
      title: '时间',
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '用户名',
      width: 100,
      dataIndex: 'user_name',
    },
    {
      title: '联系方式',
      width: 160,
      dataIndex: 'contact',
    },
    {
      title: '留言内容',
      dataIndex: 'content',
      render: (text, record) => <div style={{ wordBreak: 'break-all' }}>{text}</div>,
    },
    {
      title: 'IP',
      dataIndex: 'ip',
      width: 100,
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
              handlePreview(record);
            }}
          >
            查看
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
        headerTitle="网站留言管理"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <Button
            key="export"
            onClick={() => {
              handleExportGuestbook();
            }}
          >
            导出留言
          </Button>,
          <GuestbookSetting>
            <Button
              key="setting"
              onClick={() => {
                //todo
              }}
            >
              网站留言设置
            </Button>
          </GuestbookSetting>,
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
          return pluginGetGuestbooks(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      {editVisible && (
        <GuestbookForm
          visible={editVisible}
          editingGuestbook={currentGuestbook}
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

export default PluginGuestbook;
