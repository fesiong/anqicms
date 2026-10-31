import { anqiSendFeedback } from '@/services';
import { FormOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProFormRadio,
  ProFormTextArea,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl, useModel } from '@umijs/max';
import { FloatButton, message } from 'antd';
import { useState } from 'react';
import AiChat from '../aiChat';
import './index.less';

let loading = false;
const Footer: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const [visible, setVisible] = useState<boolean>(false);
  const [aiChatVisible, setAiChatVisible] = useState<boolean>(false);
  const currentYear = new Date().getFullYear();
  const intl = useIntl();

  const isLogin = initialState?.currentUser?.id > 0;
  const handleFeedback = async (values: any) => {
    if (loading) {
      return;
    }
    loading = true;
    const postData = Object.assign({}, values);
    const hide = message.loading(
      intl.formatMessage({ id: 'component.footer.submitting' }),
      0,
    );
    anqiSendFeedback(postData)
      .then((res) => {
        if (res.code === 0) {
          message.info(
            res.msg || intl.formatMessage({ id: 'component.footer.submitted' }),
          );
          setVisible(false);
        } else {
          message.info(res.msg);
        }
      })
      .finally(() => {
        loading = false;
        hide();
      });
  };

  const handleOpenAiChat = () => {
    setAiChatVisible(true);
  };

  return (
    <div className="anqi-footer">
      <span>
        © {currentYear} <FormattedMessage id="component.footer.copyright" />
      </span>
      <div>
        <a href="https://www.anqicms.com/help" target="_blank" rel="noreferrer">
          <FormattedMessage id="component.footer.help" />
        </a>
        <span>|</span>
        <a
          href="https://www.anqicms.com/manual"
          target="_blank"
          rel="noreferrer"
        >
          <FormattedMessage id="component.footer.template-manual" />
        </a>
        <span>|</span>
        <a
          className="feedback link"
          onClick={() => {
            setVisible(true);
          }}
        >
          <FormattedMessage id="component.footer.feedback" />
        </a>
      </div>
      <FloatButton.Group shape="square" style={{ insetInlineEnd: 10 }}>
        {isLogin && (
          <FloatButton
            icon={
              <svg
                className="ai-icon"
                viewBox="0 0 1024 1024"
                version="1.1"
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
              >
                <path
                  d="M826.368 325.632c0-7.168 2.048-10.24 10.24-10.24h123.904c7.168 0 10.24 2.048 10.24 10.24v621.568c0 7.168-2.048 10.24-10.24 10.24h-122.88c-8.192 0-10.24-4.096-10.24-10.24l-1.024-621.568z m-8.192-178.176c0-50.176 35.84-79.872 79.872-79.872 48.128 0 79.872 32.768 79.872 79.872 0 52.224-33.792 79.872-81.92 79.872-46.08 1.024-77.824-27.648-77.824-79.872zM462.848 584.704C441.344 497.664 389.12 307.2 368.64 215.04h-2.048c-16.384 92.16-58.368 247.808-92.16 369.664h188.416zM243.712 712.704l-62.464 236.544c-2.048 7.168-4.096 8.192-12.288 8.192H54.272c-8.192 0-10.24-2.048-8.192-12.288l224.256-783.36c4.096-13.312 7.168-26.624 8.192-65.536 0-6.144 2.048-8.192 7.168-8.192H450.56c6.144 0 8.192 2.048 10.24 8.192l250.88 849.92c2.048 7.168 0 10.24-7.168 10.24H573.44c-7.168 0-10.24-2.048-12.288-7.168l-65.536-236.544c1.024 1.024-251.904 0-251.904 0z"
                  fill="#ffffff"
                ></path>
              </svg>
            }
            type="primary"
            tooltip={intl.formatMessage({ id: 'component.ai-chat.title' })}
            onClick={handleOpenAiChat}
          />
        )}
        <FloatButton
          onClick={() => {
            setVisible(true);
          }}
          icon={<FormOutlined />}
        />
        <FloatButton.BackTop visibilityHeight={1000} />
      </FloatButton.Group>
      <AiChat visible={aiChatVisible} onClose={() => setAiChatVisible(false)} />
      <ModalForm
        open={visible}
        width={600}
        title={intl.formatMessage({ id: 'component.footer.feedback' })}
        layout="horizontal"
        onOpenChange={(flag) => {
          setVisible(flag);
        }}
        onFinish={handleFeedback}
      >
        <ProFormRadio.Group
          name="type"
          required
          width="lg"
          label={intl.formatMessage({ id: 'component.footer.feedback.type' })}
          valueEnum={{
            bug: intl.formatMessage({ id: 'component.footer.feedback.bug' }),
            suggest: intl.formatMessage({
              id: 'component.footer.feedback.suggest',
            }),
            consult: intl.formatMessage({
              id: 'component.footer.feedback.consult',
            }),
          }}
        />
        <ProFormTextArea
          name="content"
          required
          label={intl.formatMessage({
            id: 'component.footer.feedback.description',
          })}
          width="lg"
          fieldProps={{
            rows: 6,
          }}
        />
      </ModalForm>
    </div>
  );
};

export default Footer;
