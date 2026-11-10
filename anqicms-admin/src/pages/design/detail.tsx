import {
  UploadDesignFileInfo,
  backupDesignData,
  copyDesignFileInfo,
  deleteDesignFileInfo,
  getDesignInfo,
  restoreDesignData,
  saveDesignFileInfo,
  saveDesignInfo,
} from '@/services';
import { downloadFile, sizeFormat } from '@/utils';
import { PlusOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormInstance,
  ProFormRadio,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl, useModel } from '@umijs/max';
import {
  Avatar,
  Button,
  Checkbox,
  Image,
  Input,
  Modal,
  Space,
  Tooltip,
  Upload,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';
import TemplateShare from './components/share';
import './index.less';

let autoBackup = false;
const DesignDetail: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const formRef = React.createRef<ProFormInstance>();
  const [designInfo, setDesignInfo] = useState<any>({});
  const [addVisible, setAddVisible] = useState<boolean>(false);
  const [addFileType, setAddFileType] = useState<string>('');
  const [editVisible, setEditVisible] = useState<boolean>(false);
  const [currentFile, setCurrentFile] = useState<any>({});
  const [visible, setVisible] = useState<boolean>(false);
  const actionRef = useRef<ActionType>();
  const staticActionRef = useRef<ActionType>();
  const [staticDirs, setStaticDirs] = useState<any[]>([]);
  const [templateDirs, setTemplateDirs] = useState<any[]>([]);
  const [detailVisible, setDetailVisible] = useState<boolean>(false);
  const [tempDirs, setTempDirs] = useState<any[]>([]);
  const intl = useIntl();

  const inputRef = useRef<any>();
  const inputRef2 = useRef<any>();

  const anqiUser = initialState?.anqiUser;

  const fetchDesignInfo = async () => {
    const searchParams = new URLSearchParams(window.location.search);
    const packageName = searchParams.get('package') || '';
    getDesignInfo({
      package: packageName,
    })
      .then((res) => {
        setDesignInfo(res.data || {});
        actionRef.current?.reload();
        staticActionRef.current?.reload();

        let tmpDirs = new Set();
        for (let item of res.data.tpl_files) {
          let path = item.path.substring(0, item.path.lastIndexOf('/') + 1);
          if (!path) {
            path = '/';
          }
          tmpDirs.add(path);
        }
        let tmpList = [];
        for (let key of tmpDirs.keys()) {
          tmpList.push({
            label: key,
            value: key,
          });
        }
        setTemplateDirs(tmpList);

        tmpDirs.clear();
        for (let item of res.data.static_files) {
          let path = item.path.substring(0, item.path.lastIndexOf('/') + 1);
          if (!path) {
            path = '/';
          }
          tmpDirs.add(path);
        }
        let tmpList2 = [];
        for (let key of tmpDirs.keys()) {
          tmpList2.push({
            label: key,
            value: key,
          });
        }
        setStaticDirs(tmpList2);
      })
      .catch((err) => {
        console.log(err);
        message.error(intl.formatMessage({ id: 'design.editor.get.error' }));
      });
  };

  useEffect(() => {
    fetchDesignInfo();
  }, []);

  const handleShowEdit = (type: string, info: any) => {
    // 可编辑的文件
    if (
      info.path.indexOf('.html') !== -1 ||
      info.path.indexOf('.yml') !== -1 ||
      info.path.indexOf('.yaml') !== -1 ||
      info.path.indexOf('.css') !== -1 ||
      info.path.indexOf('.less') !== -1 ||
      info.path.indexOf('.scss') !== -1 ||
      info.path.indexOf('.sass') !== -1 ||
      info.path.indexOf('.js') !== -1
    ) {
      history.push(
        `/design/editor?package=${designInfo.package}&type=${type}&path=${info.path}`,
      );
    } else {
      setAddFileType(type);
      setCurrentFile(info);
      setDetailVisible(true);
    }
  };

  const handleRemove = (type: string, info: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.detail.confirm-delete' }),
      onOk: () => {
        deleteDesignFileInfo({
          package: designInfo.package,
          type: type,
          path: info.path,
        })
          .then((res) => {
            message.info(res.msg);
          })
          .finally(() => {
            setDetailVisible(false);
          });
        fetchDesignInfo();
      },
    });
  };

  const handleCopy = (type: string, info: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.detail.confirm-copy' }),
      content: (
        <div>
          <div style={{ padding: '10px 0' }}>
            <div>
              <FormattedMessage id="design.detail.copy-original-name" />
            </div>
            <div>
              <Input disabled value={info.path} />
            </div>
          </div>
          <div style={{ padding: '10px 0' }}>
            <div>{intl.formatMessage({ id: 'design.detail.new-name' })}</div>
            <div>
              <Input ref={inputRef} defaultValue={info.path} />
            </div>
          </div>
          <div style={{ padding: '10px 0' }}>
            <div>{intl.formatMessage({ id: 'design.detail.new-remark' })}</div>
            <div>
              <Input ref={inputRef2} defaultValue={info.remark} />
            </div>
          </div>
        </div>
      ),
      onOk: () => {
        const newPath = inputRef.current?.input?.value;
        if (!newPath || newPath === info.path) {
          message.error(
            intl.formatMessage({ id: 'design.detail.name-duplicate' }),
          );
          return false;
        }
        const remark = inputRef2.current?.input?.value;
        copyDesignFileInfo({
          package: designInfo.package,
          type: type,
          path: info.path,
          new_path: newPath,
          remark: remark,
        }).then((res) => {
          message.info(res.msg);
          Modal.destroyAll();
          fetchDesignInfo();
        });
        return true;
      },
    });
  };

  const onSearchTempDir = (val: string) => {
    let tmpData = addFileType === 'static' ? staticDirs : templateDirs;
    let e = val;
    if (e === '') {
      setTempDirs(tmpData);
      return;
    }
    let index = -1;
    for (let i in tmpData) {
      if (tmpData[i].value.indexOf(e) !== -1) {
        index = Number(i);
        break;
      }
    }
    if (index === -1) {
      if (e.lastIndexOf('/') !== e.length - 1) {
        e = e + '/';
      }
      setTempDirs([
        {
          label:
            intl.formatMessage({ id: 'design.detail.new-directory' }) +
            ': ' +
            e,
          value: e,
        },
      ]);
    }
  };

  const handleAddFile = (type: string) => {
    onSearchTempDir('');
    setAddFileType(type);
    setAddVisible(true);
  };

  const handleAddRemark = (type: string, info: any) => {
    setAddFileType(type);
    setCurrentFile(info);
    setEditVisible(true);
  };

  const handleSaveFile = (values: any) => {
    values.rename_path = values.path;
    values.path = currentFile.path;
    if (!values.path) {
      values.path = values.rename_path;
    }
    if (values.path.trim() === '') {
      message.error(intl.formatMessage({ id: 'design.detail.name-required' }));
      return;
    }
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    values.package = designInfo.package;
    values.type = addFileType;

    saveDesignFileInfo(values)
      .then((res) => {
        message.info(res.msg);
        fetchDesignInfo();
        setEditVisible(false);
        setAddVisible(false);
        setDetailVisible(false);
      })
      .finally(() => {
        hide();
      });
  };

  const handleSaveInfo = (values: any) => {
    designInfo.name = values.name;
    designInfo.template_type = values.template_type;

    saveDesignInfo(designInfo).then((res) => {
      message.info(res.msg);
      fetchDesignInfo();
      setVisible(false);
    });
  };

  const handleDownload = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.detail.confirm-download' }),
      onOk: async () => {
        downloadFile(
          '/design/download',
          {
            package: designInfo.package,
          },
          designInfo.package,
        );
      },
    });
  };

  const handleUploadTemplate = (e: any) => {
    let values = formRef.current?.getFieldsValue();
    let savePath = values.path || '';
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.detail.confirm-upload' }),
      content:
        intl.formatMessage({
          id: 'design.detail.confirm-upload.content-before',
        }) +
        (addFileType === 'static'
          ? intl.formatMessage({ id: 'design.static.name' })
          : intl.formatMessage({ id: 'design.tempalte.name' })) +
        intl.formatMessage({ id: 'design.detail.confirm-upload.directory' }) +
        `${savePath}`,
      onOk: async () => {
        let formData = new FormData();
        formData.append('file', e.file);
        formData.append('package', designInfo.package);
        formData.append('type', addFileType);
        formData.append('path', savePath);

        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        UploadDesignFileInfo(formData)
          .then((res) => {
            if (res.code !== 0) {
              message.info(res.msg);
            } else {
              message.info(
                res.msg ||
                  intl.formatMessage({ id: 'setting.system.upload-success' }),
              );
              setAddVisible(false);
              fetchDesignInfo();
            }
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleReplaceFile = (e: any) => {
    let replacePath = currentFile.path?.substring(
      0,
      currentFile.path?.lastIndexOf('/') + 1,
    );
    let replaceName = currentFile.path?.substring(
      currentFile.path?.lastIndexOf('/') + 1,
    );
    let formData = new FormData();
    formData.append('file', e.file);
    formData.append('name', replaceName);
    formData.append('package', designInfo.package);
    formData.append('type', addFileType);
    formData.append('path', replacePath);

    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    UploadDesignFileInfo(formData)
      .then((res) => {
        if (res.code !== 0) {
          message.info(res.msg);
        } else {
          message.info(
            res.msg ||
              intl.formatMessage({ id: 'setting.system.upload-success' }),
          );
          setAddVisible(false);
          setDetailVisible(false);
          actionRef.current?.reload();
        }
      })
      .finally(() => {
        hide();
      });
  };

  const handleRestoreDesignData = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.data.confirm-install' }),
      content: (
        <div>
          <p>{intl.formatMessage({ id: 'design.data.tips1' })}</p>
          <p>{intl.formatMessage({ id: 'design.data.tips2' })}</p>
          <p>{intl.formatMessage({ id: 'design.data.tips3' })}</p>
          <div>
            <Checkbox
              value={true}
              onChange={(e) => {
                autoBackup = e.target.checked;
              }}
            >
              <span className="text-red">*</span>
              {intl.formatMessage({ id: 'design.data.auto-backup' })}
            </Checkbox>
          </div>
        </div>
      ),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        restoreDesignData({
          package: designInfo.package,
          auto_backup: autoBackup,
        })
          .then((res) => {
            message.info(res.msg);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleBackupDesignData = () => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.detail.backup-data' }),
      content: (
        <div>
          <p>{intl.formatMessage({ id: 'design.detail.backup-data.tips' })}</p>
          {designInfo.preview_data && (
            <p>
              {intl.formatMessage({
                id: 'design.detail.backup-data.cover.tips',
              })}
            </p>
          )}
        </div>
      ),
      onOk: async () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'design.detail.backup-data.doing' }),
          0,
        );
        backupDesignData({ package: designInfo.package })
          .then((res) => {
            message.info(res.msg);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'design.data.name' }),
      dataIndex: 'path',
      render: (text: any, record: any) => (
        <a
          title={intl.formatMessage({ id: 'design.click-to-edit' })}
          onClick={() => {
            handleShowEdit('template', record);
          }}
        >
          {text}
        </a>
      ),
    },
    {
      title: intl.formatMessage({ id: 'design.remark' }),
      dataIndex: 'remark',
      width: 200,
    },
    {
      title: intl.formatMessage({ id: 'design.size' }),
      dataIndex: 'size',
      width: 150,
      render: (text: any) => <div>{sizeFormat(text)}</div>,
    },
    {
      title: intl.formatMessage({ id: 'design.update-time' }),
      dataIndex: 'last_mod',
      width: 200,
      render: (text: any) =>
        dayjs((text as number) * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      key: 'action',
      width: 100,
      render: (_: any, record: any) => (
        <Space size={16}>
          <Button
            type="link"
            onClick={() => {
              handleAddRemark('template', record);
            }}
          >
            <FormattedMessage id="design.detail.rename" />
          </Button>
          <Button
            type="link"
            onClick={() => {
              handleCopy('template', record);
            }}
          >
            <FormattedMessage id="design.detail.make-copy" />
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              handleRemove('template', record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </Button>
        </Space>
      ),
    },
  ];

  const columnsStatic: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'design.data.name' }),
      dataIndex: 'path',
      render: (text: any, record: any) => (
        <Space size={16}>
          <a
            title={intl.formatMessage({ id: 'design.click-to-edit' })}
            onClick={() => {
              handleShowEdit('static', record);
            }}
          >
            {text}
          </a>
          {(record.path.indexOf('.jpg') !== -1 ||
            record.path.indexOf('.jpeg') !== -1 ||
            record.path.indexOf('.png') !== -1 ||
            record.path.indexOf('.gif') !== -1 ||
            record.path.indexOf('.webp') !== -1 ||
            record.path.indexOf('.svg') !== -1 ||
            record.path.indexOf('.bmp') !== -1 ||
            record.path.indexOf('.tiff') !== -1 ||
            record.path.indexOf('.avif') !== -1 ||
            record.path.indexOf('.heic') !== -1 ||
            record.path.indexOf('.heif') !== -1) && (
            <Image
              height={50}
              className="img"
              preview={{
                src:
                  (initialState?.system?.base_url || '') +
                  '/static/' +
                  designInfo.package +
                  '/' +
                  record.path,
              }}
              src={
                (initialState?.system?.base_url || '') +
                '/static/' +
                designInfo.package +
                '/' +
                record.path
              }
              alt={record.path}
            />
          )}
        </Space>
      ),
    },
    {
      title: intl.formatMessage({ id: 'design.remark' }),
      dataIndex: 'remark',
      width: 200,
    },
    {
      title: intl.formatMessage({ id: 'design.size' }),
      dataIndex: 'size',
      width: 150,
      render: (text: any) => <div>{sizeFormat(text)}</div>,
    },
    {
      title: intl.formatMessage({ id: 'design.update-time' }),
      dataIndex: 'last_mod',
      width: 200,
      render: (text: any) =>
        dayjs((text as number) * 1000).format('YYYY-MM-DD HH:mm'),
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      key: 'action',
      width: 100,
      render: (_: any, record: any) => (
        <Space size={16}>
          <Button
            type="link"
            onClick={() => {
              handleAddRemark('static', record);
            }}
          >
            <FormattedMessage id="design.detail.rename" />
          </Button>
          <Button
            type="link"
            onClick={() => {
              handleCopy('static', record);
            }}
          >
            <FormattedMessage id="design.detail.make-copy" />
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              handleRemove('static', record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </Button>
        </Space>
      ),
    },
  ];

  const canShare =
    anqiUser?.auth_id > 0 &&
    (designInfo.template_id === 0 || designInfo.auth_id === anqiUser?.auth_id);

  return (
    <PageContainer
      title={
        designInfo?.name +
        intl.formatMessage({ id: 'design.detail.file-manage' })
      }
    >
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'design.tempalte.name' })}
        actionRef={actionRef}
        rowKey="path"
        search={false}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleAddFile('template');
            }}
          >
            <PlusOutlined /> <FormattedMessage id="design.detail.new-file" />
          </Button>,
          <Button
            key="edit"
            onClick={() => {
              setVisible(true);
            }}
          >
            <FormattedMessage id="design.detail.template.edit" />
          </Button>,
          <Button
            key="download"
            onClick={() => {
              handleDownload();
            }}
          >
            <FormattedMessage id="design.detail.template.download" />
          </Button>,
          designInfo.status === 1 && (
            <Button key="backup" onClick={handleBackupDesignData}>
              <FormattedMessage id="design.detail.template.backup" />
            </Button>
          ),
          designInfo.preview_data && designInfo.status === 1 && (
            <Tooltip
              title={intl.formatMessage({ id: 'design.data.install.example' })}
              key="restore"
            >
              <Button onClick={handleRestoreDesignData}>
                <FormattedMessage id="design.detail.template.restore" />
              </Button>
            </Tooltip>
          ),
          (canShare || !anqiUser || anqiUser?.auth_id === 0) && (
            <Tooltip
              title={intl.formatMessage({
                id: 'design.detail.template.tomarket',
              })}
              key="share"
            >
              <TemplateShare
                designInfo={designInfo}
                canShare={canShare}
                templateId={designInfo.template_id}
                onFinished={() => {
                  actionRef.current?.reload();
                }}
              >
                <Button>
                  <FormattedMessage id="design.detail.template.share" />
                </Button>
              </TemplateShare>
            </Tooltip>
          ),
        ]}
        request={async () => {
          return {
            data: designInfo.tpl_files || [],
            success: true,
          };
        }}
        columnsState={{
          persistenceKey: 'design-tpl-files-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        pagination={{
          showSizeChanger: true,
        }}
      />
      <ProTable<any>
        headerTitle={intl.formatMessage({ id: 'design.static.name' })}
        actionRef={staticActionRef}
        rowKey="path"
        search={false}
        toolBarRender={() => [
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleAddFile('static');
            }}
          >
            <PlusOutlined /> <FormattedMessage id="design.detail.new-static" />
          </Button>,
        ]}
        request={async () => {
          return {
            data: designInfo.static_files || [],
            success: true,
          };
        }}
        columnsState={{
          persistenceKey: 'design-static-files-table',
          persistenceType: 'localStorage',
        }}
        columns={columnsStatic}
        pagination={{
          showSizeChanger: true,
        }}
      />
      {addVisible && (
        <ModalForm
          width={600}
          title={
            intl.formatMessage({ id: 'design.detail.addnew' }) +
            (addFileType === 'static'
              ? intl.formatMessage({ id: 'design.static.name' })
              : intl.formatMessage({ id: 'design.tempalte.name' }))
          }
          formRef={formRef}
          open={addVisible}
          modalProps={{
            onCancel: () => {
              setAddVisible(false);
            },
          }}
          //layout="horizontal"
          onFinish={async () => {
            setAddVisible(false);
          }}
        >
          <ProFormSelect
            label={intl.formatMessage({ id: 'design.detail.save-path' })}
            showSearch
            name="path"
            width="lg"
            fieldProps={{
              onSearch: async (e) => {
                onSearchTempDir(e);
              },
            }}
            options={tempDirs}
          />
          <ProFormText
            name="tpl"
            label={intl.formatMessage({ id: 'design.tempalte.name' })}
          >
            <Upload
              name="file"
              showUploadList={false}
              customRequest={handleUploadTemplate}
            >
              <Button type="primary">
                <FormattedMessage id="design.detail.select-file" />
              </Button>
            </Upload>
          </ProFormText>
          <div>
            <p>
              <FormattedMessage id="design.detail.upload-tips" />
            </p>
          </div>
        </ModalForm>
      )}
      {editVisible && (
        <ModalForm
          width={600}
          title={
            currentFile.path +
            intl.formatMessage({ id: 'design.detail.edit-file' })
          }
          open={editVisible}
          modalProps={{
            onCancel: () => {
              setEditVisible(false);
            },
          }}
          initialValues={currentFile}
          //layout="horizontal"
          onFinish={async (values) => {
            handleSaveFile(values);
          }}
        >
          <ProFormText
            name="path"
            label={intl.formatMessage({ id: 'design.detail.file-name' })}
          />
          <ProFormText
            name="remark"
            label={intl.formatMessage({ id: 'design.remark' })}
          />
        </ModalForm>
      )}
      {visible && (
        <ModalForm
          width={600}
          title={intl.formatMessage({ id: 'design.detail.template.edit' })}
          open={visible}
          modalProps={{
            onCancel: () => {
              setVisible(false);
            },
          }}
          initialValues={designInfo}
          //layout="horizontal"
          onFinish={async (values) => {
            handleSaveInfo(values);
          }}
        >
          <ProFormText
            name="name"
            label={intl.formatMessage({ id: 'design.detail.template-name' })}
          />
          <ProFormRadio.Group
            name="template_type"
            label={intl.formatMessage({ id: 'design.detail.template-type' })}
            extra={intl.formatMessage({
              id: 'design.detail.template-type.description',
            })}
            options={[
              {
                value: 0,
                label: intl.formatMessage({
                  id: 'setting.system.template-type.auto',
                }),
              },
              {
                value: 1,
                label: intl.formatMessage({
                  id: 'setting.system.template-type.code',
                }),
              },
              {
                value: 2,
                label: intl.formatMessage({
                  id: 'setting.system.template-type.pc-m',
                }),
              },
            ]}
          />
        </ModalForm>
      )}
      <Modal
        width={900}
        title={intl.formatMessage({ id: 'design.detail.static-detail' })}
        onCancel={() => setDetailVisible(false)}
        onOk={() => setDetailVisible(false)}
        open={detailVisible}
      >
        <div className="attachment-detail">
          <div className="preview">
            {currentFile.path &&
            (currentFile.path.indexOf('.jpg') !== -1 ||
              currentFile.path.indexOf('.jpeg') !== -1 ||
              currentFile.path.indexOf('.png') !== -1 ||
              currentFile.path.indexOf('.gif') !== -1 ||
              currentFile.path.indexOf('.webp') !== -1 ||
              currentFile.path.indexOf('.svg') !== -1 ||
              currentFile.path.indexOf('.bmp') !== -1 ||
              currentFile.path.indexOf('.tiff') !== -1 ||
              currentFile.path.indexOf('.avif') !== -1 ||
              currentFile.path.indexOf('.heic') !== -1 ||
              currentFile.path.indexOf('.heif') !== -1) ? (
              <Image
                width={'100%'}
                className="img"
                preview={{
                  src:
                    (initialState?.system?.base_url || '') +
                    '/static/' +
                    designInfo.package +
                    '/' +
                    currentFile.path,
                }}
                src={
                  (initialState?.system?.base_url || '') +
                  '/static/' +
                  designInfo.package +
                  '/' +
                  currentFile.path
                }
                alt={currentFile.path}
              />
            ) : (
              <Avatar className="avatar">
                {currentFile.path?.substring(
                  currentFile.path?.lastIndexOf('/') + 1,
                )}
              </Avatar>
            )}
          </div>
          <div className="detail">
            <div className="info">
              <div className="item">
                <div className="name">
                  <FormattedMessage id="design.detail.path" />:
                </div>
                <div className="value">{currentFile.path}</div>
              </div>
              <div className="item">
                <div className="name">
                  <FormattedMessage id="design.detail.type" />:
                </div>
                <div className="value">
                  {currentFile.path?.substring(
                    currentFile.path?.lastIndexOf('.'),
                  )}
                </div>
              </div>
              <div className="item">
                <div className="name">
                  <FormattedMessage id="design.update-time" />:
                </div>
                <div className="value">
                  {dayjs(currentFile.last_mod * 1000).format(
                    'YYYY-MM-DD HH:mm:ss',
                  )}
                </div>
              </div>
              <div className="item">
                <div className="name">
                  <FormattedMessage id="design.size" />:
                </div>
                <div className="value">{sizeFormat(currentFile.size)}</div>
              </div>
              <div className="item">
                <div className="name">
                  <FormattedMessage id="design.remark" />:
                </div>
                <div className="value">{currentFile.remark}</div>
              </div>
            </div>
            <Space size={16} align="center" className="btns">
              <Upload
                name="file"
                showUploadList={false}
                accept={currentFile.path?.substring(
                  currentFile.path?.lastIndexOf('.'),
                )}
                customRequest={handleReplaceFile}
              >
                <Button>
                  <FormattedMessage id="design.detail.replace" />
                </Button>
              </Upload>
              <Button
                onClick={() => {
                  handleAddRemark('template', currentFile);
                }}
              >
                <FormattedMessage id="design.detail.update-name" />
              </Button>
              <Button
                onClick={() => {
                  handleRemove('template', currentFile);
                }}
              >
                <FormattedMessage id="setting.system.delete" />
              </Button>
              <Button danger onClick={() => setDetailVisible(false)}>
                <FormattedMessage id="setting.action.close" />
              </Button>
            </Space>
            <div className="tips">
              <p>
                <FormattedMessage id="design.detail.tips.name" />
              </p>
              <div>
                <FormattedMessage id="design.detail.tips1" />
              </div>
              <div>
                <FormattedMessage id="design.detail.tips2" />
              </div>
            </div>
          </div>
        </div>
      </Modal>
    </PageContainer>
  );
};

export default DesignDetail;
