import React, { useState } from 'react';
import { Modal } from 'antd';
import ProTable, { ProColumns } from '@ant-design/pro-table';
import { pluginGetKeywords } from '@/services/plugin/keyword';

export type KeywordsProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: string[]) => Promise<void>;
  visible: boolean;
};

const Keywords: React.FC<KeywordsProps> = (props) => {
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);

  const columns: ProColumns<any>[] = [
    {
      title: '关键词',
      dataIndex: 'title',
    },
  ];

  return (
    <Modal
      width={600}
      title="选择关键词"
      visible={props.visible}
      onCancel={() => {
        props.onCancel();
      }}
      onOk={() => {
        props.onSubmit(selectedRowKeys);
      }}
    >
      <ProTable<any>
        rowKey="title"
        search={{
          span: 12,
          labelWidth: 120,
        }}
        toolBarRender={false}
        tableAlertRender={false}
        tableAlertOptionRender={false}
        request={(params, sort) => {
          return pluginGetKeywords(params);
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          defaultPageSize: 10,
        }}
      />
    </Modal>
  );
};

export default Keywords;
