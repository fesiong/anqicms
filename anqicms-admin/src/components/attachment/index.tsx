import { useIntl } from '@umijs/max';
import { Modal } from 'antd';
import React, { useState } from 'react';
import AttachmentContent from './content';
import './index.less';

export type AttachmentProps = {
  onSelect: (row?: any) => void;
  onCancel?: (row?: any) => void;
  open?: boolean;
  manual?: boolean;
  multiple?: boolean;
  children?: React.ReactNode;
};

const AttachmentSelect: React.FC<AttachmentProps> = (props) => {
  const [visible, setVisible] = useState<boolean>(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<any[]>([]);
  const intl = useIntl();

  const visibleControl = (flag: boolean) => {
    if (props.manual) {
      props.onCancel?.(flag);
    } else {
      setVisible(flag);
    }
  };

  const setDetail = (row: any) => {
    if (!props.multiple) {
      props.onSelect(row);
      visibleControl(false);
    } else {
      setSelectedRowKeys(row);
    }
  };

  const onSubmit = () => {
    if (props.multiple) {
      props.onSelect(selectedRowKeys);
    }
    visibleControl(false);
  };

  return (
    <>
      <div
        style={{ display: 'inline-block' }}
        onClick={() => {
          visibleControl(!visible);
        }}
      >
        {props.children}
      </div>
      <Modal
        className="material-modal"
        width={950}
        open={props.manual ? props.open : visible}
        onCancel={() => {
          visibleControl(false);
        }}
        onOk={() => {
          onSubmit();
        }}
      >
        <AttachmentContent
          multiple={props.multiple}
          onSelect={(e: any) => {
            setDetail(e);
          }}
          intl={intl}
        />
      </Modal>
    </>
  );
};

export default AttachmentSelect;
