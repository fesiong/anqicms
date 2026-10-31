import { FormattedMessage, history, useIntl } from '@umijs/max';
import { Button, Result } from 'antd';
import React from 'react';

const NoFoundPage: React.FC = () => {
  const intl = useIntl();
  return (
    <Result
      status="404"
      title="404"
      subTitle={intl.formatMessage({ id: 'pages.404.description' })}
      extra={
        <Button type="primary" onClick={() => history.push('/')}>
          <FormattedMessage id="pages.404.back-home" />
        </Button>
      }
    />
  );
};

export default NoFoundPage;
