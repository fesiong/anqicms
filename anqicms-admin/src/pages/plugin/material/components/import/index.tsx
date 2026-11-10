import {
  pluginGetMaterialCategories,
  pluginMaterialConvertFile,
  pluginMaterialImport,
} from '@/services/plugin/material';
import { getWordsCount, removeHtmlTag } from '@/utils';
import { CloseCircleOutlined } from '@ant-design/icons';
import { ModalForm } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Alert,
  Button,
  Checkbox,
  Col,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Upload,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

export type MaterialImportProps = {
  onCancel: (flag?: boolean) => void;
  children?: React.ReactNode;
};

const MaterialImport: React.FC<MaterialImportProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [showTextarea, setShowTextarea] = useState<boolean>(false);
  const [currentCategoryId, setCurrentCategoryId] = useState<number>(0);
  const [uploadedMaterials, setUploadedMaterials] = useState<any[]>([]);
  const [categories, setCategories] = useState<any[]>([]);
  const [editingContent, setEditingContent] = useState<string>('');
  const [containHtml, setContainHtml] = useState<boolean>(false);
  const intl = useIntl();

  const getSetting = async () => {
    const res = await pluginGetMaterialCategories();
    let categories = res.data || [];
    setCategories(categories);
  };

  useEffect(() => {
    getSetting();
  }, []);

  const handleSelectCategory = (categoryId: number) => {
    setCurrentCategoryId(categoryId);
    for (let i in uploadedMaterials) {
      uploadedMaterials[i].category_id = categoryId;
    }
    setUploadedMaterials([].concat(...uploadedMaterials));
  };

  const updateUploadedMaterials = (str: string) => {
    let items: any = str.split('\n');
    let tmp = '';
    for (let item of items) {
      if (tmp) {
        tmp += '<br/>' + item.trim();
      } else {
        tmp += item.trim();
      }
      if (getWordsCount(item) < 10) {
        continue;
      }
      let exists = false;
      for (let frag of uploadedMaterials) {
        if (frag.content === tmp) {
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

  const handleUploadArticle = (e: any) => {
    //需要先上传
    const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
    let formData = new FormData();
    formData.append('file', e.file);
    formData.append('remove_tag', 'true');
    pluginMaterialConvertFile(formData)
      .then((res) => {
        let count = updateUploadedMaterials(res.data);

        message.success(
          intl.formatMessage({ id: 'plugin.material.import.selected' }) +
            count +
            intl.formatMessage({ id: 'plugin.material.import.segment' }),
        );
      })
      .finally(() => {
        hide();
      });
  };

  const handleClearUpload = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.material.import.clear' }),
      onOk: () => {
        setUploadedMaterials([]);
      },
    });
  };

  const handleRemoveUploadedFragment = (index: number) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.material.delete.confirm' }),
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
    let content = editingContent;
    if (!containHtml) {
      content = removeHtmlTag(editingContent);
    }
    let count = updateUploadedMaterials(content);
    setShowTextarea(false);
    message.success(
      intl.formatMessage({ id: 'plugin.material.import.selected' }) +
        count +
        intl.formatMessage({ id: 'plugin.material.import.segment' }),
    );
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
            {intl.formatMessage({ id: 'plugin.material.import.submit.tips.before' })}
            <span className="text-red">{noCategoryId}</span>{' '}
            {intl.formatMessage({ id: 'plugin.material.import.submit.tips.after' })}
          </span>
        ),
        onOk: () => {
          const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
          pluginMaterialImport({ materials: uploadedMaterials })
            .then((res) => {
              message.success(res.msg);
              setVisible(false);
              props.onCancel();
            })
            .catch(() => {
              message.error(intl.formatMessage({ id: 'plugin.material.import.upload-error' }));
            })
            .finally(() => {
              hide();
            });
        },
      });
    } else {
      const hide = message.loading(intl.formatMessage({ id: 'setting.system.submitting' }), 0);
      pluginMaterialImport({ materials: uploadedMaterials })
        .then((res) => {
          message.success(res.msg);
          setVisible(false);
          props.onCancel();
        })
        .catch(() => {
          message.error(intl.formatMessage({ id: 'plugin.material.import.upload-error' }));
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
        title={intl.formatMessage({ id: 'plugin.material.import.batch-add' })}
        open={visible}
        modalProps={{
          onCancel: () => {
            setVisible(false);
          },
        }}
        layout="horizontal"
        onFinish={async () => {
          handleSubmitImport();
        }}
      >
        <Alert message={intl.formatMessage({ id: 'plugin.material.import.batch-add.tips' })} />
        <div className="mt-normal">
          <Row className="input-field-item">
            <Col flex={0} className="field-label">
              <FormattedMessage id="plugin.material.import.default-category" />
            </Col>
            <Col flex={1} className="field-value">
              <Select
                className="large-selecter"
                placeholder={intl.formatMessage({
                  id: 'plugin.material.import.default-category.placeholder',
                })}
                onChange={handleSelectCategory}
                allowClear
                value={currentCategoryId}
                style={{ width: '150px' }}
              >
                <Select.Option value={0}>
                  <FormattedMessage id="plugin.material.import.default-category.all" />
                </Select.Option>
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
            <FormattedMessage id="plugin.material.import.select-file" />
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
                <Button>
                  <FormattedMessage id="plugin.material.import.select-file.btn" />
                </Button>
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
                <FormattedMessage id="plugin.material.import.paste" />
              </Button>
            </div>
          </Col>
          <Col flex={1} className="field-value">
            <div>
              {uploadedMaterials.length > 0 && (
                <>
                  <span className="ml-normal">
                    <FormattedMessage id="plugin.material.import.selected.count" />
                    {uploadedMaterials.length}
                  </span>
                  <span className="ml-normal">
                    <Button
                      size="small"
                      onClick={() => {
                        handleClearUpload();
                      }}
                    >
                      <FormattedMessage id="plugin.material.import.paste.clear" />
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
                      placeholder={intl.formatMessage({
                        id: 'plugin.material.import.category.select',
                      })}
                      onChange={(e) => {
                        handleSelectInnerCategory(index, e);
                      }}
                      allowClear
                      value={item.category_id}
                      style={{ width: '150px' }}
                    >
                      <Select.Option value={0}>
                        <FormattedMessage id="plugin.material.import.default-category.all" />
                      </Select.Option>
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
                      <FormattedMessage id="plugin.material.import.merge-to-next" />
                    </Button>
                  </Space>
                </Col>
              </Row>
            ))}
          </div>
        </div>
      </ModalForm>
      <Modal
        title={intl.formatMessage({ id: 'plugin.material.import.paste.title' })}
        open={showTextarea}
        width={800}
        okText={intl.formatMessage({ id: 'plugin.material.import.paste.analysis' })}
        onCancel={() => {
          setShowTextarea(false);
          setEditingContent('');
        }}
        onOk={submitTextarea}
      >
        <Alert
          className="mb-normal"
          message={
            <div>
              <div>
                <FormattedMessage id="plugin.material.import.paste.description" />
                <Checkbox
                  value={true}
                  checked={containHtml}
                  onChange={(e) => setContainHtml(e.target.checked)}
                >
                  <FormattedMessage id="plugin.material.import.paste.description.btn" />
                </Checkbox>
              </div>
            </div>
          }
        />
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
