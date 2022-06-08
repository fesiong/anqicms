import React, { useState, useRef, useEffect } from 'react';
import { PageContainer, FooterToolbar } from '@ant-design/pro-layout';
import type { ProColumns, ActionType } from '@ant-design/pro-table';
import ProTable from '@ant-design/pro-table';
import { Button, Space, Modal, message, Upload } from 'antd';
import { deleteDesignFileInfo, deleteDesignInfo, getDesignInfo, getDesignList, saveDesignFileInfo, saveDesignInfo, useDesignInfo } from '@/services/design';
import { history } from 'umi';
import moment from 'moment';
import { PlusOutlined } from '@ant-design/icons';
import { ModalForm, ProFormRadio, ProFormText } from '@ant-design/pro-form';

const DesignDetail: React.FC = () => {
  const [designInfo, setDesignInfo] = useState<any>({})
  const [addVisible, setAddVisible] = useState<boolean>(false)
  const [currentFile, setCurrentFile] = useState<any>({})
  const [visible, setVisible] = useState<boolean>(false)
  const actionRef = useRef<ActionType>();

  useEffect(() => {
    fetchDesignInfo();
  }, []);

  const fetchDesignInfo = async() => {
    const packageName = history.location.query?.package;
    getDesignInfo({
      package: packageName,
    }).then(res => {
      setDesignInfo(res.data);
      actionRef.current?.reload();
    }).catch(() => {
      message.error('获取模板信息出错')
    })
  }

  const handleShowEdit = (info: any) => {
    history.push('/design/editor?package=' + designInfo.package + "&path=" + info.path);
  }

  const handleRemove = (info: any) => {
    Modal.confirm({
      title: '确定要删除这个文件吗？',
      onOk: () => {
        deleteDesignFileInfo({
          package: designInfo.package,
          path: info.path,
        }).then(res => {
          message.info(res.msg)
        })
        fetchDesignInfo();
      }
    })
  }

  const getSize = (size: any) => {
    if (size < 500) {
      return size + 'B';
    }
    if (size < 1024 * 1024) {
      return (size/1024).toFixed(2) + 'KB';
    }

    return (size / 1024 / 1024).toFixed(2) + 'MB'
  }

  const handleAddFile = () => {
    setCurrentFile({})
    setAddVisible(true)
  }

  const handleAddRemark = (info: any) => {
    setCurrentFile(info)
    setAddVisible(true)
  }

  const handleSaveFile = (values: any) => {
    values.rename_path = values.path;
    values.path = currentFile.path;
    if (!values.path) {
      values.path = values.rename_path;
    }
    if (values.path.trim() == "") {
      message.error('文件名不能为空')
      return
    }
    values.package = designInfo.package;


    saveDesignFileInfo(values).then(res => {
      message.info(res.msg);
      fetchDesignInfo();
      setAddVisible(false);
    })
  }

  const handleSaveInfo = (values: any) => {
    designInfo.name = values.name;
    designInfo.template_type = values.template_type;

    saveDesignInfo(designInfo).then(res => {
      message.info(res.msg);
      fetchDesignInfo
      setVisible(false);
    })
  }


  const columns: ProColumns<any>[] = [
    {
      title: '名称',
      dataIndex: 'path',
      render: (text: any, record: any) => (<a title='点击编辑' onClick={() => {
        handleShowEdit(record)
      }}>{text}</a>),
    },
    {
      title: '备注',
      dataIndex: 'remark',
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
              handleAddRemark(record);
            }}
          >
            +备注
          </Button>
          <Button
            danger
            type="link"
            onClick={() => {
              handleRemove(record);
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
        headerTitle="设计文件管理"
        actionRef={actionRef}
        rowKey="path"
        search={false}
        toolBarRender={() => [
          <Button
            key="edit"
            onClick={() => {
              setVisible(true)
            }}
          >
            修改模板信息
          </Button>,
          <Button
            type="primary"
            key="add"
            onClick={() => {
              handleAddFile()
            }}
          >
            <PlusOutlined /> 添加新文件
          </Button>,
        ]}
        request={async(params, sort) => {
          let result: any[] = (designInfo.tpl_files || []).concat(...(designInfo.static_files || []));

          return {
            data: result,
            success: true,
          }
        }}
        pagination={false}
        columns={columns}
      />
      {addVisible && <ModalForm
          width={600}
          title={currentFile.name ? currentFile.name + '修改文件' : '添加文件'}
          visible={addVisible}
          modalProps={{
            onCancel: () => {
              setAddVisible(false);
            },
          }}
          initialValues={currentFile}
          //layout="horizontal"
          onFinish={async (values) => {
            handleSaveFile(values);
          }}
        >

          <ProFormText name="path" label="文件名" />
          <ProFormText
            name="remark"
            label="备注"
          />
        </ModalForm>}
        {visible && <ModalForm
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
        </ModalForm>}
    </PageContainer>
  );
};

export default DesignDetail;
