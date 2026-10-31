import {
  pluginDeleteSubscriberCategory,
  pluginGetSubscriberCategories,
  pluginSaveSubscriberCategory,
} from '@/services/plugin/subscriber';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Input, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

export type AttachmentCategoryProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const SubscriptionCategory: React.FC<AttachmentCategoryProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [editVisbile, setEditVisible] = useState<boolean>(false);
  const [editingCategory, setEditingCategory] = useState<any>({});
  const [editingInput, setEditingInput] = useState<string>('');
  const intl = useIntl();

  const handleAddCategory = () => {
    setEditingCategory({});
    setEditingInput('');
    setEditVisible(true);
  };

  const handleEditCategory = (record: any) => {
    setEditingCategory(record);
    setEditingInput(record.title);
    setEditVisible(true);
  };

  const handleRemove = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.subscription.category.delete.confirm',
      }),
      onOk: async () => {
        let res = await pluginDeleteSubscriberCategory(record);

        message.info(res.msg);
        actionRef.current?.reloadAndRest?.();
      },
    });
  };

  const handleSaveCategory = () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSaveSubscriberCategory({
      id: editingCategory.id,
      title: editingInput,
    })
      .then((res) => {
        if (res.code === 0) {
          setEditVisible(false);

          actionRef.current?.reloadAndRest?.();
        } else {
          message.error(res.msg);
        }
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: 'ID',
      dataIndex: 'id',
      width: 60,
    },
    {
      title: intl.formatMessage({ id: 'content.category.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.subscription.count' }),
      dataIndex: 'attach_count',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 120,
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleEditCategory(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemove(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        open={visible}
        title={
          <Button
            type="primary"
            onClick={() => {
              handleAddCategory();
            }}
          >
            <FormattedMessage id="content.category.new" />
          </Button>
        }
        width={600}
        onCancel={() => {
          setVisible(false);
          props.onCancel();
        }}
        footer={false}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <ProTable<any>
            headerTitle={intl.formatMessage({
              id: 'plugin.subscription.category.manage',
            })}
            actionRef={actionRef}
            rowKey="id"
            search={false}
            toolBarRender={false}
            request={(params) => {
              return pluginGetSubscriberCategories(params);
            }}
            columnsState={{
              persistenceKey: 'attachment-categories-table',
              persistenceType: 'localStorage',
            }}
            columns={columns}
            pagination={{
              showSizeChanger: true,
            }}
          />
        </div>
      </Modal>
      <Modal
        open={editVisbile}
        title={
          editingCategory.id
            ? intl.formatMessage({ id: 'content.category.rename' }) +
              ':' +
              editingCategory.title
            : intl.formatMessage({ id: 'content.category.new' })
        }
        width={480}
        zIndex={2000}
        maskClosable={false}
        onOk={handleSaveCategory}
        onCancel={() => {
          setEditVisible(false);
        }}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <div>
            <FormattedMessage id="content.attachment.alt.alert" />:{' '}
          </div>
          <Input
            size="large"
            value={editingInput}
            onChange={(e) => {
              setEditingInput(e.target.value);
            }}
          />
        </div>
      </Modal>
    </>
  );
};

export default SubscriptionCategory;
