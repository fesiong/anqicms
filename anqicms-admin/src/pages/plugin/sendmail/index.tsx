import NewContainer from '@/components/NewContainer';
import {
  pluginGetSendmailSetting,
  pluginGetSendmailTemplates,
  pluginTestSendmail,
} from '@/services';
import { CheckCircleOutlined } from '@ant-design/icons';
import { ActionType } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Button, Card, Col, Row, Space, message } from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import PluginSendMailLogs from './components/logs';
import SendmailSetting from './components/setting';
import EmailTemplateForm from './components/templateForm';
import { EmailTemplate } from './components/templates';
import './index.less';

const PluginSendmail: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [setting, setSetting] = useState<any>({});
  const [settingVisible, setSettingVisible] = useState<boolean>(false);
  const [logsVisible, setLogsVisible] = useState<boolean>(false);
  const [templates, setTemplates] = useState<EmailTemplate[]>([]);
  const [currentTemplate, setCurrentTemplate] =
    useState<EmailTemplate | null>();
  const [templateVisible, setTemplateVisible] = useState<boolean>(false);
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const getSetting = async () => {
    pluginGetSendmailSetting().then((res) => {
      setSetting(res.data || {});
    });
    pluginGetSendmailTemplates().then((res) => {
      setTemplates(res.data || []);
    });
  };

  const onTabChange = (key: string) => {
    getSetting().then(() => {
      setNewKey(key);
    });
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleSendTest = async () => {
    const hide = message.loading(
      intl.formatMessage({ id: 'plugin.sendmail.test.sending' }),
      0,
    );

    let res = await pluginTestSendmail();
    actionRef?.current?.reload();
    hide();
    message.info(res.msg);
  };

  const handleEditTemplate = (template: EmailTemplate) => {
    setCurrentTemplate(template);
    setTemplateVisible(true);
  };

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <Card
        key={newKey}
        title={intl.formatMessage({ id: 'menu.promotion.automation' })}
        extra={
          <Space>
            <Button onClick={() => setLogsVisible(true)}>
              <FormattedMessage id="plugin.sendmail.logs" />
            </Button>
            <Button onClick={() => setSettingVisible(true)}>
              <FormattedMessage id="plugin.sendmail.setting" />
            </Button>
            <div key="sender">
              <span>
                <FormattedMessage id="plugin.sendmail.recipient" />:{' '}
              </span>
              <span>
                {setting.recipient || setting.account
                  ? setting.recipient || setting.account
                  : intl.formatMessage({
                      id: 'plugin.sendmail.recipient.required',
                    })}
              </span>
              {(setting.recipient || setting.account) && (
                <span>
                  &nbsp;&nbsp;&nbsp;
                  <Button onClick={() => handleSendTest()}>
                    <FormattedMessage id="plugin.sendmail.test.send" />
                  </Button>
                </span>
              )}
            </div>
          </Space>
        }
      >
        <Row gutter={[16, 16]}>
          {templates.map((item) => (
            <Col key={item.key} sm={8} xs={24}>
              <Card
                className={`template-card ${item.open ? 'open' : ''}`}
                onClick={() => handleEditTemplate(item)}
              >
                <h3 className="template-card-header">
                  <span>{item.name}</span>
                  <span className="template-open-icon">
                    <CheckCircleOutlined />
                  </span>
                </h3>
                <div>{item.description}</div>
              </Card>
            </Col>
          ))}
        </Row>
      </Card>
      {logsVisible && (
        <PluginSendMailLogs
          open={logsVisible}
          onCancel={() => {
            setLogsVisible(false);
          }}
        />
      )}
      {settingVisible && (
        <SendmailSetting
          open={settingVisible}
          onCancel={() => {
            setSettingVisible(false);
            getSetting();
          }}
        />
      )}
      {templateVisible && (
        <EmailTemplateForm
          open={templateVisible}
          template={currentTemplate || {}}
          onCancel={() => {
            setTemplateVisible(false);
          }}
          onSubmit={() => {
            setTemplateVisible(false);
            getSetting();
          }}
        />
      )}
    </NewContainer>
  );
};

export default PluginSendmail;
