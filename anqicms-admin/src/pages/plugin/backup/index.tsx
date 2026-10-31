import NewContainer from '@/components/NewContainer';
import {
  pluginBackupCleanup,
  pluginBackupData,
  pluginBackupDelete,
  pluginBackupImport,
  pluginBackupRemark,
  pluginBackupRestore,
  pluginGetBackupList,
  pluginGetBackupStatus,
} from '@/services';
import config from '@/services/config';
import { calculateFileMd5, sizeFormat } from '@/utils';
import { getSessionStore, getStore } from '@/utils/store';
import { ActionType, ProColumns, ProTable } from '@ant-design/pro-components';
import { FormattedMessage, useIntl } from '@umijs/max';
import {
  Button,
  Input,
  Modal,
  Progress,
  Radio,
  Space,
  Tag,
  Upload,
  message,
} from 'antd';
import dayjs from 'dayjs';
import React, { useEffect, useRef, useState } from 'react';

let running = false;
let intXhr: any = null;

const PluginBackup: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const [task, setTask] = useState<any>(null);
  const [newKey, setNewKey] = useState<string>('');

  let cleanUploads = false;
  const intl = useIntl();

  const syncTask = async () => {
    pluginGetBackupStatus().then((res) => {
      if (res.data) {
        running = true;
        setTask(res.data);
      } else {
        if (running) {
          running = false;
          actionRef.current?.reload();
        }
        clearInterval(intXhr);
        setTask(null);
      }
    });
  };

  const onTabChange = (key: string) => {
    running = false;
    clearInterval(intXhr);
    // 定时查询task
    intXhr = setInterval(() => {
      syncTask();
    }, 1000);

    setNewKey(key);
  };

  useEffect(() => {
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

  const handleBackupData = async () => {
    if (running) {
      return;
    }

    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.confirm' }),
      onOk: () => {
        running = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.backup.backuping' }),
          0,
        );
        pluginBackupData({})
          .then((res) => {
            message.info(res.msg);
            // 马上执行一遍
            intXhr = setInterval(() => {
              syncTask();
            }, 1000);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleBackupRestore = async (record: any) => {
    if (running) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.restore.confirm' }),
      content: intl.formatMessage({ id: 'plugin.backup.restore.content' }),
      onOk: () => {
        running = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.backup.restoring' }),
          0,
        );
        pluginBackupRestore(record)
          .then(async (res) => {
            message.info(res.msg);
            // 马上执行一遍
            intXhr = setInterval(() => {
              syncTask();
            }, 1000);
          })
          .finally(() => {
            hide();
          });
      },
    });
  };

  const handleDelete = (row: any) => {
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.delete.confirm' }),
      onOk: () => {
        pluginBackupDelete(row).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleRemark = (record: any) => {
    let remark = record.remark || '';
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.remark.title' }),
      content: (
        <Input.TextArea
          rows={3}
          defaultValue={remark}
          onChange={(e) => {
            remark = e.target.value;
          }}
          placeholder={intl.formatMessage({
            id: 'plugin.backup.remark.placeholder',
          })}
        />
      ),
      onOk: () => {
        return pluginBackupRemark({
          name: record.name,
          remark: remark,
        }).then((res) => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  };

  const handleDownloadBackup = (record: any) => {
    // 使用浏览器原生下载（window.open），避免大文件全量缓冲到内存。
    // 认证 token 通过 query string 传递，后端 ParseAdminToken 已支持 fallback。
    const token = getSessionStore('adminToken') || getStore('adminToken') || '';
    const siteId = getSessionStore('site-id') || '';
    const params = new URLSearchParams({
      name: record.name,
      token,
      site_id: siteId,
    });
    const url = `${config.baseUrl}/plugin/backup/export?${params.toString()}`;
    window.open(url, '_blank');
  };

  const handleUploadFile = async (e: any) => {
    const formData = new FormData();
    let hide = message.loading({
      content: intl.formatMessage({
        id: 'setting.system.submitting',
      }),
      duration: 0,
      key: 'uploading',
    });
    const size = e.file.size;
    const md5Value = await calculateFileMd5(e.file);
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const totalChunks = Math.ceil(size / chunkSize);
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
          const res = await pluginBackupImport(formData);
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
      pluginBackupImport(formData)
        .then((res) => {
          message.success(res.msg);
          actionRef.current?.reload();
        })
        .finally(() => {
          hide();
        });
    }
  };

  const onChangeData = (e: any) => {
    cleanUploads = e.target.value;
  };

  const handleCleanup = async () => {
    if (running) {
      return;
    }
    Modal.confirm({
      title: intl.formatMessage({ id: 'plugin.backup.cleanup.confirm' }),
      content: (
        <div>
          <p>{intl.formatMessage({ id: 'plugin.backup.cleanup.tips1' })}</p>
          <p>{intl.formatMessage({ id: 'plugin.backup.cleanup.tips2' })}</p>
          <Radio.Group onChange={onChangeData}>
            <Radio value={false}>
              {intl.formatMessage({ id: 'plugin.backup.cleanup.upload.false' })}
            </Radio>
            <Radio value={true}>
              {intl.formatMessage({ id: 'plugin.backup.cleanup.upload.true' })}
            </Radio>
          </Radio.Group>
        </div>
      ),
      onOk: () => {
        running = true;
        const hide = message.loading(
          intl.formatMessage({ id: 'plugin.backup.cleaning' }),
          0,
        );
        pluginBackupCleanup({
          clean_uploads: cleanUploads,
        })
          .then((res) => {
            message.success(res.msg);
          })
          .finally(() => {
            hide();
            running = false;
          });
      },
    });
  };

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'plugin.backup.time' }),
      hideInSearch: true,
      dataIndex: 'last_mod',
      render: (item) => {
        if (`${item}` === '0') {
          return false;
        }
        return dayjs((item as number) * 1000).format('YYYY-MM-DD HH:mm');
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.backup.name' }),
      dataIndex: 'name',
    },
    {
      title: intl.formatMessage({ id: 'plugin.backup.format' }),
      dataIndex: 'name',
      width: 80,
      render: (_, record) => {
        if (record.name.endsWith('.zip')) {
          return <Tag color="blue">ZIP</Tag>;
        }
        return <Tag>SQL</Tag>;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.backup.remark' }),
      dataIndex: 'remark',
      width: 180,
      ellipsis: true,
      render: (text) => {
        if (!text) {
          return <span style={{ color: '#999' }}>-</span>;
        }
        return text;
      },
    },
    {
      title: intl.formatMessage({ id: 'plugin.backup.size' }),
      dataIndex: 'size',
      render: (item) => {
        return sizeFormat(item as number);
      },
    },
    {
      title: intl.formatMessage({ id: 'setting.action' }),
      dataIndex: 'option',
      valueType: 'option',
      render: (_, record) => (
        <Space size={20}>
          <a
            key="edit"
            onClick={() => {
              handleBackupRestore(record);
            }}
          >
            <FormattedMessage id="plugin.backup.restore" />
          </a>
          <a
            key="edit"
            onClick={() => {
              handleDownloadBackup(record);
            }}
          >
            <FormattedMessage id="plugin.backup.download" />
          </a>
          <a
            onClick={() => {
              handleRemark(record);
            }}
          >
            <FormattedMessage id="plugin.backup.remark.edit" />
          </a>
          <a
            onClick={() => {
              handleDelete(record);
            }}
          >
            <FormattedMessage id="setting.system.delete" />
          </a>
        </Space>
      ),
    },
  ];

  return (
    <NewContainer onTabChange={(key) => onTabChange(key)}>
      <ProTable<any>
        key={newKey}
        actionRef={actionRef}
        rowKey="id"
        toolBarRender={() => [
          <Button type="primary" key="add" onClick={() => handleBackupData()}>
            <FormattedMessage id="plugin.backup.new" />
          </Button>,
          <Upload
            key="upload"
            name="file"
            className="logo-uploader"
            showUploadList={false}
            accept=".sql"
            customRequest={handleUploadFile}
          >
            <Button type="primary">
              <FormattedMessage id="plugin.backup.import" />
            </Button>
          </Upload>,
          <Button key="clean" onClick={() => handleCleanup()}>
            <FormattedMessage id="plugin.backup.cleanup" />
          </Button>,
        ]}
        search={false}
        tableAlertOptionRender={false}
        request={(params) => {
          return pluginGetBackupList(params);
        }}
        columnsState={{
          persistenceKey: 'backup-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={false}
        pagination={{
          showSizeChanger: true,
        }}
        summary={() => (
          <tr>
            <td colSpan={6}>
              <div style={{ marginTop: 10 }}>
                <FormattedMessage id="plugin.backup.tips" />
              </div>
            </td>
          </tr>
        )}
      />
      {task !== null && (
        <Modal
          title={
            task.type === 'backup'
              ? intl.formatMessage({ id: 'plugin.backup.new' })
              : intl.formatMessage({ id: 'plugin.backup.restore' })
          }
          open={true}
          footer={null}
        >
          <div className="task-progress">
            <Progress percent={task.finished ? 100 : task.percent} />
          </div>
          <div className="task-message">{task.message}</div>
        </Modal>
      )}
    </NewContainer>
  );
};

export default PluginBackup;
