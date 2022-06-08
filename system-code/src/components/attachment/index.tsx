import React, { useEffect, useRef, useState } from 'react';
import { Button, message, Modal, Image, Avatar, Upload, Select, Space } from 'antd';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import ProForm, {
  ModalForm,
  ProFormRadio,
  ProFormText,
  ProFormTextArea,
} from '@ant-design/pro-form';
import { deleteArticleSetting, getArticleSetting, saveArticleSetting } from '@/services/archive';
import ProList from '@ant-design/pro-list';
import { getAttachmentCategories, getAttachments, uploadAttachment } from '@/services/attachment';
import './index.less'

export type AttachmentProps = {
  onSelect: (row?: any) => void;
  visible: boolean;
};

const AttachmentSelect: React.FC<AttachmentProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [visible, setVisible] = useState<boolean>(false);
  const [categories, setCategories] = useState<any[]>([]);
  const [categoryId, setCategoryId] = useState<number>(0);

  useEffect(() => {
    getAttachmentCategories().then(res => {
      setCategories(res.data || []);
    });
  }, []);

  const handleUploadImage = (e: any) => {
    let formData = new FormData();
    formData.append('file', e.file);
    uploadAttachment(formData).then((res) => {
      message.info(res.msg);
      actionRef.current?.reload();
    });
  };

  const handleChangeCategory = (e:any) => {
    setCategoryId(e)
    actionRef.current?.reload();
  }

  const useDetail = (row: any) => {
    props.onSelect(row);
    setVisible(false);
  }

  return (
    <>
      <div
        style={{display: 'inline-block'}}
        onClick={() => {
          setVisible(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        className='material-modal'
        width={800}
        title={<div>
          <Space size={16}>
              <span>选择图片</span>
                <Select
                  defaultValue={categoryId}
                  style={{ width: 120 }}
                  onChange={handleChangeCategory}
                >
                  <Select.Option value={0}>全部资源</Select.Option>
                  {categories.map((item: any, index) => (
                    <Select.Option key={item.id} value={item.id}>
                      {item.title}
                    </Select.Option>
                  ))}
                </Select>
                <Upload
                  name="file"
                  showUploadList={false}
                  accept=".jpg,.jpeg,.png,.gif"
                  customRequest={handleUploadImage}
                >
                  <Button type="primary">上传新图片</Button>
                </Upload>
              </Space>
        </div>}
        visible={visible}
        onCancel={() => {
          setVisible(false);
        }}
        onOk={() => {
          setVisible(false);
        }}
      >
        <ProList<any>
          actionRef={actionRef}
          className="material-table"
          rowKey="id"
          request={(params) => {
            params.category_id = categoryId;
            return getAttachments(params);
          }}
          grid={{ gutter: 16, column: 6 }}
          pagination={{
            defaultPageSize: 18,
          }}
          showActions="hover"
          showExtra="hover"
          search={false}
          rowClassName='image-row'
          metas={{
            content: {
              search: false,
              render: (text: any, row: any) => {
                return (
                  <div className="image-item">
                    <div className="inner">
                      {row.thumb ? (
                        <Image
                          className="img"
                          preview={{
                            src: row.logo,
                          }}
                          src={row.thumb}
                          alt={row.file_name}
                        />
                      ) : (
                        <Avatar className="default-img" size={100}>
                          {row.file_location.substring(row.file_location.lastIndexOf('.'))}
                        </Avatar>
                      )}
                      <div
                        className="info link"
                        onClick={() => {
                          useDetail(row);
                        }}
                      >
                        点击使用
                      </div>
                    </div>
                  </div>
                );
              },
            },
          }}
        />
      </Modal>
    </>
  );
};

export default AttachmentSelect;
