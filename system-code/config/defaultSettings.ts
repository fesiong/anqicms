import { Settings as LayoutSettings } from '@ant-design/pro-layout';

const Settings: LayoutSettings & {
  pwa?: boolean;
  logo?: string;
} = {
  navTheme: 'light',
  // 拂晓蓝
  primaryColor: '#1890ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: false,
  fixSiderbar: true,
  colorWeak: false,
  title: '安企CMS',
  logo: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAEAAAABABAMAAABYR2ztAAAABGdBTUEAALGPC/xhBQAAACBjSFJNAAB6JgAAgIQAAPoAAACA6AAAdTAAAOpgAAA6mAAAF3CculE8AAAAKlBMVEUAAAASltsAh9YgnN4uod9pvOj///+s2fJXsuUAdtA7p+F8xevR6vjm9PsH5ipRAAAAAXRSTlMAQObYZgAAAAFiS0dEBmFmuH0AAAAHdElNRQfmBRsHOSPqneecAAABiklEQVRIx2NgQAWCggx4gaAgXhWMgmAgQEAepwq4PA4VgiiAkDyGCkZBDCBAQB5FBVZ5JBVgnhIaQHIIWL+iMSowEUKYAWIJhaWhgVRFuBEghmp6BxooC0JWICRWORMNTE9URFbQ5rl6NwrYNSUDRUGY5zZUJ2RPScVQcDYt7Q4+BZc121KVZPAouDTTLWPmHDwKcsuvpZUfw+cGAo4ML0cBpegKHJU0lSYhoYkqaAqc0ONaBcMNd1EAGY6kioIcmB+P4VBQOQmaHqfjUFCkqCSkCMRK6jgU5J6BgmskOpKRaAXp24FgdxmSAgFUBZCUPRu3giMmLi4uzu64FWC6QQCWeXEpYKCWAkagAkt0BZOBCgQQCtpWeqBm7haVDEWkIkZI7BB67p6TiFTSCQpq5IRGh25FQqHHOpEUMApK9qCXMLeQbAAFlRJaGWWshFIOgmIcs5QjoSDFLKkxSmuCCggV5tiMIFQfoBlAsELBMALDAMJ1FqoKbPIEq0XCFSvhqhmqAo88A5bmAQCS8mmHdD1yeAAAACV0RVh0ZGF0ZTpjcmVhdGUAMjAyMi0wNS0yN1QwNzo1NzozNSswMDowMAfM010AAAAldEVYdGRhdGU6bW9kaWZ5ADIwMjItMDUtMjdUMDc6NTc6MzUrMDA6MDB2kWvhAAAAAElFTkSuQmCC',
  pwa: false,
  iconfontUrl: '',
};

export default Settings;
