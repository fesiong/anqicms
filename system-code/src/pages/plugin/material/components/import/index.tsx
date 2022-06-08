import React, { useEffect, useState } from 'react';
import { Alert, Button, Card, Col, Input, message, Modal, Row, Select, Space, Upload } from 'antd';
import { ModalForm, ProFormSelect } from '@ant-design/pro-form';
import { exportFile, getWordsCount, removeHtmlTag } from '@/utils';
import { pluginImportKeyword } from '@/services/plugin/keyword';
import {
  pluginGetMaterialCategories,
  pluginMaterialConvertFile,
  pluginMaterialImport,
} from '@/services/plugin/material';
import { CloseCircleOutlined } from '@ant-design/icons';
import './index.less';

export type MaterialImportProps = {
  onCancel: (flag?: boolean) => void;
};

const MaterialImport: React.FC<MaterialImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [showTextarea, setShowTextarea] = useState<boolean>(false);
  const [currentCategoryId, setCurrentCategoryId] = useState<number>(0);
  const [uploadedMaterials, setUploadedMaterials] = useState<any[]>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [editingContent, setEditingContent] = useState<string>('');

  useEffect(() => {
    getSetting();
  }, []);

  const getSetting = async () => {
    const res = await pluginGetMaterialCategories();
    let categories = res.data || [];
    setCategories(categories);
  };

  const handleSelectCategory = (categoryId: number) => {
    setCurrentCategoryId(categoryId);
    for (let i in uploadedMaterials) {
      uploadedMaterials[i].category_id = categoryId;
    }
    setUploadedMaterials([].concat(...uploadedMaterials));
  };

  const handleUploadArticle = (e: any) => {
    //需要先上传
    const hide = message.loading('正在处理中', 0);
    let formData = new FormData();
    formData.append('file', e.file);
    formData.append('remove_tag', 'true');
    pluginMaterialConvertFile(formData)
      .then((res) => {
        let count = updateUploadedMaterials(res.data);

        message.success({
          content: '已选择个' + count + '片段',
          key: 'single-message',
        });
      })
      .finally(() => {
        hide();
      });
  };

  const updateUploadedMaterials = (str: string) => {
    let items: any = str.split('\n');
    let tmp = '';
    for (let item of items) {
      if(tmp){
        tmp += "<br/>" + item.trim();
      } else {
        tmp += item.trim();
      }
      if (getWordsCount(item) < 30) {
        continue;
      }
      let exists = false;
      for (let frag of uploadedMaterials) {
        if (frag.content == tmp) {
          exists = true;
          tmp = '';
          break;
        }
      }
      if (!exists) {
        uploadedMaterials.push({
          content: tmp,
          category_id: currentCategoryId,
        });
        tmp = '';
      }
    }
    setUploadedMaterials([].concat(...uploadedMaterials));

    return uploadedMaterials.length;
  };

  const handleClearUpload = () => {
    Modal.confirm({
      title: '确定要清除已选择上传的内容素材吗',
      cancelText: '取消',
      okText: '确定',
      onOk: () => {
        setUploadedMaterials([]);
      },
    });
  };

  const handleRemoveUploadedFragment = (index: number) => {
    Modal.confirm({
      title: '确定要删除选中的内容素材吗？',
      cancelText: '取消',
      okText: '确定',
      onOk: () => {
        uploadedMaterials.splice(index, 1);
        setUploadedMaterials([].concat(...uploadedMaterials));
      },
    });
  };

  const mergeToNext = (index: number) => {
    if (uploadedMaterials.length <= index + 1) {
      return;
    }
    let item = uploadedMaterials[index].content;
    uploadedMaterials.splice(index, 1);
    uploadedMaterials[index].content = item + '\n' + uploadedMaterials[index].content;
    setUploadedMaterials([].concat(...uploadedMaterials));
  };

  const handleSelectInnerCategory = (index: number, category_id: number) => {
    uploadedMaterials[index].category_id = category_id;
    setUploadedMaterials([].concat(...uploadedMaterials));
  };

  const submitTextarea = () => {
    let content = removeHtmlTag(editingContent);
    let count = updateUploadedMaterials(content);
    setShowTextarea(false);
    message.success({
      content: '已选择个' + count + '片段',
      key: 'single-message',
    });
  };

  const handleSubmitImport = () => {
    // 先检查是否有选择栏目
    let noCategoryId = 0;
    for (let i in uploadedMaterials) {
      if (!uploadedMaterials[i].category_id) {
        noCategoryId++;
      }
    }
    if (noCategoryId > 0) {
      Modal.confirm({
        title: (
          <span>
            你选择的素材中，有 <span className="text-red">{noCategoryId}</span>{' '}
            个素材未选择板块，是否要继续提交？
          </span>
        ),
        onOk: () => {
          const hide = message.loading('正在处理中', 0);
          pluginMaterialImport({ materials: uploadedMaterials })
            .then((res) => {
              message.success({ content: res.msg, key: 'single-message' });
              setVisible(false);
              props.onCancel();
            })
            .catch((err) => {
              message.error({
                content: '上传错误，请稍后重试',
                key: 'single-message',
              });
            })
            .finally(() => {
              hide();
            });
        },
      });
    } else {
      const hide = message.loading('正在处理中', 0);
      pluginMaterialImport({ materials: uploadedMaterials })
        .then((res) => {
          message.success({ content: res.msg, key: 'single-message' });
          setVisible(false);
          props.onCancel();
        })
        .catch((err) => {
          message.error({
            content: '上传错误，请稍后重试',
            key: 'single-message',
          });
        })
        .finally(() => {
          hide();
        });
    }
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
        width={800}
        title={'批量添加素材'}
        visible={visible}
        modalProps={{
          onCancel: () => {
            setVisible(false);
          },
        }}
        layout="horizontal"
        onFinish={async (values) => {
          handleSubmitImport();
        }}
      >
        <Alert message="说明：可以上传存放在txt或html的文章。" />
        <div className="mt-normal">
          <Row className="input-field-item">
            <Col flex={0} className="field-label">
              默认导入到：
            </Col>
            <Col flex={1} className="field-value">
              <Select
                className="large-selecter"
                placeholder="选择要导入的板块"
                onChange={handleSelectCategory}
                allowClear
                value={currentCategoryId}
                style={{ width: '150px' }}
              >
                <Select.Option value={0}>全部</Select.Option>
                {categories?.map((category: any) => (
                  <Select.Option key={category.id} value={category.id}>
                    {category.title}
                  </Select.Option>
                ))}
              </Select>
            </Col>
          </Row>
        </div>
        <Row className="input-field-item">
          <Col flex={0} className="field-label">
            选&nbsp;择&nbsp;上&nbsp;传：
          </Col>
          <Col className="field-label">
            <div>
              <Upload
                name="file"
                accept=".txt,.html"
                multiple={true}
                showUploadList={false}
                customRequest={handleUploadArticle}
              >
                <Button>选择Txt或html文章文件</Button>
              </Upload>
            </div>
          </Col>
          <Col className="field-label">
            <div>
              <Button
                onClick={() => {
                  setEditingContent('');
                  setShowTextarea(true);
                }}
              >
                或点击粘贴文本
              </Button>
            </div>
          </Col>
          <Col flex={1} className="field-value">
            <div>
              {uploadedMaterials.length > 0 && (
                <>
                  <span className="ml-normal">已选择段落素材：{uploadedMaterials.length}个</span>
                  <span className="ml-normal">
                    <Button
                      size="small"
                      onClick={() => {
                        handleClearUpload();
                      }}
                    >
                      清除
                    </Button>
                  </span>
                </>
              )}
            </div>
          </Col>
        </Row>
        <div className="tips mb-normal">
          <div className="fragment-list" style={{ height: '250px', overflowY: 'scroll' }}>
            {uploadedMaterials.map((item: any, index: number) => (
              <Row align="middle" className="fragment-item" key={index} gutter={20}>
                <Col span={18}>
                  <span
                    className="close"
                    onClick={() => {
                      handleRemoveUploadedFragment(index);
                    }}
                  >
                    <CloseCircleOutlined />
                  </span>
                  <div>{item.content}</div>
                </Col>
                <Col span={6}>
                  <Space direction="vertical">
                    <Select
                      className="large-selecter"
                      placeholder="选择板块"
                      onChange={(e) => {
                        handleSelectInnerCategory(index, e);
                      }}
                      allowClear
                      value={item.category_id}
                      style={{ width: '150px' }}
                    >
                      <Select.Option value={0}>全部</Select.Option>
                      {categories?.map((category: any) => (
                        <Select.Option key={category.id} value={category.id}>
                          {category.title}
                        </Select.Option>
                      ))}
                    </Select>
                    <Button
                      onClick={() => {
                        mergeToNext(index);
                      }}
                    >
                      向下合并
                    </Button>
                  </Space>
                </Col>
              </Row>
            ))}
          </div>
        </div>
      </ModalForm>
      <Modal
        title="请在这里粘贴文章内容"
        visible={showTextarea}
        width={800}
        okText="解析内容"
        onCancel={() => {
          setShowTextarea(false);
          setEditingContent('');
        }}
        onOk={submitTextarea}
      >
        <Input.TextArea
          style={{ margin: '10px 0', padding: '10px' }}
          rows={15}
          onChange={(e: any) => {
            setEditingContent(e.target.value);
          }}
          value={editingContent}
        />
      </Modal>
    </>
  );
};

export default MaterialImport;
