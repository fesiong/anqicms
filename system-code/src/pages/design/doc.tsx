import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import moment from 'moment';
import { Alert, Card, Input, Modal } from 'antd';
import './index.less';
import { getDesignDocs } from '@/services/design';
import 'prismjs/components/prism-clike';
import 'prismjs/components/prism-javascript';
import 'prismjs/components/prism-markup'
import 'prismjs/themes/prism.css';

const DesignDoc: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [templateDocs, setTemplateDocs] = useState<any[]>([]);
  const [currentDoc, setCurrentDoc] = useState<any>({});

  useEffect(() => {
    fetchTemplateDocs();
  }, []);

  const fetchTemplateDocs = () => {
    getDesignDocs().then(res => {
      setTemplateDocs(res.data)
    })
  }

  const handleShowDoc = (doc: any) => {
    Modal.info({
      title: doc.title,
      icon: false,
      width: 800,
      maskClosable: true,
      content: '',
    })
  }

  return (
    <PageContainer>
      <Card>
        <Alert message={<div>模板使用文档，请查看：<a href='https://www.kandaoni.com/category/10' target={'_blank'}>https://www.kandaoni.com/category/10</a></div>} />
      </Card>
    </PageContainer>
  );
};

export default DesignDoc;
