import { pluginImportKeyword } from '@/services/plugin/keyword';
import { exportFile } from '@/utils';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, message, Upload } from 'antd';
import React, { useState } from 'react';

export type KeywordImportProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const KeywordImport: React.FC<KeywordImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDownloadExample = () => {
    const header = ['title', 'category_id'];
    const content = [['SEO', 1]];

    exportFile(header, content, 'csv');
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginImportKeyword(formData)
      .then((res) => {
        message.success(res.msg);
        setVisible(false);
        props.onCancel();
      })
      .finally(() => {
        hide();
      });
  };

  return (
    <>
      <div
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <ModalForm
        width={600}
        title={intl.formatMessage({ id: 'plugin.keyword.batch-import' })}
        open={visible}
        modalProps={{
          onCancel: () => {
            setVisible(false);
          },
        }}
        layout="horizontal"
        onFinish={async () => {
          setVisible(false);
        }}
      >
        <Alert message={intl.formatMessage({ id: 'plugin.keyword.batch-import.tips' })} />
        <div className="mt-normal">
          <Card size="small" title={intl.formatMessage({ id: 'plugin.keyword.batch-import.step1' })} bordered={false}>
            <div className="text-center">
              <Button onClick={handleDownloadExample}><FormattedMessage id="plugin.keyword.batch-import.step1.btn" /></Button>
            </div>
          </Card>
          <Card size="small" title={intl.formatMessage({ id: 'plugin.keyword.batch-import.step2' })} bordered={false}>
            <div className="text-center">
              <Upload
                name="file"
                className="logo-uploader"
                showUploadList={false}
                accept=".csv"
                customRequest={handleUploadFile}
              >
                <Button type="primary"><FormattedMessage id="plugin.keyword.batch-import.step2.btn" /></Button>
              </Upload>
            </div>
          </Card>
        </div>
      </ModalForm>
    </>
  );
};

export default KeywordImport;
