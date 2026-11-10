import NewContainer from '@/components/NewContainer';
import {
  UploadDesignInfo,
  activeDesignInfo,
  checkUploadDesignInfo,
  deleteDesignInfo,
  getDesignList,
  restoreDesignData,
} from '@/services';
import {
  ActionType,
  ModalForm,
  ProColumns,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Checkbox, Modal, Space, Tooltip, Upload, message } from 'antd';
import React, { useRef, useState } from 'react';

let autoBackup = false;
let autoCleanup = false;
const DesignIndex: React.FC = () => {
  const [addVisible, setAddVisible] = useState<boolean>(false);
  const actionRef = useRef<ActionType>();
  const [newKey, setNewKey] = useState<string>('');
  const intl = useIntl();

  const onTabChange = (key: string) => {
    setNewKey(key);
  };

  const handleUseTemplate = (template: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.confirm-enable' }),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'design.switching-template' }),
          0,
        );
        activeDesignInfo({ package: template.package });
        setTimeout(() => {
          hide();
          actionRef.current?.reload();
        }, 2000);
      },
    });
  };

  const handleManage = (template: any) => {
    history.push('/design/detail?package=' + template.package);
  };

  const handleShowEdit = (template: any) => {
    history.push('/design/editor?package=' + template.package);
  };

  const handleRemove = (packageName: string) => {
    if (packageName === 'default') {
      message.error(intl.formatMessage({ id: 'design.cannot-delete' }));
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.confirm-delete' }),
      onOk: () => {
        const hide = message.loading(
          intl.formatMessage({ id: 'setting.system.submitting' }),
          0,
        );
        deleteDesignInfo({ package: packageName })
          .then((res) => {
            message.info(res.msg);
            actionRef.current?.reload();
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleUploadZip = (e: any, cover: string) => {
    // end
    const formData = new FormData();
    formData.append('cover', cover);
    formData.append('file', e.file);
    const hide = message.loading(
      intl.formatMessage({ id: 'setting.system.submitting' }),
      0,
    );
    UploadDesignInfo(formData)
      .then((res) => {
        if (res.code !== 0) {
          message.info(res.msg);
        } else {
          message.info(
            res.msg ||
              intl.formatMessage({ id: 'setting.system.upload-success' }),
          );
          setAddVisible(false);
          actionRef.current?.reload();
        }
      })
      .finally(() => {
        hide();
      });
  };

  const checkUploadZip = async (e: any) => {
    // 上传之前先验证是否存在相同的模板
    try {
      const checkRes = await checkUploadDesignInfo({
        package: e.file.name.replace('.zip', ''),
      });
      if (checkRes.code !== 0) {
        Modal.confirm({
          title: intl.formatMessage({ id: 'design.data.confirm-upload' }),
          onOk: () => {
            handleUploadZip(e, 'new');
          },
          okText: intl.formatMessage({ id: 'design.data.confirm.new' }),
          footer: (_, { OkBtn, CancelBtn }) => (
            <>
              <CancelBtn />
              <Button
                onClick={() => {
                  Modal.destroyAll();
                  handleUploadZip(e, 'cover');
                }}
              >
                {intl.formatMessage({ id: 'design.data.confirm.cover' })}
              </Button>
              <OkBtn />
            </>
          ),
        });
      } else {
        handleUploadZip(e, '');
      }
    } catch (err) {
      message.error(err as string);
    }
  };

  const handleRestoreDesignData = (record: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'design.data.confirm-install' }),
      width: 480,
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
            <Checkbox
              value={true}
              onChange={(e) => {
                autoCleanup = e.target.checked;
                if (autoCleanup) {
                  autoBackup = true;
                }
              }}
            >
              <span className="text-red">*</span>
              {intl.formatMessage({ id: 'design.data.auto-clean' })}
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
          package: record.package,
          auto_backup: autoBackup,
          auto_cleanup: autoCleanup,
        })
          .then((res) => {
            message.info(res.msg);
            actionRef.current?.reload();
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
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'design.data.package' }),
      dataIndex: 'package',
    },
    {
      title: intl.formatMessage({ id: 'setting.system.template-type' }),
      dataIndex: 'template_type',
      valueEnum: {
        0: intl.formatMessage({ id: 'setting.system.template-type.auto' }),
        1: intl.formatMessage({ id: 'setting.system.template-type.code' }),
        2: intl.formatMessage({ id: 'setting.system.template-type.pc-m' }),
      },
    },
    {
      title: intl.formatMessage({ id: 'website.status' }),
      dataIndex: 'status',
      valueEnum: {
        0: {
          text: intl.formatMessage({ id: 'design.data.notenable' }),
          status: 'Default',
        },
        1: {
          text: intl.formatMessage({ id: 'design.data.enable' }),
          status: 'Success',
        },
      },
    },
    {
      title: intl.formatMessage({ id: 'account.time' }),
      dataIndex: 'created',
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      key: 'action',
      width: 300,
      render: (_, record: any) => (
        <Space size={16}>
          {record.status !== 1 && (
            <Button
              type="link"
              onClick={() => {
                handleUseTemplate(record);
              }}
            >
              <FormattedMessage id="design.data.enable.action" />
            </Button>
          )}
          {record.preview_data && record.status === 1 && (
            <Tooltip
              title={intl.formatMessage({ id: 'design.data.install.example' })}
            >
              <Button
                type="link"
                onClick={() => {
                  handleRestoreDesignData(record);
                }}
              >
                <FormattedMessage id="design.data.initial" />
              </Button>
            </Tooltip>
          )}
          <Button
            type="link"
            onClick={() => {
              handleManage(record);
            }}
          >
            <FormattedMessage id="design.data.manage" />
          </Button>
          <Button
            type="link"
            onClick={() => {
              handleShowEdit(record);
            }}
          >
            <FormattedMessage id="setting.action.edit" />
          </Button>
          {record.package !== 'default' && (
            <Button
              danger
              type="link"
              onClick={() => {
                handleRemove(record.package);
              }}
            >
              <FormattedMessage id="setting.system.delete" />
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProTable<any>
        key={newKey}
        headerTitle={intl.formatMessage({ id: 'design.list' })}
        toolBarRender={() => [
          <Button
            key="upload"
            onClick={() => {
              setAddVisible(true);
            }}
          >
            <FormattedMessage id="design.upload" />
          </Button>,
        ]}
        actionRef={actionRef}
        rowKey="package"
        search={false}
        request={(params) => {
          return getDesignList(params);
        }}
        columnsState={{
          persistenceKey: 'design-table',
          persistenceType: 'localStorage',
        }}
        pagination={false}
        columns={columns}
      />
      {addVisible && (
        <ModalForm
          width={600}
          title={intl.formatMessage({ id: 'design.upload' })}
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
          <ProFormText
            name="tpl"
            label={intl.formatMessage({ id: 'design.package.zip' })}
          >
            <Upload
              name="file"
              showUploadList={false}
              accept=".zip"
              customRequest={checkUploadZip}
            >
              <Button type="primary">
                <FormattedMessage id="design.package.zip.select" />
              </Button>
            </Upload>
          </ProFormText>
          <div>
            <p>{intl.formatMessage({ id: 'design.package.zip.tips' })}</p>
          </div>
        </ModalForm>
      )}
    </NewContainer>
  );
};

export default DesignIndex;
