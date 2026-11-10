import { pluginGetKeywords } from '@/services/plugin/keyword';
import { ProColumns, ProTable } from '@ant-design/pro-components';
import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import React, { useState } from 'react';

export type KeywordsProps = {
  onCancel: (flag?: boolean) => void;
  onSubmit: (values: string[]) => Promise<void>;
  open: boolean;
};

const Keywords: React.FC<KeywordsProps> = (props) => {
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const intl = useIntl();

  const columns: ProColumns<any>[] = [
    {
      title: intl.formatMessage({ id: 'component.keywords.title' }),
      dataIndex: 'title',
    },
  ];

  return (
    <Modal
      width={600}
      title={intl.formatMessage({ id: 'component.keywords.select' })}
      open={props.open}
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
        request={(params) => {
          return pluginGetKeywords(params);
        }}
        columnsState={{
          persistenceKey: 'keywords-table',
          persistenceType: 'localStorage',
        }}
        columns={columns}
        rowSelection={{
          onChange: (selectedRowKeys) => {
            setSelectedRowKeys(selectedRowKeys);
          },
        }}
        pagination={{
          defaultPageSize: 10,
          showSizeChanger: true,
          showQuickJumper: true,
        }}
      />
    </Modal>
  );
};

export default Keywords;
