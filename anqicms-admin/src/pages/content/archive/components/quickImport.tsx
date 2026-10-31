import AttachmentSelect from '@/components/attachment';
import {
  archiveQuickImport,
  getCategories,
  getQuickImportArchiveStatus,
} from '@/services';
import { calculateFileMd5, downloadFile } from '@/utils';
import { PlusOutlined } from '@ant-design/icons';
import {
  ModalForm,
  ProFormDigit,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
} from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Button,
  Col,
  Divider,
  Image,
  Modal,
  Progress,
  Row,
  Upload,
  message,
} from 'antd';
import React, { useEffect, useState } from 'react';
import './index.less';

export type quickImportProps = {
  open: boolean;
  onOpenChange: (flag: boolean) => void;
};

let running = false;
let intXhr: any;

const QuickImportModal: React.FC<quickImportProps> = (props) => {
  const [planType, setPlanType] = useState<number>(0);
  const [uploadedFile, setUploadedFile] = useState<any>(null);
  const [task, setTask] = useState<any>(null);
  const [insertImage, setInsertImage] = useState<number>(0);
  const [images, setImages] = useState<any[]>([]);
  const [categoryId, setCategoryId] = useState<number>(0);
  const [categories, setCategories] = useState<any[]>([]);
  const intl = useIntl();

  const syncTask = async () => {
    getQuickImportArchiveStatus().then((res) => {
      if (res.data) {
        setTask(res.data);
      } else {
        if (running) {
          running = false;
          props.onOpenChange(false);
        }
        clearInterval(intXhr);
        setTask(null);
      }
    });
  };

  useEffect(() => {
    getCategories().then((res) => {
      setCategories(res.data || []);
    });
    // 进入页面的时候查询一次task
    syncTask();
    // 定时查询task
    intXhr = setInterval(() => {
      syncTask();
    }, 1000);
    return () => {
      running = false;
      clearInterval(intXhr);
    };
  }, []);

  const handleRemoveImage = (index: number) => {
    images.splice(index, 1);

    setImages([...images]);
  };

  const handleSelectLogo = (rows: any) => {
    for (const row of rows) {
      let exists = false;
      for (let i in images) {
        if (images[i] === row.logo) {
          exists = true;
          break;
        }
      }
      if (!exists) {
        images.push(row.logo);
      }
    }

    setImages([...images]);
  };

  const handleSelectUploadZip = (e: any) => {
    setUploadedFile(e.file);
  };

  const handleGetTemplate = () => {
    if (categoryId === 0) {
      message.error(
        intl.formatMessage({
          id: 'content.quick-import.category_id.required',
        }),
      );
      return;
    }
    downloadFile('/archive/import/exceltemplate', {
      category_id: categoryId,
    });
  };

  const handleSubmit = async (values: any) => {
    if (!uploadedFile) {
      message.info(
        intl.formatMessage({
          id: 'content.quick-import.upload-file-required',
        }),
      );
      return;
    }
    if (!values['category_id']) {
      message.info(
        intl.formatMessage({
          id: 'content.quick-import.category-required',
        }),
      );
      return;
    }
    const formData = new FormData();
    let hide = message.loading({
      content: intl.formatMessage({
        id: 'setting.system.submitting',
      }),
      duration: 0,
      key: 'uploading',
    });
    // eslint-disable-next-line guard-for-in
    for (let key in values) {
      formData.append(key, values[key]);
    }
    for (let item of images) {
      formData.append('images[]', item);
    }
    const size = uploadedFile.size;
    const md5Value = await calculateFileMd5(uploadedFile);
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const totalChunks = Math.ceil(size / chunkSize);
    formData.append('file_name', uploadedFile.name);
    formData.append('md5', md5Value as string);
    if (totalChunks > 1) {
      // 大于 chunkSize 的，使用分片上传
      formData.append('chunks', totalChunks + '');
      for (let i = 0; i < totalChunks; i++) {
        const chunk = uploadedFile.slice(i * chunkSize, (i + 1) * chunkSize);
        chunk.name = uploadedFile.name;
        chunk.uid = uploadedFile.uid;
        formData.set('chunk', i + '');
        formData.set('file', chunk);
        try {
          const res = await archiveQuickImport(formData);
          if (res.code !== 0) {
            message.info(res.msg);
            hide();
          } else {
            hide = message.loading({
              content:
                intl.formatMessage({
                  id: 'setting.system.submitting',
                }) +
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
                  intl.formatMessage({
                    id: 'setting.system.upload-success',
                  }),
              );
              running = true;
              intXhr = setInterval(() => {
                syncTask();
              }, 1000);
            }
          }
        } catch (err) {
          hide();
          message.info(
            intl.formatMessage({ id: 'content.quick-import.upload-failed' }),
          );
        }
      }
    } else {
      // 小于 chunkSize 的，直接上传
      formData.append('file', uploadedFile);
      archiveQuickImport(formData)
        .then((res) => {
          message.success(res.msg);
          running = true;
          intXhr = setInterval(() => {
            syncTask();
          }, 1000);
        })
        .finally(() => {
          hide();
        });
    }
  };

  return (
    <>
      <ModalForm
        open={props.open}
        width={600}
        title={intl.formatMessage({ id: 'content.quick-import.name' })}
        layout="horizontal"
        onOpenChange={(flag) => {
          props.onOpenChange(flag);
        }}
        onFinish={handleSubmit}
      >
        <Divider>
          <FormattedMessage id="content.quick-import.step1" />
        </Divider>
        <ProFormRadio.Group
          name="plan_type"
          width="lg"
          label={intl.formatMessage({
            id: 'content.quick-import.type',
          })}
          valueEnum={{
            0: intl.formatMessage({
              id: 'content.quick-import.type.normal',
            }),
            1: intl.formatMessage({
              id: 'content.quick-import.type.draft',
            }),
            2: intl.formatMessage({
              id: 'content.quick-import.type.plan',
            }),
          }}
          fieldProps={{
            onChange: (e) => {
              setPlanType(e.target.value);
            },
          }}
        />
        {Number(planType) === 2 && (
          <>
            <ProFormSelect
              name="plan_start"
              label={intl.formatMessage({
                id: 'content.quick-import.plan_start',
              })}
              valueEnum={{
                0: intl.formatMessage({
                  id: 'content.quick-import.plan_start.0',
                }),
                1: intl.formatMessage({
                  id: 'content.quick-import.plan_start.1',
                }),
                2: intl.formatMessage({
                  id: 'content.quick-import.plan_start.2',
                }),
                3: intl.formatMessage({
                  id: 'content.quick-import.plan_start.3',
                }),
                4: intl.formatMessage({
                  id: 'content.quick-import.plan_start.4',
                }),
                5: intl.formatMessage({
                  id: 'content.quick-import.plan_start.5',
                }),
                6: intl.formatMessage({
                  id: 'content.quick-import.plan_start.6',
                }),
                7: intl.formatMessage({
                  id: 'content.quick-import.plan_start.7',
                }),
                8: intl.formatMessage({
                  id: 'content.quick-import.plan_start.8',
                }),
                9: intl.formatMessage({
                  id: 'content.quick-import.plan_start.9',
                }),
                10: intl.formatMessage({
                  id: 'content.quick-import.plan_start.10',
                }),
                11: intl.formatMessage({
                  id: 'content.quick-import.plan_start.11',
                }),
                12: intl.formatMessage({
                  id: 'content.quick-import.plan_start.12',
                }),
                13: intl.formatMessage({
                  id: 'content.quick-import.plan_start.13',
                }),
              }}
            />
            <ProFormDigit
              name="days"
              required
              label={intl.formatMessage({
                id: 'content.quick-import.days',
              })}
              width="lg"
              fieldProps={{
                addonAfter: intl.formatMessage({
                  id: 'content.quick-import.days.suffix',
                }),
              }}
            />
          </>
        )}
        <ProFormRadio.Group
          name="title_type"
          width="lg"
          label={intl.formatMessage({
            id: 'content.quick-import.title-type',
          })}
          valueEnum={{
            0: intl.formatMessage({
              id: 'content.quick-import.title-type.title',
            }),
            1: intl.formatMessage({
              id: 'content.quick-import.title-type.content',
            }),
          }}
        />
        <ProFormRadio.Group
          name="check_duplicate"
          width="lg"
          label={intl.formatMessage({
            id: 'content.quick-import.check_duplicate',
          })}
          valueEnum={{
            0: intl.formatMessage({
              id: 'content.quick-import.check_duplicate.default',
            }),
            1: intl.formatMessage({
              id: 'content.quick-import.check_duplicate.check',
            }),
          }}
        />
        <ProFormSelect
          name="category_id"
          required
          label={intl.formatMessage({
            id: 'content.quick-import.category_id.title',
          })}
          options={[
            {
              parents: [],
              title: intl.formatMessage({ id: 'content.please-select' }),
              id: 0,
            },
          ]
            .concat(categories)
            .map((cat: any) => ({
              title: cat.title,
              label: (
                <div title={cat.title}>
                  {cat.parents?.length > 0 ? (
                    <span className="text-muted">
                      {cat.parents
                        .map((parent: any) => parent.title)
                        .join(' > ')}
                      {' > '}
                    </span>
                  ) : (
                    ''
                  )}
                  {cat.title}
                </div>
              ),
              value: cat.id,
              disabled: cat.status !== 1,
            }))}
          fieldProps={{
            showSearch: true,
            filterOption: (input: string, option: any) =>
              (option?.title ?? option?.label)
                .toLowerCase()
                .includes(input.toLowerCase()),
            onChange: (value) => {
              setCategoryId(value as number);
            },
          }}
        />
        <ProFormRadio.Group
          name="insert_image"
          label={intl.formatMessage({
            id: 'plugin.aigenerate.insert-image',
          })}
          options={[
            {
              label: intl.formatMessage({
                id: 'plugin.aigenerate.insert-image.default',
              }),
              value: 0,
            },
            {
              label: intl.formatMessage({
                id: 'plugin.aigenerate.insert-image.diy',
              }),
              value: 2,
            },
            {
              label: intl.formatMessage({
                id: 'plugin.aigenerate.insert-image.category',
              }),
              value: 3,
            },
          ]}
          fieldProps={{
            onChange: (e) => {
              setInsertImage(e.target.value);
            },
          }}
        />
        {insertImage === 2 && (
          <ProFormText
            label={intl.formatMessage({
              id: 'plugin.aigenerate.insert-image.list',
            })}
          >
            <div className="insert-image">
              <Row gutter={[16, 16]} className="image-list">
                {images?.map((item: any, index: number) => (
                  <Col span={6} key={index}>
                    <div className="image-item">
                      <div className="inner">
                        <div className="link">
                          <Image className="img" preview={true} src={item} />
                          <span
                            className="close"
                            onClick={() => handleRemoveImage(index)}
                          >
                            <FormattedMessage id="setting.system.delete" />
                          </span>
                        </div>
                      </div>
                    </div>
                  </Col>
                ))}
                <Col span={6}>
                  <div className="image-item">
                    <div className="inner">
                      <div className="link">
                        <AttachmentSelect
                          onSelect={handleSelectLogo}
                          open={false}
                          multiple={true}
                        >
                          <div className="ant-upload-item">
                            <div className="add">
                              <PlusOutlined />
                              <div style={{ marginTop: 8 }}>
                                <FormattedMessage id="setting.system.upload" />
                              </div>
                            </div>
                          </div>
                        </AttachmentSelect>
                      </div>
                    </div>
                  </div>
                </Col>
              </Row>
            </div>
          </ProFormText>
        )}
        {insertImage === 3 && (
          <ProFormSelect
            label={intl.formatMessage({
              id: 'plugin.aigenerate.image.category',
            })}
            name="image_category_id"
            required
            extra={intl.formatMessage({
              id: 'plugin.aigenerate.image.category.description',
            })}
            request={async () => {
              const res = await getCategories();
              const data = (res.data || []).concat(
                {
                  id: 0,
                  title: intl.formatMessage({
                    id: 'plugin.aigenerate.image.category.default',
                  }),
                },
                {
                  id: -1,
                  title: intl.formatMessage({
                    id: 'plugin.aigenerate.image.category.all',
                  }),
                },
                {
                  id: -2,
                  title: intl.formatMessage({
                    id: 'plugin.aigenerate.image.category.match',
                  }),
                },
              );
              return data;
            }}
            fieldProps={{
              fieldNames: {
                label: 'title',
                value: 'id',
              },
            }}
          />
        )}
        <Divider>
          <FormattedMessage id="content.quick-import.step2" />
          <div className="download-template">
            <Button type="link" onClick={handleGetTemplate}>
              <FormattedMessage id="content.quick-import.excel-template" />
            </Button>
          </div>
        </Divider>
        <ProFormText
          label={intl.formatMessage({
            id: 'content.quick-import.upload',
          })}
          extra={intl.formatMessage({
            id: 'content.quick-import.upload.description',
          })}
        >
          <Upload
            name="file"
            multiple
            showUploadList={false}
            accept=".zip,.xlsx"
            customRequest={(e) => {
              handleSelectUploadZip(e);
            }}
          >
            <Button>
              <FormattedMessage id="content.quick-import.upload.btn" />
            </Button>
          </Upload>
          {uploadedFile && (
            <div className="mt-normal">
              <FormattedMessage id="content.quick-import.upload.selected" />
              {uploadedFile.name}
            </div>
          )}
        </ProFormText>
      </ModalForm>
      {task !== null && (
        <Modal
          title={intl.formatMessage({ id: 'content.quick-import.status' })}
          open={true}
          footer={null}
        >
          <div className="task-progress">
            <Progress
              percent={
                task.is_finished
                  ? 100
                  : Math.ceil((task.finished * 100) / task.total)
              }
            />
          </div>
          <div className="task-message">{task.message}</div>
        </Modal>
      )}
    </>
  );
};

export default QuickImportModal;
