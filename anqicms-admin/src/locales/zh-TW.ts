import account from './zh-TW/account';
import ai from './zh-TW/ai';
import common from './zh-TW/common';
import component from './zh-TW/component';
import content from './zh-TW/content';
import dashboard from './zh-TW/dashboard';
import design from './zh-TW/design';
import menu from './zh-TW/menu';
import pages from './zh-TW/pages';
import plugin from './zh-TW/plugin';
import pwa from './zh-TW/pwa';
import setting from './zh-TW/setting';
import statistic from './zh-TW/statistic';
import tool from './zh-TW/tool';
import website from './zh-TW/website';

export default {
  'site.title': 'AnQiCMS',
  ...pages,
  ...menu,
  ...setting,
  ...pwa,
  ...component,
  ...dashboard,
  ...tool,
  ...website,
  ...account,
  ...ai,
  ...statistic,
  ...design,
  ...content,
  ...plugin,
  ...common,
};
