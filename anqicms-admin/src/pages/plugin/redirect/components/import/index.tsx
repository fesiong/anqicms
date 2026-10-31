import { pluginImportRedirect } from '@/services/plugin/redirect';
import { exportFile } from '@/utils';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import { Alert, Button, Card, message, Upload } from 'antd';
import React, { useState } from 'react';

export type RedirectImportProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const RedirectImport: React.FC<RedirectImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const intl = useIntl();

  const handleDownloadExample = () => {
    const header = ['from_url', 'to_url'];
    const content = [['/old.html', '/new.html']];

    exportFile(header, content, 'csv');
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    pluginImportRedirect(formData)
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
        title={intl.formatMessage({ id: 'plugin.redirect.import' })}
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
        <Alert message={intl.formatMessage({ id: 'plugin.redirect.import.tips' })} />
        <div className="mt-normal">
          <Card size="small" title={intl.formatMessage({ id: 'plugin.redirect.import.step1' })} bordered={false}>
            <div className="text-center">
              <Button onClick={handleDownloadExample}><FormattedMessage id="plugin.redirect.import.step1.download" /></Button>
            </div>
          </Card>
          <Card size="small" title={intl.formatMessage({ id: 'plugin.redirect.import.step2' })} bordered={false}>
            <div className="text-center">
              <Upload
                name="file"
                className="logo-uploader"
                showUploadList={false}
                accept=".csv"
                customRequest={handleUploadFile}
              >
                <Button type="primary"><FormattedMessage id="plugin.redirect.import.step2.upload" /></Button>
              </Upload>
            </div>
          </Card>
        </div>
      </ModalForm>
    </>
  );
};

export default RedirectImport;
