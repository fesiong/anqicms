import NewContainer from '@/components/NewContainer';
import {
  pluginGetPush,
  pluginGetPushLogs,
  pluginSavePush,
} from '@/services/plugin/push';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProForm,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Modal, Space, message } from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

const PluginPush: React.FC<any> = () => {
  const actionRef = useRef<ActionType>();
  const [pushSetting, setPushSetting] = useState<any>({});
  const [jsCodes, setJsCodes] = useState<any[]>([]);
  const [fetched, setFetched] = useState<boolean>(false);
  const [logVisible, setLogVisible] = useState<boolean>(false);
  const [editCodeVisible, setEditCodeVisible] = useState<boolean>(false);
  const [currentIndex, setCurrentIndex] = useState<number>(-1);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetPush();
    let setting = res.data || {};
    setPushSetting(setting);
    setJsCodes(setting.js_codes || []);
    setFetched(true);
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const onSubmit = async (data: any) => {
    let values = Object.assign(pushSetting, data);
    pushSetting.js_codes = jsCodes;
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    pluginSavePush(values)
      .then((res) => {
        message.success(res.msg);
      })
      .catch((err) => {
        console.log(err);
      })
      .finally(() => {
        hide();
      });
  };

  const handleShowAddJs = () => {
    let index = jsCodes.push({ name: '', value: '' }) - 1;
    setCurrentIndex(index);
    setJsCodes([].concat(...jsCodes));
    setEditCodeVisible(true);
  };

  const handleEditJs = (row: any, index: number) => {
    setCurrentIndex(index);
    setEditCodeVisible(true);
  };

  const handleSaveEditJs = async (values: any) => {
    jsCodes[currentIndex] = values;
    setJsCodes([].concat(...jsCodes));
    setEditCodeVisible(false);
    actionRef.current?.reloadAndRest?.();
  };

  const handleRemoveJs = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'setting.system.confirm-delete' }),
      onOk: async () => {
        jsCodes.splice(index, 1);
        setJsCodes([].concat(...jsCodes));
        actionRef.current?.reloadAndRest?.();
      },
    });
  };

  const handleShowPushLog = () => {
    setLogVisible(true);
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.aigenerate.time' }),
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) =>
        dayjs(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'plugin.push.engine' }),
      width: 160,
      dataIndex: 'spider',
    },
    {
      title: intl.formatMessage({ id: 'plugin.push.result' }),
      dataIndex: 'result',
    },
  ];

  const jsColumns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.push.name' }),
      width: 200,
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.push.code' }),
      dataIndex: 'value',
      render: (text) => (
        <div
          style={{
            maxHeight: 60,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            wordBreak: 'break-all',
          }}
        >
          {text}
        </div>
      ),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      width: 150,
      render: (_, record, index) => (
        <Space size={20}>
          <a
            key="check"
            onClick={() => {
              handleEditJs(record, index);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </a>
          <a
            className="text-red"
            key="delete"
            onClick={() => {
              handleRemoveJs(index);
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
      <Card key={newKey}>
        <Alert
          message={
            <div>
              <span>
                <FormattedMessage id="plugin.push.tips" />
              </span>
              <Button size="small" onClick={handleShowPushLog}>
                <FormattedMessage id="plugin.push.view-log" />
              </Button>
            </div>
          }
        />
        <div className="mt-normal">
          {fetched && (
            <ProForm onFinish={onSubmit} initialValues={pushSetting}>
              <Card
                size="small"
                title={intl.formatMessage({ id: 'plugin.push.baidu' })}
                bordered={false}
              >
                <ProFormText
                  name="baidu_api"
                  label={intl.formatMessage({ id: 'plugin.push.api-link' })}
                  extra={intl.formatMessage({
                    id: 'plugin.push.baidu.api-link.description',
                  })}
                />
              </Card>
              <Card
                size="small"
                title={intl.formatMessage({ id: 'plugin.push.bing' })}
                bordered={false}
              >
                <ProFormText
                  name="bing_api"
                  label={intl.formatMessage({ id: 'plugin.push.api-link' })}
                  extra={intl.formatMessage({
                    id: 'plugin.push.bing.api-link.description',
                  })}
                />
              </Card>
              <Card
                size="small"
                title={intl.formatMessage({ id: 'plugin.push.google' })}
                bordered={false}
              >
                <ProFormTextArea
                  name="google_json"
                  label={intl.formatMessage({ id: 'plugin.push.google.json' })}
                  fieldProps={{
                    rows: 5,
                  }}
                  extra={intl.formatMessage({
                    id: 'plugin.push.google.description',
                  })}
                />
              </Card>
              <Card
                size="small"
                title={intl.formatMessage({ id: 'plugin.push.other-js' })}
                bordered={false}
              >
                <ProTable<any>
                  actionRef={actionRef}
                  rowKey="name"
                  search={false}
                  toolBarRender={() => [
                    <Button
                      key="add"
                      onClick={() => {
                        handleShowAddJs();
                      }}
                    >
                      <FormattedMessage id="plugin.push.other-js.add" />
                    </Button>,
                  ]}
                  tableAlertOptionRender={false}
                  request={async () => {
                    return {
                      data: jsCodes,
                    };
                  }}
                  columnsState={{
                    persistenceKey: 'push-jscode-table',
                    persistenceType: 'localStorage',
                  }}
                  columns={jsColumns}
                  pagination={false}
                />
                <div>
                  <p>
                    <FormattedMessage id="plugin.push.other-js.tips1" />
                  </p>
                  <p>
                    <FormattedMessage id="plugin.push.other-js.tips2" />
                  </p>
                  <p>
                    <FormattedMessage id="plugin.push.other-js.tips3" />
                  </p>
                </div>
              </Card>
            </ProForm>
          )}
        </div>
      </Card>
      <Modal
        title={intl.formatMessage({ id: 'plugin.push.view-log' })}
        width={900}
        open={logVisible}
        onCancel={() => {
          setLogVisible(false);
        }}
        onOk={() => {
          setLogVisible(false);
        }}
      >
        <ProTable<any>
          rowKey="id"
          search={false}
          toolBarRender={false}
          request={(params) => {
            return pluginGetPushLogs(params);
          }}
          columnsState={{
            persistenceKey: 'push-log-table',
            persistenceType: 'localStorage',
          }}
          columns={columns}
          pagination={{
            showSizeChanger: true,
            showQuickJumper: true,
          }}
        />
      </Modal>
      {editCodeVisible && (
        <ModalForm
          open={editCodeVisible}
          onFinish={handleSaveEditJs}
          onOpenChange={(flag) => {
            setEditCodeVisible(flag);
          }}
          initialValues={jsCodes[currentIndex]}
        >
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'plugin.push.other-js.name' })}
            placeholder={intl.formatMessage({
              id: 'plugin.push.other-js.name.placeholder',
            })}
          />
          <ProFormTextArea
            name="value"
            label={intl.formatMessage({ id: 'plugin.push.other-js.code' })}
            extra={intl.formatMessage({
              id: 'plugin.push.other-js.code.placeholder',
            })}
            fieldProps={{
              rows: 8,
            }}
          />
        </ModalForm>
      )}
    </NewContainer>
  );
};

export default PluginPush;
