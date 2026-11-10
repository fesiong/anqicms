import { pluginDeleteActivity, pluginGetActivities } from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';
import ActivityForm from './components/activityForm';

const PluginActivity: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [currentActivity, setCurrentActivity] = useState<any>({});
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleRemove = async (selectedRowKeys: any[]) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.keyword.delete.confirm' }),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'content.delete.deletting' }),
          0,
        );
        if (!selectedRowKeys) return true;
        try {
          for (let i = 0; i < selectedRowKeys.length; i++) {
            await pluginDeleteActivity({
              id: selectedRowKeys[i],
            });
          }
          hide();
          message.success(intl.formatMessage({ id: 'content.delete.success' }));
          setSelectedRowKeys([]);
          if (actionRef.current) {
            actionRef.current.reloadAndRest?.();
          }
          return true;
        } catch (error) {
          hide();
          message.error(intl.formatMessage({ id: 'content.delete.failure' }));
          return true;
        }
      },
    });
  };

  const handleEditActivity = async (record: any) => {
    setCurrentActivity(record);
    setEditVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      hideInSearch: true,
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.description' }),
      hideInSearch: true,
      dataIndex: 'description',
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.goods' }),
      hideInSearch: true,
      dataIndex: 'goods.title',
      render: (_, record) => <div>{record.goods?.title || '-'}</div>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.origin-price' }),
      hideInSearch: true,
      dataIndex: 'goods.price',
      render: (_, record) => (
        <div>{(record.goods?.price / 100).toFixed(2)}</div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.price' }),
      hideInSearch: true,
      dataIndex: 'price',
      render: (_, record) => <div>{(record.price / 100).toFixed(2)}</div>,
    },
    {
      title: intl.formatMessage({ id: 'plugin.activity.start-time' }),
      hideInSearch: true,
      sorter: true,
      dataIndex: 'start_time',
      render: (_, record) => {
        return (
          <div>
            {dayjs(record.start_time * 1000).format('YYYY-MM-DD')} -
            {dayjs(record.end_time * 1000).format('YYYY-MM-DD')}
          </div>
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.coupon.status' }),
      dataIndex: 'status',
      hideInSearch: true,
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'plugin.coupon.status.hide' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'plugin.coupon.status.show' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditActivity(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={async () => {
              handleRemove([record.id]);
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
        headerTitle={intl.formatMessage({ id: 'menu.sale.activity' })}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditActivity({ status: 1 });
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.activity.add" />
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
          return pluginGetActivities(params);
        }}
        columnsState={{
          persistenceKey: 'plugin-activity-table',
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
      {editVisible && (
        <ActivityForm
          open={editVisible}
          activity={currentActivity}
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

export default PluginActivity;
