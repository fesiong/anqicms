import { getCategories } from '@/services';
import { getAttachments, uploadAttachment } from '@/services/attachment';
import { acceptedExtensions, calculateFileMd5 } from '@/utils';
import { CloseOutlined } from '@ant-design/icons';
import { ActionType, ProList } from '@ant-design/pro-components';
import {
  Avatar,
  Button,
  Checkbox,
  Image,
  Input,
  Select,
  Space,
  Upload,
  message,
} from 'antd';
import React, { useEffect, useRef, useState } from 'react';
import AiImageGenerate from '../aiimage';
import AttachmentAddUrl from './addUrl';
import './index.less';

export type AttachmentContentProps = {
  onSelect: (row?: any) => void;
  onCancel?: () => void;
  style?: React.CSSProperties;
  className?: string;
  multiple?: boolean;
  intl?: any;
};

const AttachmentContent: React.FC<AttachmentContentProps> = (props) => {
  const actionRef = useRef<ActionType>();
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [categoryId, setCategoryId] = useState<number>(0);
  const [keyword, setKeyword] = useState<string>('');
  const [aiVisible, setAiVisible] = useState<boolean>(false);
  const [addUrlVisible, setAddUrlVisible] = useState<boolean>(false);

  useEffect(() => {
    getCategories().then((res) => {
      setCategories(res.data || []);
    });
  }, []);

  const handleAddUrl = () => {
    setAddUrlVisible(true);
  };
  const handleUploadImage = async (e: any) => {
    // 上传前先计算MD5
    let hide = message.loading({
      content:
        props.intl?.formatMessage({ id: 'component.footer.submitting' }) ||
        '上传中...',
      duration: 0,
      key: 'uploading',
    });
    const size = e.file.size;
    const md5Value = await calculateFileMd5(e.file);
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const totalChunks = Math.ceil(size / chunkSize);
    let formData = new FormData();
    formData.append('category_id', categoryId + '');
    formData.append('file_name', e.file.name);
    formData.append('md5', md5Value as string);
    if (totalChunks > 1) {
      // 大于 chunkSize 的，使用分片上传
      formData.append('chunks', totalChunks + '');
      for (let i = 0; i < totalChunks; i++) {
        const chunk = e.file.slice(i * chunkSize, (i + 1) * chunkSize);
        chunk.name = e.file.name;
        chunk.uid = e.file.uid;
        formData.set('chunk', i + '');
        formData.set('file', chunk);
        try {
          const res = await uploadAttachment(formData);
          if (res.code !== 0) {
            message.info(res.msg);
            hide();
          } else {
            hide = message.loading({
              content:
                (props.intl?.formatMessage({
                  id: 'component.footer.submitting',
                }) || '正在上传') +
                ' - ' +
                Math.ceil(((i + 1) * 100) / totalChunks) +
                '%',
              duration: 0,
              key: 'uploading',
            });
            if (res.data) {
              // 上传完成
              hide();
              message.info(
                res.msg ||
                  props.intl?.formatMessage({
                    id: 'component.footer.uploaded',
                  }) ||
                  '上传成功',
              );
              if (props.multiple) {
                setSelectedRowKeys([]);
                props.onSelect([]);
              }
              actionRef.current?.reload();
            }
          }
        } catch (err) {
          hide();
          message.info('upload failed');
        }
      }
    } else {
      // 小于 chunkSize 的，直接上传
      formData.append('file', e.file);
      uploadAttachment(formData)
        .then((res) => {
          if (res.code !== 0) {
            message.info(res.msg);
          } else {
            message.info(
              res.msg ||
                props.intl?.formatMessage({
                  id: 'component.footer.uploaded',
                }) ||
                '上传成功',
            );
            if (props.multiple) {
              setSelectedRowKeys([]);
              props.onSelect([]);
            }
            actionRef.current?.reload();
          }
        })
        .finally(() => {
          hide();
        });
    }
  };

  const handleGenerateImage = async () => {
    setAiVisible(true);
  };

  const handleChangeCategory = (e: any) => {
    setCategoryId(e);
    if (props.multiple) {
      setSelectedRowKeys([]);
      props.onSelect([]);
    }
    actionRef.current?.reload();
  };

  const setDetail = (row: any) => {
    if (props.multiple) {
      let exist = false;
      let tmpKeys = [];
      for (let i in selectedRowKeys) {
        if (selectedRowKeys[i].id === row.id) {
          exist = true;
        } else {
          tmpKeys.push(selectedRowKeys[i]);
        }
      }
      if (!exist) {
        tmpKeys.push(row);
      }
      setSelectedRowKeys(tmpKeys);
      props.onSelect(tmpKeys);
    } else {
      props.onSelect(row);
    }
  };

  const handleSearch = (kw: any) => {
    setKeyword(kw);
    if (props.multiple) {
      setSelectedRowKeys([]);
      props.onSelect([]);
    }
    actionRef.current?.reload();
  };

  const onChangeSelect = (e: any) => {
    setSelectedRowKeys(e);
    props.onSelect(e);
  };

  const handleSubmitAi = () => {
    actionRef.current?.reloadAndRest?.();
    setAiVisible(false);
  };

  const handleSubmitAddUrl = () => {
    actionRef.current?.reloadAndRest?.();
    setAddUrlVisible(false);
  };

  return (
    <div style={props.style} className={props.className}>
      <div className="material-header">
        <Space size={16}>
          <Select
            defaultValue={categoryId}
            style={{ width: 180 }}
            onChange={handleChangeCategory}
          >
            <Select.Option value={0}>
              {props.intl?.formatMessage({ id: 'component.attachment.all' }) ||
                '全部资源'}
            </Select.Option>
            {categories.map((item: any) => (
              <Select.Option key={item.id} value={item.id}>
                {item.parents?.length > 0 ? (
                  <span className="text-muted">
                    {item.parents
                      ?.map((parent: any) => parent.title)
                      .join(' > ')}
                    {' > '}
                  </span>
                ) : (
                  ''
                )}
                {item.title}
              </Select.Option>
            ))}
          </Select>
          <Input.Search
            placeholder={
              props.intl?.formatMessage({
                id: 'component.attachment.search.placeholder',
              }) || '输入文件名关键词搜索'
            }
            onSearch={handleSearch}
          />
          <Upload
            name="file"
            showUploadList={false}
            multiple
            accept={acceptedExtensions}
            customRequest={handleUploadImage}
          >
            <Button type="primary">
              {props.intl?.formatMessage({
                id: 'component.attachment.upload',
              }) || '上传新文件'}
            </Button>
          </Upload>
          <Button onClick={() => handleAddUrl()}>
            {props.intl?.formatMessage({ id: 'content.attachment.add-url' }) ||
              '添加远程资源'}
          </Button>
          <Button type="default" onClick={handleGenerateImage}>
            {props.intl?.formatMessage({
              id: 'component.aiimage.generate',
            }) || 'AI生成图片'}
          </Button>
          {selectedRowKeys.length > 0 && (
            <span>
              {props.intl?.formatMessage({
                id: 'component.attachment.select.selected',
              }) || '已选'}
              {selectedRowKeys.length}
              {props.intl?.formatMessage({
                id: 'component.attachment.select.selected.suffix',
              }) || '个'}
            </span>
          )}
        </Space>
        {typeof props.onCancel === 'function' && (
          <a className="close" onClick={() => props.onCancel?.()}>
            <CloseOutlined />
          </a>
        )}
      </div>
      <Checkbox.Group
        onChange={onChangeSelect}
        value={selectedRowKeys}
        style={{ display: 'block' }}
      >
        <ProList<any>
          actionRef={actionRef}
          className="material-table"
          rowKey="id"
          request={(params) => {
            params.category_id = categoryId;
            params.q = keyword;
            return getAttachments(params);
          }}
          grid={{ gutter: 16, column: 6 }}
          pagination={{
            defaultPageSize: 18,
          }}
          showActions="hover"
          showExtra="hover"
          search={false}
          rowClassName="image-row"
          metas={{
            content: {
              search: false,
              render: (_: any, row: any) => {
                return (
                  <div className="image-item">
                    {props.multiple && (
                      <Checkbox className="checkbox" value={row} />
                    )}
                    <div className="inner" title={row.file_name}>
                      {row.thumb ? (
                        <Image
                          className="img"
                          preview={{
                            src: row.file_path,
                          }}
                          src={row.thumb}
                          alt={row.file_name}
                        />
                      ) : (
                        <a
                          href={row.file_path}
                          target="_blank"
                          rel="noreferrer"
                        >
                          <Avatar className="default-img" size={100}>
                            {row.file_location.substring(
                              row.file_location.lastIndexOf('.'),
                            )}
                          </Avatar>
                        </a>
                      )}
                      <div className="info name">{row.file_name}</div>
                      <div
                        className="info link"
                        onClick={() => {
                          setDetail(row);
                        }}
                      >
                        {props.intl?.formatMessage({
                          id: 'component.attachment.use',
                        }) || '点击使用'}
                      </div>
                    </div>
                  </div>
                );
              },
            },
          }}
        />
      </Checkbox.Group>
      {aiVisible && (
        <AiImageGenerate
          onCancel={() => setAiVisible(false)}
          onSubmit={handleSubmitAi}
          open={aiVisible}
          intl={props.intl}
        />
      )}
      {addUrlVisible && (
        <AttachmentAddUrl
          onCancel={() => setAddUrlVisible(false)}
          onSubmit={handleSubmitAddUrl}
          open={addUrlVisible}
          intl={props.intl}
        />
      )}
    </div>
  );
};

export default AttachmentContent;
