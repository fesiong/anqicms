import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { Alert, Card, Input, Modal } from 'antd';
import './index.less';
import { getDesignDocs } from '@/services/design';

const DesignDoc: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [templateDocs, setTemplateDocs] = useState<any[]>([]);
  const [currentDoc, setCurrentDoc] = useState<any>({});

  useEffect(() => {
    fetchTemplateDocs();
  }, []);

  const fetchTemplateDocs = () => {
    getDesignDocs().then(res => {
      setTemplateDocs(res.data || [])
    })
  }

  const handleShowDoc = (doc: any) => {
    Modal.confirm({
      title: doc.title,
      icon: false,
      width: 860,
      maskClosable: true,
      content: <div>
        <div dangerouslySetInnerHTML={{__html: `<iframe id="inlineFrameExample" style="border: 1px solid #e5e5e5" width="800" height="540" src=${doc.link}></iframe>`}}></div>
      </div>,
      onOk: (close) => {
        return close();
      },
      cancelText: '查看：' + doc.link,
      onCancel: (close) => {
        window.open(doc.link)
        close();
      }
    })
  }

  return (
    <PageContainer>
      <Card>
        <Alert message={<div>更详细的模板使用文档，请查看：<a href='https://www.kandaoni.com/category/14' target={'_blank'}>https://www.kandaoni.com/category/14</a></div>} />

        <div className='template-docs'>
        {templateDocs.map((item, index) => (
            <div className='group' key={index}>
              <div className='label'>{item.title}</div>
              <div className='content'>
                {item.docs?.map((doc: any, index2:number) => (
                  <a className='link item' key={index2} onClick={() => {
                    handleShowDoc(doc)
                  }}>{doc.title}</a>
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
