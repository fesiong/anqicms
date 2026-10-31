import {
  pluginDeleteTranslateTextLog,
  pluginGetMultiLangSites,
  pluginSaveTranslateTextLog,
  pluginTranslateTextLogs,
} from '@/services';
import { supportLanguages } from '@/utils';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, message, Modal, Space } from 'antd';
import dayjs from 'dayjs';
import React, { useRef, useState } from 'react';

export type TextLogProps = {
  open: boolean;
  onCancel: () => void;
};

const TranslateTextLog: React.FC<TextLogProps> = (props) => {
  const [currentItem, setCurrentItem] = useState<any>(null);
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const actionRef = useRef<ActionType>();
  const intl = useIntl();

  const handleRemoveItem = async (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.delete-confirm',
      }),
      onOk: () => {
        pluginDeleteTranslateTextLog(record)
          .then((res) => {
            message.success(res.msg);
            actionRef.current?.reload();
          })
          .catch((err) => {
            console.log(err);
          });
      },
    });
  };

  const handleRemoveAll = async () => {
    Modal.confirm({
      title: intl.formatMessage({
        id: 'plugin.multilang.html-cache.crean-all-confirm',
      }),
      onOk: () => {
        pluginDeleteTranslateTextLog({
          all: true,
        })
          .then((res) => {
            message.success(res.msg);
            actionRef.current?.reload();
          })
          .catch((err) => {
            console.log(err);
          });
      },
    });
  };

  const handleSubmit = async (values: any) => {
    let postValues = {
      text: values.text,
      to_language: values.to_language,
      translated: values.translated,
      id: currentItem.id || 0,
      language: currentItem.language || '',
    };

    pluginSaveTranslateTextLog(postValues)
      .then((res) => {
        message.success(res.msg);
        setEditVisible(false);
        actionRef.current?.reload();
      })
      .catch((err) => {
        console.log(err);
      });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-log.create-time',
      }),
      hideInSearch: true,
      dataIndex: 'created_time',
      render: (item) => {
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({
        id: 'plugin.multilang.html-log.to-language',
      }),
      hideInSearch: true,
      dataIndex: 'to_language',
      render: (text, record: any) => {
        return (
          supportLanguages.find((item) => item.value === record.to_language)
            ?.label || text
        );
      },
    },
    {
      title: intl.formatMessage({ id: 'content.translate.origin-content' }),
      dataIndex: 'text',
      width: 400,
    },
    {
      title: intl.formatMessage({ id: 'plugin.translate.result' }),
      dataIndex: 'translated',
      width: 400,
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      hideInSearch: true,
      dataIndex: 'option',
      valueType: 'option',
      render: (text: any, record) => (
        <Space size={20}>
          <a
            onClick={() => {
              setCurrentItem(record);
              setEditVisible(true);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            onClick={() => {
              handleRemoveItem(record);
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
      <Modal
        width={1200}
        title={intl.formatMessage({ id: 'plugin.multilang.text-log.manage' })}
        open={props.open}
        onCancel={props.onCancel}
        footer={null}
      >
        <ProTable<any>
          actionRef={actionRef}
          rowKey="id"
          ghost
          toolBarRender={() => [
            <Button
              type="primary"
              key="add"
              onClick={() => {
                setCurrentItem({});
                setEditVisible(true);
              }}
            >
              <FormattedMessage id="plugin.multilang.text-log.add" />
            </Button>,
            <Button
              key="update"
              onClick={() => {
                handleRemoveAll();
              }}
            >
              <FormattedMessage id="plugin.multilang.translate-cache.clear-all" />
            </Button>,
          ]}
          request={(params) => {
            return pluginTranslateTextLogs(params);
          }}
          columnsState={{
            persistenceKey: 'translate-text-log-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Modal>
      <ModalForm
        width={600}
        title={intl.formatMessage({ id: 'plugin.multilang.text-log.edit' })}
        open={editVisible}
        initialValues={currentItem}
        layout="horizontal"
        onOpenChange={(flag) => {
          setEditVisible(flag);
        }}
        onFinish={handleSubmit}
      >
        <ProFormText
          name="text"
          label={intl.formatMessage({ id: 'content.translate.origin-content' })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'plugin.multilang.text-log.text-required',
              }),
            },
          ]}
        />
        <ProFormSelect
          label={intl.formatMessage({
            id: 'plugin.multilang.select',
          })}
          showSearch
          name="to_language"
          request={async () => {
            const res = await pluginGetMultiLangSites({});
            const data = res.data.map((item: any) => {
              return {
                label: item.name + '(' + item.language + ')',
                value: item.language,
              };
            });
            return data;
          }}
        />
        <ProFormText
          name="translated"
          label={intl.formatMessage({ id: 'plugin.translate.result' })}
          rules={[
            {
              required: true,
              message: intl.formatMessage({
                id: 'plugin.multilang.text-log.translated-required',
              }),
            },
          ]}
        />
      </ModalForm>
    </>
  );
};

export default TranslateTextLog;
