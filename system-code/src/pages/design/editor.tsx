import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import MonacoEditor from 'react-monaco-editor';
import { Button, Card, Col, message, Row, Space, Collapse, Modal } from 'antd';
import { history } from 'umi';
import { deleteDesignHistoryFile, getDesignFileHistories, getDesignFileInfo, getDesignInfo, restoreDesignFileInfo, saveDesignFileInfo } from '@/services/design';
import './index.less';
import ProTable, { ActionType, ProColumns } from '@ant-design/pro-table';
import moment from 'moment';
import { getLanguage } from '@ant-design/pro-layout/lib/locales';

const DesignEditor: React.FC = () => {
  const [fileInfo, setFileInfo] = useState<any>({});
  const [designInfo, setDesignInfo] = useState<any>({});
  const [showHistory, setShowHistory] = useState<boolean>(false);
  const [code, setCode] = useState<string>(``);
  const actionRef = useRef<ActionType>();
  const [loaded, setLoaded] = useState<boolean>(false);

  var unsave = false;

  useEffect(() => {
    fetchDesignInfo();

    const path = history.location.query?.path || '';
    fetchDesignFileInfo(path);
  }, []);

  const fetchDesignInfo = async () => {
    const packageName = history.location.query?.package;
    getDesignInfo({
      package: packageName,
    })
      .then((res) => {
        setDesignInfo(res.data);
      })
      .catch(() => {
        message.error('获取模板信息出错');
      });
  };

  const fetchDesignFileInfo = async (path: any) => {
    const packageName = history.location.query?.package;
    setLoaded(false);
    getDesignFileInfo({
      package: packageName,
      path: path,
    })
      .then((res) => {
        setFileInfo(res.data);
        setCode(res.data.content || '');
        setLoaded(true);
        actionRef.current?.reload();
      })
      .catch(() => {
        message.error('获取模板信息出错');
      });
  };

  const editorDidMount = (editor: any, monaco: any) => {
    //console.log('editorDidMount', editor);
    //editor.focus();
  }

  const onChangeCode = (newCode: string) => {
    if (code != newCode) {
      setCode(newCode);
      unsave = true;
    }
  };

  const handleSave = () => {
    fileInfo.content = code;
    fileInfo.package = designInfo.package;
    fileInfo.update_content = true;
    unsave = false;
    saveDesignFileInfo(fileInfo).then((res) => {
      message.info(res.msg);
      actionRef.current?.reload();
    });
  };

  const handleEditFile = (info: any) => {
    if (unsave) {
      Modal.confirm({
        title: '你有未保存的代码，确定要编辑新文件吗？',
        content: '这么做将会导致未保存的代码丢失。',
        onOk: () => {
          fetchDesignFileInfo(info.path);
        },
      });
    } else {
      fetchDesignFileInfo(info.path);
    }
  };

  const handleRestore = (info: any) => {
    Modal.confirm({
      title: '确定要恢复到指定时间的版本吗？',
      content: '这么做将会导致未保存的代码丢失。',
      onOk: () => {
        restoreDesignFileInfo({
          hash: info.hash,
          package: designInfo.package,
          path: fileInfo.path,
        }).then(res => {
          message.info(res.msg);
          fetchDesignFileInfo(info.path);
        });
      },
    });
  }

  const deleteHistoryFile = (info: any) => {
    Modal.confirm({
      title: '确定要删除这个历史记录吗？',
      onOk: () => {
        deleteDesignHistoryFile({
          hash: info.hash,
          package: designInfo.package,
          path: fileInfo.path,
        }).then(res => {
          message.info(res.msg);
          actionRef.current?.reload();
        });
      },
    });
  }

  const handleKeyDown = (event: React.KeyboardEvent) => {
    if (event.key === "s" && (event.ctrlKey || event.metaKey)) {

      // 自动保存
      handleSave();

      event.preventDefault();
    }
  };

  const getSize = (size: any) => {
    if (size < 500) {
      return size + 'B';
    }
    if (size < 1024 * 1024) {
      return (size/1024).toFixed(2) + 'KB';
    }

    return (size / 1024 / 1024).toFixed(2) + 'MB'
  }

  const getLanguage = (filePath: string) => {
    return filePath.indexOf('.html') !== -1 ? 'html' : filePath.indexOf('.css') !== -1 ? 'css' : 'javascript';
  }

  const getHeight = () => {
    let num = window?.innerHeight - 260;
    if (num < 450) {
      num = 450;
    } else if (num > 900) {
      num = 900;
    }

    return num;
  }

  const columns: ProColumns<any>[] = [
    {
      title: 'Hash',
      dataIndex: 'hash',
    },
    {
      title: '大小',
      dataIndex: 'size',
      render: (text: any, record: any) => (<div>{getSize(text)}</div>),
    },
    {
      title: '修改时间',
      dataIndex: 'last_mod',
      render: (text: any) => (moment((text as number) * 1000).format('YYYY-MM-DD HH:mm'))
    },
    {
      title: '操作',
      key: 'action',
      width: 100,
      render: (text: any, record: any) => (
        <Space size={16}>
          <Button
            type="link"
            onClick={() => {
              handleRestore(record);
            }}
          >
            恢复
          </Button><Button
            danger
            type="link"
            onClick={() => {
              deleteHistoryFile(record);
            }}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title={<div>正在编辑: {fileInfo?.path}</div>}>
      <Card>
        <Row gutter={16}>
          <Col span={18}>
            <div className='code-editor-box' onKeyDown={handleKeyDown}>
            {loaded && <MonacoEditor
              height={getHeight()}
              language={getLanguage(fileInfo?.path || '')}
              theme="vs-dark"
              value={code}
              options={{
                selectOnLineNumbers: false,
                wordWrap: 'on'
              }}
              onChange={onChangeCode}
              editorDidMount={editorDidMount}
            />}
            </div>
            <div className="mt-normal">
              <Space size={16}>
                <Button
                  type="primary"
                  onClick={() => {
                    handleSave();
                  }}
                >
                  保存
                </Button>
                <Button
                  onClick={() => {
                    history.goBack();
                  }}
                >
                  返回
                </Button>
                <Button
                  onClick={() => {
                    setShowHistory(true)
                  }}
                >
                  查看历史
                </Button>
              </Space>
            </div>
          </Col>
          <Col span={6}>
            <Collapse defaultActiveKey={['1']}>
              <Collapse.Panel className="tpl-file-list" showArrow={false} header="模板文件" key="1">
                {designInfo?.tpl_files?.map((item: any, index: number) => (
                  <div
                    key={index}
                    className={'tpl-item link ' + (fileInfo.path == item.path ? 'active' : '')}
                    onClick={() => handleEditFile(item)}
                  >
                    <div className="name">{item.path}</div>
                    <div className="extra">{item.remark}</div>
                  </div>
                ))}
              </Collapse.Panel>
              <Collapse.Panel className="tpl-file-list" showArrow={false} header="资源文件" key="2">
                {designInfo?.static_files?.map((item: any, index: number) => {
                  if (item.path.indexOf('.js') !== -1 || item.path.indexOf('.css') !== -1) {
                    return (
                      <div
                        key={index}
                        className={'tpl-item link ' + (fileInfo.path == item.path ? 'active' : '')}
                        onClick={() => handleEditFile(item)}
                      >
                        <div className="name">{item.path}</div>
                        <div className="extra">{item.remark}</div>
                      </div>
                    );
                  }
                  return null;
                })}
              </Collapse.Panel>
            </Collapse>
          </Col>
        </Row>
      </Card>
      <Modal title='文件历史' visible={showHistory} onCancel={() => {setShowHistory(false)}} onOk={() => {setShowHistory(false)}} width={800}>
      <ProTable<any>
        headerTitle="设计文件管理"
        actionRef={actionRef}
        rowKey="path"
        search={false}
        toolBarRender={false}
        request={async(params, sort) => {
          params.package = designInfo.package;
          params.path = fileInfo.path;
          return getDesignFileHistories(params)
        }}
        pagination={false}
        columns={columns}
      />
      </Modal>
    </PageContainer>
  );
};

export default DesignEditor;
