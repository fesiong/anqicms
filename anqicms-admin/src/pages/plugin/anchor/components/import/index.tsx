import { pluginImportAnchor } from '@/services/plugin/anchor';
import { exportFile } from '@/utils';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, Upload, message } from 'antd';
import React, { useState } from 'react';

export type AnchorImportProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const AnchorImport: React.FC<AnchorImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDownloadExample = () => {
    const header = ['title', 'link', 'weight'];
    const content = [['SEO', '/a/123.html', 9]];

    exportFile(header, content, 'csv');
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginImportAnchor(formData)
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
        title={intl.formatMessage({ id: 'plugin.anchor.import' })}
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
        <Alert message={intl.formatMessage({ id: 'plugin.anchor.import.description' })} />
        <div className="mt-normal">
          <Card
            size="small"
            title={intl.formatMessage({ id: 'plugin.anchor.step1' })}
            bordered={false}
          >
            <div className="text-center">
              <Button onClick={handleDownloadExample}>
                <FormattedMessage id="plugin.anchor.step1.download" />
              </Button>
            </div>
          </Card>
          <Card
            size="small"
            title={intl.formatMessage({ id: 'plugin.anchor.step2' })}
            bordered={false}
          >
            <div className="text-center">
              <Upload
                name="file"
                className="logo-uploader"
                showUploadList={false}
                accept=".csv"
                customRequest={handleUploadFile}
              >
                <Button type="primary">
                  <FormattedMessage id="plugin.anchor.step2.upload" />
                </Button>
              </Upload>
            </div>
          </Card>
        </div>
      </ModalForm>
    </>
  );
};

export default AnchorImport;
