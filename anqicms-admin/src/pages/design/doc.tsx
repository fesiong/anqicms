import { getDesignDocs } from '@/services';
import { PageContainer } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Card, Modal } from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

const DesignDoc: React.FC = () => {
  const [templateDocs, setTemplateDocs] = useState<any[]>([]);
  const intl = useIntl();

  const fetchTemplateDocs = () => {
    getDesignDocs().then((res) => {
      setTemplateDocs(res.data || []);
    });
  };

  useEffect(() => {
    fetchTemplateDocs();
  }, []);

  const handleShowDoc = (doc: any) => {
    Modal.confirm({
      title: doc.title,
      icon: null,
      width: 860,
      maskClosable: true,
      content: (
        <div>
          <div
            dangerouslySetInnerHTML={{
              __html: `<iframe id="inlineFrameExample" style="border: 1px solid #e5e5e5" width="800" height="540" src=${doc.link}></iframe>`,
            }}
          ></div>
        </div>
      ),
      onOk: (close) => {
        return close();
      },
      cancelText: intl.formatMessage({ id: 'design.view' }) + ': ' + doc.link,
      onCancel: (close) => {
        window.open(doc.link);
        close();
      },
    });
  };

  return (
    <PageContainer>
      <Card>
        <Alert
          message={
            <div>
              <FormattedMessage id="design.doc.tips" />
              <a href="https://www.anqicms.com/manual" target="_blank" rel="noreferrer">
                https://www.anqicms.com/manual
              </a>
            </div>
          }
        />

        <div className="template-docs">
          {templateDocs.map((item, index) => (
            <div className="group" key={index}>
              <div className="label">{item.title}</div>
              <div className="content">
                {item.docs?.map((doc: any, index2: number) => (
                  <a
                    className="link item"
                    key={index2}
                    onClick={() => {
                      handleShowDoc(doc);
                    }}
                  >
                    {doc.title}
                  </a>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Card>
    </PageContainer>
  );
};

export default DesignDoc;
