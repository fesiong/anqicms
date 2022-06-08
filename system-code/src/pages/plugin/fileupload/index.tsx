import { PlusOutlined } from '@ant-design/icons';
import { Button, message, Input, Drawer, Modal, Space, Alert, Card, Upload } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import {
  pluginDeleteFile,
  pluginGetUploadFiles,
  pluginUploadFile,
} from '@/services/plugin/fileupload';

const PluginFileupload: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [visible, setVisible] = useState<boolean>(false);

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: '确定要删除选中的文件吗？',
      onOk: async () => {
        const hide = message.loading('正在删除');
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteFile({
              hash: item,
            });
          }
          hide();
          message.success('删除成功');
          if (actionRef.current) {
            actionRef.current.reload();
          }
        } catch (error) {
          hide();
          message.error('删除失败');
        }
      },
    });
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    pluginUploadFile(formData).then((res) => {
      message.success(res.msg);
      setVisible(false);
      actionRef.current?.reloadAndRest?.();
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: '文件名',
      dataIndex: 'file_name',
    },
    {
      title: '时间',
      width: 200,
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '操作',
      dataIndex: 'option',
      valueType: 'option',
      width: 180,
      render: (_, record) => (
        <Space size={20}>
          <a key="edit" target="_blank" href={record.link}>
            查看
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.hash]);
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
        headerTitle="验证文件上传管理"
        actionRef={actionRef}
        rowKey="hash"
        search={false}
        toolBarRender={() => [
          <Button
            key="upload"
            onClick={() => {
              setVisible(true);
            }}
          >
            <PlusOutlined /> 上传新文件
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
          return pluginGetUploadFiles(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
      />

      <Modal
        title="上传新文件"
        visible={visible}
        width={800}
        cancelText="关闭"
        okText={false}
        onCancel={() => {
          setVisible(false);
        }}
        onOk={() => {
          setVisible(false);
        }}
      >
        <Alert message={'说明：只允许上传 txt/htm/html 格式的验证文件'} />
        <div className="mt-normal">
          <div className="text-center">
            <Upload
              name="file"
              className="logo-uploader"
              showUploadList={false}
              accept=".txt,.htm,.html"
              customRequest={handleUploadFile}
            >
              <Button type="primary">上传文件</Button>
            </Upload>
          </div>
        </div>
      </Modal>
    </PageContainer>
  );
};

export default PluginFileupload;
