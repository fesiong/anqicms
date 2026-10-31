import { ProLayoutProps } from '@ant-design/pro-components';

const Settings: ProLayoutProps & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'light',
  // 拂晓蓝
  colorPrimary: '#1890ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: false,
  fixSiderbar: true,
  colorWeak: false,
  logo: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABACAMAAACdt4HsAAAASFBMVEUAAAABZf8AZf8BZf8BZf8BZf8AZP8AZP8AZf8AZP8AZf8BZP8AZP8BZf8BZP8BZf8AZf8BZf8AZf8AZv8AZP8AZP8BZP8BZf9/EyIJAAAAF3RSTlMA9RDg65t/IkM4LspyvrSOaKtMGlVd04/Ly7sAAAHfSURBVFjD7ZZJdsQgDERbiMkYMJ6a+980nZGAwIRkk0XX0rb0iuKDfHvqqSsxswUpw2bYr6r1MnOIDwGf7Xj95iEmLaPlSkD8JtCD9XqOmWY2aB9jLjlWP5X1fBpb/xwL+bEViFgqjAXIy3pUQw1WYkCMJUgMwDHUQBID933oBJAtGMR4A7ICPbyHbYyZOeQiFnk0D7hKEFKMlXX8k01n63t7QmxhvMu8Ocr9AgKKsabpOnpEDLYwPjFSISHExgbGJ4818bOAwDUw1hjrQt3FODvgVPP+E4yX2JbsYFxES4Wmg3FhAEU4gsDMQg/j/Z4aruZ9u9fsmw7GOhkT7BPL1AGmHsY2RapqyIUexqKa+JKC7mHss0hp4msPY1+dLxN8+exgfPMkrjzatXcbCxJXbli0MS7fOFY7d7J3Gx/w5ckmA+nh1sSYAsbth4fAy6hFDQIKCPjTKKUFlFErrEdI3QFHBGqUYrx+C8zHtlZWxxini4lPvzN4fVtaYrDYF0ufZ2Li6kqkGPNAhraAugPBKmuct8rYl7zeYSEQoFT1we+gmUJ6BbM0zX+P0/PGbNm1Fd45L+zELn9fpiDcHRH5m+ChGKNTH29JbVWM7UqZV00P6SMsTt7+JqZuT/0DvQA1ZYV3fzevuwAAAABJRU5ErkJggg==',
  pwa: false,
  iconfontUrl: '',
  token: {
    // 参见ts声明，demo 见文档，通过token 修改样式
    //https://procomponents.ant.design/components/layout#%E9%80%9A%E8%BF%87-token-%E4%BF%AE%E6%94%B9%E6%A0%B7%E5%BC%8F
  },
};

export default Settings;
