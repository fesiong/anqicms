import {
  deleteArchive,
  getArchiveInfo,
  getArchives,
  recoverArchive,
} from '@/services';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

const ArchiveRecycleList: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [visible, setVisible] = useState<boolean>(false);
  const [currentArchive, setCurrentArchive] = useState<any>(null);
  const intl = useIntl();

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await deleteArchive({
              id: item,
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleRecover = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'content.recover.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.recover.recovering' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let item of selectedRowKeys) {
            await recoverArchive({
              id: item,
            });
          }
          hide();
          message.success(
            intl.formatMessage({ id: 'content.recover.success' }),
          );
          setSelectedRowKeys([]);
          actionRef.current?.reloadAndRest?.();
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.recover.failure' }));
          return true;
        }
      },
    });
  };

  const previewArchive = (item: any) => {
    setCurrentArchive(item);
    setVisible(true);
    getArchiveInfo({ id: item.id }).then((res) => {
      setCurrentArchive(res.data || {});
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'content.title.name' }),
      dataIndex: 'title',
      hideInSearch: true,
      render: (dom, entity) => {
        return (
          <div style={{ maxWidth: 400 }}>
            <a onClick={() => previewArchive(entity)}>{dom}</a>
          </div>
        );
      },
    },
    {
      title: 'thumb',
      dataIndex: 'thumb',
      hideInSearch: true,
      render: (text, record) => {
        return text ? <img src={record.thumb} className="list-thumb" /> : null;
      },
    },
    {
      title: intl.formatMessage({ id: 'content.module.name' }),
      dataIndex: 'module_name',
      hideInSearch: true,
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
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
            <FormattedMessage id="content.action.recover" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              await handleRemove([record.id]);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'content.recycle.name' })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => []}
        tableAlertOptionRender={({ selectedRowKeys, onCleanSelected }) => (
          <Space>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRecover(selectedRowKeys);
              }}
            >
              <FormattedMessage id="content.option.batch-recover" />
            </Button>
            <Button
              size={'small'}
              onClick={async () => {
                await handleRemove(selectedRowKeys);
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
          params.recycle = true;
          return getArchives(params);
        }}
        columnsState={{
          persistenceKey: 'archive-recycle-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      <Modal
        title={intl.formatMessage({ id: 'content.preview' })}
        open={visible}
        width={1000}
        onOk={() => setVisible(false)}
        onCancel={() => setVisible(false)}
      >
        <h3>{currentArchive?.title}</h3>
        <div className="article-content">
          <div
            dangerouslySetInnerHTML={{ __html: currentArchive?.data?.content }}
          ></div>
        </div>
      </Modal>
    </PageContainer>
  );
};

export default ArchiveRecycleList;
