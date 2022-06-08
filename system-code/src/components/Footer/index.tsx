import { GithubOutlined } from '@ant-design/icons';
import { DefaultFooter } from '@ant-design/pro-layout';

const Footer: React.FC = () => {
  const defaultMessage = 'kandaoni.com';

  const currentYear = new Date().getFullYear();

  return (
    <DefaultFooter
      copyright={`${currentYear} ${defaultMessage}`}
      links={[
        {
          key: 'kandaoni.com/template',
          title: '模板使用教程',
          href: 'https://www.kandaoni.com/category/10',
          blankTarget: true,
        },
      ]}
    />
  );
};

export default Footer;
