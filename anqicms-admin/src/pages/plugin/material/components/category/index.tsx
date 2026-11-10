import {
  pluginDeleteMaterialCategory,
  pluginGetMaterialCategories,
  pluginSaveMaterialCategory,
} from '@/services/plugin/material';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Input, Modal, Space, message } from 'antd';
import React, { useRef, useState } from 'react';

export type MaterialCategoryProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const MaterialCategory: React.FC<MaterialCategoryProps> = (props) => {
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
      title: intl.formatMessage({ id: 'plugin.material.category.delete.confirm' }),
      onOk: async () => {
        let res = await pluginDeleteMaterialCategory(record);

        message.info(res.msg);
        actionRef.current?.reloadAndRest?.();
      },
    });
  };

  const handleSaveCategory = () => {
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginSaveMaterialCategory({
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
      title: intl.formatMessage({ id: 'plugin.material.category.title' }),
      dataIndex: 'title',
    },
    {
      title: intl.formatMessage({ id: 'plugin.material.category.count' }),
      dataIndex: 'material_count',
      width: 80,
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
            <FormattedMessage id="plugin.material.category.add" />
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
            headerTitle={intl.formatMessage({ id: 'plugin.material.category.manage' })}
            actionRef={actionRef}
            rowKey="id"
            search={false}
            pagination={false}
            toolBarRender={false}
            request={(params) => {
              return pluginGetMaterialCategories(params);
            }}
            columnsState={{
              persistenceKey: 'material-category-table',
              persistenceType: 'localStorage',
            }}
            columns={columns}
          />
        </div>
      </Modal>
      <Modal
        open={editVisbile}
        title={editingCategory.id ? intl.formatMessage({ id: 'plugin.material.category.edit' }) + editingCategory.title : intl.formatMessage({ id: 'plugin.material.category.add' })}
        width={480}
        zIndex={2000}
        maskClosable={false}
        onOk={handleSaveCategory}
        onCancel={() => {
          setEditVisible(false);
        }}
      >
        <div style={{ marginTop: '20px', marginBottom: '20px' }}>
          <p><FormattedMessage id="plugin.material.category.title.tips" />: </p>
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

export default MaterialCategory;
