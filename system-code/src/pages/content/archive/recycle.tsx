import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import { history } from 'umi';
import ReplaceKeywords from '@/components/replaceKeywords';
import { deleteArchive, getArchives, getModules, recoverArchive } from '@/services';

const ArchiveList: React.FC = (props) => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [replaceVisible, setReplaceVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的文档吗？',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteArchive({
              id: item,
            });
          }
          hide();
          message.success('删除成功');
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error('删除失败');
          return true;
        }
      },
    });
  };

  const handleRecover = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要恢复选中的文档吗？',
      onOk: async () => {
        const hide = message.loading('正在恢复');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await recoverArchive({
              id: item,
            });
          }
          hide();
          message.success('恢复成功');
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error('恢复失败');
          return true;
        }
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: '编号',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: '标题',
      dataIndex: 'title',
      hideInSearch: true,
      render: (dom, entity) => {
        return (
          <div style={{maxWidth: 400}}><a href={entity.link} target="_blank">
          {dom}
        </a></div>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      render: (text, record) => {
        return (
          text ? <img src={record.thumb} className='list-thumb' /> : null
        );
      },
    },
    {
      title: '内容模型',
      dataIndex: 'module_name',
      hideInSearch: true,
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
        <a
          className="text-red"
          key="recover"
          onClick={async () => {
            await handleRecover([record.id]);
          }}
        >
          恢复
        </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
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
        headerTitle="文档回收站"
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={false}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
          <Button
            size={'small'}
            onClick={async () => {
              await handleRecover(selectedRowKeys);
            }}
          >
            批量恢复
          </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
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
          params.recycle = true;
          return getArchives(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />
      <ReplaceKeywords
        visible={replaceVisible}
        onCancel={() => {
          setReplaceVisible(false);
        }}
      />
    </PageContainer>
  );
};

export default ArchiveList;
