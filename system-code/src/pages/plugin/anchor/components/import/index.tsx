import React, { useState } from 'react';
import { Alert, Button, Card, message, Upload } from 'antd';
import { ModalForm } from '@ant-design/pro-form';
import { exportFile } from '@/utils';
import { pluginImportAnchor } from '@/services/plugin/anchor';

export type AnchorImportProps = {
  onCancel: (flag?: boolean) => void;
};

const AnchorImport: React.FC<AnchorImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);

  const handleDownloadExample = () => {
    const header = ['title', 'link', 'weight'];
    const content = [['SEO', '/a/123.html', 9]];

    exportFile(header, content, 'csv');
  };

  const handleUploadFile = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    pluginImportAnchor(formData).then((res) => {
      message.success(res.msg);
      setVisible(false);
      props.onCancel();
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
        title={'批量导入锚文本'}
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

export default AnchorImport;
