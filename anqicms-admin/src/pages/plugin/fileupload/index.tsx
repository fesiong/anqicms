import NewContainer from '@/components/NewContainer';
import {
  pluginDeleteFile,
  pluginGetUploadFiles,
  pluginUploadFile,
} from '@/services/plugin/fileupload';
import { PlusOutlined } from '@ant-design/icons';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Modal, Space, Upload, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const PluginFileupload: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [visible, setVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.fileupload.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await pluginDeleteFile({
              hash: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
        }
      },
    });
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginUploadFile(formData)
      .then((res) => {
        message.success(res.msg);
        setVisible(false);
        actionRef.current?.reloadAndRest?.();
      })
      .finally(() => {
        hide();
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'design.detail.file-name' }),
      dataIndex: 'file_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.fileupload.create-time' }),
      width: 200,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 180,
      render: (_, record) => (
        <Space size={20}>
          <a key="edit" target="_blank" href={record.link}>
            <FormattedMessage id="plugin.fileupload.view" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove([record.hash]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProTable<any>
        key={newKey}
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
            <PlusOutlined />{' '}
            <FormattedMessage id="plugin.fileupload.upload.name" />
          </Button>,
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
          return pluginGetUploadFiles(params);
        }}
        columnsState={{
          persistenceKey: 'fileupload-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={false}
      />

      <Modal
        title={intl.formatMessage({ id: 'plugin.fileupload.upload.name' })}
        open={visible}
        width={800}
        okText={false}
        onCancel={() => {
          setVisible(false);
        }}
        onOk={() => {
          setVisible(false);
        }}
      >
        <Alert
          message={intl.formatMessage({
            id: 'plugin.fileupload.upload.support',
          })}
        />
        <div className="mt-normal">
          <div className="text-center">
            <Upload
              name="file"
              className="logo-uploader"
              showUploadList={false}
              accept=".txt,.htm,.html,.xml"
              customRequest={handleUploadFile}
            >
              <Button type="primary">
                <FormattedMessage id="plugin.fileupload.upload.btn" />
              </Button>
            </Upload>
          </div>
        </div>
      </Modal>
    </NewContainer>
  );
};

export default PluginFileupload;
