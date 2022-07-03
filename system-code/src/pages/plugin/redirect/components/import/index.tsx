import React, { useState } from 'react';
import { Alert, Button, Card, message, Upload } from 'antd';
import { ModalForm } from '@ant-design/pro-form';
import { exportFile } from '@/utils';
import { pluginImportRedirect } from '@/services/plugin/redirect';

export type RedirectImportProps = {
  onCancel: (flag?: boolean) => void;
};

const RedirectImport: React.FC<RedirectImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);

  const handleDownloadExample = () => {
    const header = ['from_url', 'to_url'];
    const content = [['/old.html', "/new.html"]];

    exportFile(header, content, 'csv');
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    const hide = message.loading('正在处理中', 0);
    pluginImportRedirect(formData).then((res) => {
      message.success(res.msg);
      setVisible(false);
      props.onCancel();
    }).finally(() => {
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
        title={'批量导入301链接'}
        visible={visible}
        modalProps={{
          onCancel: () => {
            setVisible(false);
          },
        }}
        layout="horizontal"
        onFinish={async (values) => {
          setVisible(false);
        }}
      >
        <Alert message={'说明：只支持csv格式的文件上传并导入'} />
        <div className="mt-normal">
          <Card size="small" title="第一步，下载csv模板文件" bordered={false}>
            <div className="text-center">
              <Button onClick={handleDownloadExample}>下载csv模板文件</Button>
            </div>
          </Card>
          <Card size="small" title="第二步，上传csv文件" bordered={false}>
            <div className="text-center">
              <Upload
                name="file"
                className="logo-uploader"
                showUploadList={false}
                accept=".csv"
                customRequest={handleUploadFile}
              >
                <Button type="primary">上传csv文件</Button>
              </Upload>
            </div>
          </Card>
        </div>
      </ModalForm>
    </>
  );
};

export default RedirectImport;
