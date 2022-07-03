import React, { useState, useRef, useEffect } from 'react';
import { PageContainer } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import { Button, Space, Modal, message, Image, Upload } from 'antd';
import {
  deleteDesignFileInfo,
  getDesignInfo,
  saveDesignFileInfo,
  saveDesignInfo,
  UploadDesignFileInfo,
} from '@/services/design';
import { history, useModel } from 'umi';
import moment from 'moment';
import { PlusOutlined } from '@ant-design/icons';
import { ModalForm, ProFormInstance, ProFormRadio, ProFormSelect, ProFormText } from '@ant-design/pro-form';
import { downloadFile } from '@/utils';

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

  useEffect(() => {
    fetchDesignInfo();
  }, []);

  const fetchDesignInfo = async () => {
    const packageName = history.location.query?.package;
    getDesignInfo({
      package: packageName,
    })
      .then((res) => {
        setDesignInfo(res.data);
        actionRef.current?.reload();
        staticActionRef.current?.reload();

        let tmpDirs = new Set();
        for (let item of res.data.tpl_files) {
          let path = item.path.substring(0, item.path.lastIndexOf("/") + 1);
          if (!path) {
            path = "/"
          }
          tmpDirs.add(path);
        }
        let tmpList = [];
        for(let key of tmpDirs.keys()) {
          tmpList.push({
            label: key,
            value: key,
          })
        }
        setTemplateDirs(tmpList);

        tmpDirs.clear();
        for (let item of res.data.static_files) {
          let path = item.path.substring(0, item.path.lastIndexOf("/") + 1);
          if (!path) {
            path = "/"
          }
          tmpDirs.add(path);
        }
        let tmpList2 = [];
        for(let key of tmpDirs.keys()) {
          tmpList2.push({
            label: key,
            value: key,
          })
        }
        setStaticDirs(tmpList2);
      })
      .catch((err) => {
        console.log(err)
        message.error('获取模板信息出错');
      });
  };

  const handleShowEdit = (type: string, info: any) => {
    // 可编辑的文件
    if (
      info.path.indexOf('.html') !== -1 ||
      info.path.indexOf('.css') !== -1 ||
      info.path.indexOf('.less') !== -1 ||
      info.path.indexOf('.scss') !== -1 ||
      info.path.indexOf('.sass') !== -1 ||
      info.path.indexOf('.js') !== -1
    ) {
      history.push(`/design/editor?package=${designInfo.package}&type=${type}&path=${info.path}`);
    } else if (
      info.path.indexOf('.png') !== -1 ||
      info.path.indexOf('.jpg') !== -1 ||
      info.path.indexOf('.jpeg') !== -1 ||
      info.path.indexOf('.gif') !== -1 ||
      info.path.indexOf('.webp') !== -1 ||
      info.path.indexOf('.bmp') !== -1
    ) {
      Modal.info({
        icon: false,
        width: 400,
        maskClosable: true,
        title: info.path,
        content: (
          <div>
            <Image
              width={340}
              src={
                (initialState?.system?.base_url || '') + '/static/' + designInfo.package + "/" + info.path
              }
            />
          </div>
        ),
      });
    } else {
      window.open((initialState?.system?.base_url || '') + '/static/' + designInfo.package + "/" + info.path)
    }
  };

  const handleRemove = (type: string, info: any) => {
    Modal.confirm({
      title: '确定要删除这个文件吗？',
      onOk: () => {
        deleteDesignFileInfo({
          package: designInfo.package,
          type: type,
          path: info.path,
        }).then((res) => {
          message.info(res.msg);
        });
        fetchDesignInfo();
      },
    });
  };

  const getSize = (size: any) => {
    if (size < 500) {
      return size + 'B';
    }
    if (size < 1024 * 1024) {
      return (size / 1024).toFixed(2) + 'KB';
    }

    return (size / 1024 / 1024).toFixed(2) + 'MB';
  };

  const handleAddFile = (type: string) => {
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
    if (values.path.trim() == '') {
      message.error('文件名不能为空');
      return;
    }
    const hide = message.loading('正在提交中', 0);
    values.package = designInfo.package;
    values.type = addFileType;

    saveDesignFileInfo(values).then((res) => {
      message.info(res.msg);
      fetchDesignInfo();
      setEditVisible(false);
      setAddVisible(false);
    }).finally(() => {
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
      title: '确定要打包下载该模板吗？',
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
    Modal.confirm({
      title: '确定要上传文件吗？',
      content: `你上传的文件将存放到${addFileType == 'static' ? '资源' : '模板'}目录：${values.path} 中。`,
      onOk: async () => {
        let formData = new FormData();
        formData.append('file', e.file);
        formData.append('package', designInfo.package);
        formData.append('type', addFileType);
        formData.append('path', values.path);

        const hide = message.loading('正在提交中', 0);
        UploadDesignFileInfo(formData).then((res) => {
          if (res.code !== 0 ){
            message.info(res.msg);
          } else {
            message.info(res.msg || '上传成功');
            setAddVisible(false);
            actionRef.current?.reload();
          }
        }).finally(() => {
          hide();
        });
      },
    });
  }

  const columns: ProColumns<any>[] = [
    {
      title: '名称',
      dataIndex: 'path',
      render: (text: any, record: any) => (
        <a
          title="点击编辑"
          onClick={() => {
            handleShowEdit('template', record);
          }}
        >
          {text}
        </a>
      ),
    },
    {
      title: '备注',
      dataIndex: 'remark',
      width: 200,
    },
    {
      title: '大小',
      dataIndex: 'size',
      width: 150,
      render: (text: any, record: any) => <div>{getSize(text)}</div>,
    },
    {
      title: '修改时间',
      dataIndex: 'last_mod',
      width: 200,
      render: (text: any) => moment((text as number) * 1000).format('YYYY-MM-DD HH:mm'),
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
              handleAddRemark('template', record);
            }}
          >
            +备注
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              handleRemove('template', record);
            }}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const columnsStatic: ProColumns<any>[] = [
    {
      title: '名称',
      dataIndex: 'path',
      render: (text: any, record: any) => (
        <a
          title="点击编辑"
          onClick={() => {
            handleShowEdit('static', record);
          }}
        >
          {text}
        </a>
      ),
    },
    {
      title: '备注',
      dataIndex: 'remark',
      width: 200,
    },
    {
      title: '大小',
      dataIndex: 'size',
      width: 150,
      render: (text: any, record: any) => <div>{getSize(text)}</div>,
    },
    {
      title: '修改时间',
      dataIndex: 'last_mod',
      width: 200,
      render: (text: any) => moment((text as number) * 1000).format('YYYY-MM-DD HH:mm'),
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
              handleAddRemark('static', record);
            }}
          >
            +备注
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              handleRemove('static', record);
            }}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title={designInfo.name + ' 文件管理'}>
      <ProTable<any>
        headerTitle="模板文件管理"
        actionRef={actionRef}
        rowKey="path"
        search={false}
        toolBarRender={() => [
          <Button
            key="download"
            onClick={() => {
              handleDownload();
            }}
          >
            打包下载模板
          </Button>,
          <Button
            key="edit"
            onClick={() => {
              setVisible(true);
            }}
          >
            修改模板信息
          </Button>,
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleAddFile('template');
            }}
          >
            <PlusOutlined /> 添加新文件
          </Button>,
        ]}
        request={async (params, sort) => {
          return {
            data: designInfo.tpl_files || [],
            success: true,
          };
        }}
        pagination={false}
        columns={columns}
      />
      <ProTable<any>
        headerTitle="资源文件"
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
            <PlusOutlined /> 添加新资源
          </Button>,
        ]}
        request={async (params, sort) => {
          return {
            data: designInfo.static_files || [],
            success: true,
          };
        }}
        pagination={false}
        columns={columnsStatic}
      />
      {addVisible && (
        <ModalForm
          width={600}
          title={'添加新'+(addFileType == 'static' ? '资源' : '模板')+'文件'}
          formRef={formRef}
          visible={addVisible}
          modalProps={{
            onCancel: () => {
              setAddVisible(false);
            },
          }}
          //layout="horizontal"
          onFinish={async (values) => {
            setAddVisible(false);
          }}
        >
          <ProFormSelect
            label="存放目录"
            showSearch
            name="path"
            width="lg"
            request={async () => {
              return addFileType == 'static' ? staticDirs : templateDirs;
            }}
          />
          <ProFormText name="tpl" label="模板文件">
            <Upload
                    name="file"
                    showUploadList={false}
                    customRequest={handleUploadTemplate}
                  >
                    <Button type="primary">选择文件</Button>
                  </Upload>
            </ProFormText>
            <div>
              <p>说明：只能上传模板文件(.html)、和资源文件(css,js,图片,字体等)，以及zip的文件。如果上传zip,会自动解压到当前目录。</p>
            </div>
        </ModalForm>
      )}
      {editVisible && (
        <ModalForm
          width={600}
          title={currentFile.name + '修改文件'}
          visible={editVisible}
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
          <ProFormText name="path" label="文件名" />
          <ProFormText name="remark" label="备注" />
        </ModalForm>
      )}
      {visible && (
        <ModalForm
          width={600}
          title={'修改模板'}
          visible={visible}
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
          <ProFormText name="name" label="模板名称" />
          <ProFormRadio.Group
            name="template_type"
            label="模板类型"
            extra="自适应类型模板只有一个域名和一套模板；代码适配类型有一个域名和2套模板，电脑端和手机端访问同一个域名会展示不同模板；电脑+手机类型需要2个域名和2套模板，访问电脑域名展示电脑模板，访问手机域名展示手机模板。"
            options={[
              {
                value: 0,
                label: '自适应',
              },
              {
                value: 1,
                label: '代码适配',
              },
              {
                value: 2,
                label: '电脑+手机',
              },
            ]}
          />
        </ModalForm>
      )}
    </PageContainer>
  );
};

export default DesignDetail;
