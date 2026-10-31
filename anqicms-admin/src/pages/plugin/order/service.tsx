import {
  pluginDeleteOrderService,
  pluginGetOrderServices,
  pluginSaveOrderService,
} from '@/services';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormRadio,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Modal, Space, Switch, message } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

const PluginOrderService: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [currentService, setcurrentService] = useState<any>({});
  const [feeType, setFeeType] = useState<number>(0);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const intl = useIntl();
  const handleEditService = async (record: any) => {
    setcurrentService({
      ...record,
      fee: record.fee / 100,
      max_fee: record.max_fee > 0 ? record.max_fee / 100 : 0,
      percent: record.fee_type === 2 ? record.fee : 0,
    });
    setFeeType(record.feeType || 0);
    setEditVisible(true);
  };

  const handleDeleteService = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.order.service.delete.confirm' }),
      onOk: async () => {
        await pluginDeleteOrderService({ id: record.id });
        actionRef.current?.reload();
      },
    });
  };

  const saveOrderService = async (values: any) => {
    let postData = {
      ...currentService,
      ...values,
      fee:
        values.fee_type === 2
          ? values.percent
          : Number((values.fee * 100).toFixed(0)) || 0,
      max_fee:
        values.fee_type === 2 ? Number((values.max_fee * 100).toFixed(0)) : 0,
    };

    pluginSaveOrderService(postData).then((res) => {
      if (res.code !== 0) {
        message.error(res.msg);
        return;
      }
      setEditVisible(false);
      actionRef.current?.reload();
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.order.service-id' }),
      dataIndex: 'id',
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service-name' }),
      dataIndex: 'service_name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service-code' }),
      dataIndex: 'service_code',
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service-type' }),
      dataIndex: 'service_type',
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service-enabled' }),
      dataIndex: 'enabled',
      hideInSearch: true,
      render: (dom: any) => {
        return <Switch checked={dom} disabled />;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service-fee' }),
      dataIndex: 'fee',
      hideInSearch: true,
      render: (dom: any, record: any) => (
        <div>
          {record.fee_type === 2
            ? record.fee +
              '%' +
              (record.max_fee > 0
                ? ', ' +
                  intl.formatMessage({
                    id: 'plugin.order.service-fee-max',
                  }) +
                  (record.max_fee / 100).toFixed(2)
                : '')
            : record.fee_type === 1
            ? (record.fee / 100).toFixed(2) +
              ' - ' +
              (record.max_fee / 100).toFixed(2)
            : (record.fee / 100).toFixed(2)}
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'plugin.order.service.create-time' }),
      dataIndex: 'created_time',
      hideInSearch: true,
      render: (_, entity) => {
        return dayjs(entity.created_time * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            onClick={() => {
              handleEditService(record);
            }}
          >
            <FormattedMessage id="plugin.order.view" />
          </a>
          <a
            onClick={() => {
              handleDeleteService(record);
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
        headerTitle={intl.formatMessage({ id: 'menu.order.service' })}
        actionRef={actionRef}
        rowKey="id"
        search={false}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleEditService({ enabled: true });
            }}
          >
            <PlusOutlined /> <FormattedMessage id="plugin.order.service.add" />
          </Button>,
        ]}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetOrderServices(params);
        }}
        columnsState={{
          persistenceKey: 'order-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
      {editVisible && (
        <ModalForm
          title={intl.formatMessage({ id: 'plugin.order.service' })}
          width={800}
          open={editVisible}
          onOpenChange={(flag) => {
            setEditVisible(flag);
          }}
          initialValues={currentService}
          onFinish={saveOrderService}
        >
          <ProFormText
            name="service_name"
            label={intl.formatMessage({ id: 'plugin.order.service-name' })}
          />
          <ProFormText
            name="service_code"
            label={intl.formatMessage({ id: 'plugin.order.service-code' })}
          />
          <ProFormText
            name="service_type"
            label={intl.formatMessage({ id: 'plugin.order.service-type' })}
          />
          <ProFormTextArea
            name="description"
            label={intl.formatMessage({
              id: 'plugin.order.service.description',
            })}
          />
          <ProFormDigit
            name="sort"
            label={intl.formatMessage({ id: 'plugin.order.service-sort' })}
            extra={intl.formatMessage({
              id: 'plugin.order.service-sort.description',
            })}
          />
          <ProFormRadio.Group
            label={intl.formatMessage({ id: 'plugin.order.service-fee-type' })}
            name="fee_type"
            options={[
              {
                label: intl.formatMessage({
                  id: 'plugin.order.service-fee-type.fixed',
                }),
                value: 0,
              },
              {
                label: intl.formatMessage({
                  id: 'plugin.order.service-fee-type.percentage',
                }),
                value: 2,
              },
            ]}
            fieldProps={{
              onChange: (value) => {
                setFeeType(value.target.value);
              },
            }}
          />
          {feeType === 0 && (
            <ProFormDigit
              name="fee"
              label={intl.formatMessage({ id: 'plugin.order.service-fee' })}
              min={0}
              fieldProps={{
                precision: 2,
              }}
            />
          )}
          {feeType === 2 && (
            <ProFormDigit
              name="percent"
              label={intl.formatMessage({ id: 'plugin.order.service-fee' })}
              min={0}
              fieldProps={{
                precision: 0,
                suffix: '%',
              }}
            />
          )}
          {feeType === 2 && (
            <ProFormDigit
              name="max_fee"
              label={intl.formatMessage({
                id: 'plugin.order.service-fee-max',
              })}
              fieldProps={{
                precision: 2,
              }}
              extra={intl.formatMessage({
                id: 'plugin.order.service-fee-max.description',
              })}
            />
          )}
          <ProFormSwitch
            name="enabled"
            label={intl.formatMessage({ id: 'plugin.order.service-enabled' })}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default PluginOrderService;
