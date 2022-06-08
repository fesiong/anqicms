import { Button, message, Input, Drawer, Modal, Space } from 'antd';
import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import {
  pluginGetSendmails,
  pluginGetSendmailSetting,
  pluginTestSendmail,
} from '@/services/plugin/sendmail';
import SendmailSetting from './components/setting';

const PluginSendmail: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [setting, setSetting] = useState<any>({});

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    let res = await pluginGetSendmailSetting();
    setSetting(res.data || {});
  };

  const handleSendTest = async () => {
    const hide = message.loading('发送测试邮件中', 0);

    let res = await pluginTestSendmail();
    actionRef?.current?.reload();
    hide();
    message.info(res.msg);
  };

  const columns: ProColumns<any>[] = [
    {
      title: '发送时间',
      width: 160,
      dataIndex: 'created_time',
      render: (text, record) => moment(record.created_time * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: '邮件标题',
      dataIndex: 'subject',
    },
    {
      title: '发送状态',
      width: 160,
      dataIndex: 'status',
    },
  ];

  return (
    <PageContainer>
      <ProTable<any>
        headerTitle="留言邮件提醒"
        rowKey="id"
        actionRef={actionRef}
        search={false}
        pagination={false}
        toolBarRender={() => [
          <div key="sender">
            <span>收件人: </span>
            <span>
              {setting.recipient || setting.account
                ? setting.recipient || setting.account
                : '请先进行邮件设置'}
            </span>
            {(setting.recipient || setting.account) && (
              <span>
                &nbsp;&nbsp;&nbsp;<Button onClick={() => handleSendTest()}>发送测试邮件</Button>
              </span>
            )}
          </div>,
          <SendmailSetting
            key="setting"
            onCancel={() => {
              getSetting();
            }}
          >
            <Button>邮件设置</Button>
          </SendmailSetting>,
        ]}
        request={(params, sort) => {
          return pluginGetSendmails(params);
        }}
        columns={columns}
      />
    </PageContainer>
  );
};

export default PluginSendmail;
