import config from '@/services/config';
import { getIntl } from '@umijs/max';
import { message } from 'antd';
import SparkMD5 from 'spark-md5';
import { getSessionStore, getStore } from './store';

/**
 * 校验是否登录
 * @param permits
 */
export const checkLogin = (permits: any): boolean => !!permits;

export const acceptedExtensions =
  '.jpg,.jpeg,.png,.gif,.webp,.svg,.bmp,.tiff,.avif,.heic,.heif,.raw,' +
  '.webm,.mp4,.mp3,.mov,.avi,.wmv,.mkv,.flv,.f4v,.m4v,.m4a,.wav,.flac,.aac,.ogg,' +
  '.zip,.rar,.7z,.gz,.bz2,.xz,.zst,.tar,.tgz,' +
  '.pdf,.ppt,.pptx,.doc,.docx,.xls,.xlsx,.txt,.csv,.tsv,.odt,.ods,.odp,.rtf,' +
  '.json,.xml,.yaml,.yml,.toml,.ini,.cfg,.conf,.env,.sql,.log,' +
  '.md,.markdown,.rst,.tex,' +
  '.js,.jsx,.ts,.tsx,.vue,.go,.py,.java,.c,.cpp,.h,.hpp,.rs,.rb,.sh,.bat,.ps1,.swift,.kt,.scala,.dart,.lua,.css,.scss,.less,.styl,' +
  '.psd,.ai,.eps,.sketch,.fig,' +
  '.ttf,.otf,.woff,.woff2,.eot';
export const queryParams = (params: any) => {
  let _result = [];
  for (let key in params) {
    if (params.hasOwnProperty(key)) {
      let value = params[key];
      if (value && value.constructor === Array) {
        value.forEach(function (_value) {
          _result.push(key + '=' + _value);
        });
      } else {
        _result.push(key + '=' + value);
      }
    }
  }
  return _result.join('&');
};

export const showNumber = (num: number) => {
  let result: any = '';
  if (num > 100000000) {
    result = (num / 100000000).toFixed(2) + '亿';
  } else if (num > 100000) {
    result = (num / 100000).toFixed(2) + '万';
  } else {
    result = num;
  }

  return result;
};

export const sleep = async (t: number) => {
  return new Promise<void>((resolve) => {
    setTimeout(() => {
      resolve();
    }, t);
  });
};

export const sizeFormat = (num: number) => {
  let result: any = '';
  if (num > 1000000) {
    result = (num / 1048576).toFixed(2) + 'MB';
  } else if (num > 500) {
    result = (num / 1024).toFixed(2) + 'KB';
  } else {
    result = num + 'B';
  }

  return result;
};

// 只支持csv，excel
export const exportFile = (
  titles: string[],
  data: any[][],
  fileType?: string,
) => {
  let type = fileType || 'csv';

  let textType = {
      csv: 'text/csv',
      xls: 'application/vnd.ms-excel',
    }[type],
    alink = document.createElement('a');

  alink.href =
    'data:' +
    textType +
    ';charset=utf-8,\ufeff' +
    encodeURIComponent(
      (function () {
        let content = '';
        if (type === 'csv') {
          content = titles.join(',') + '\r\n' + data.join('\r\n');
        } else {
          content += '<table border=1><thead><tr>';
          //表头
          for (let item of titles) {
            content += '<th>' + item + '</th>';
          }
          content += '</tr></thead>';
          //表体
          content += '<tbody>';
          for (let item of data) {
            content += '<tr>';
            for (let val of item) {
              content += '<td>' + val + '</td>';
            }
            content += '</tr>';
          }
          content += '</tbody>';
          content += '<table>';
        }

        return content;
      })(),
    );

  alink.download = 'table_' + new Date().getTime() + '.' + type;
  document.body.appendChild(alink);
  alink.click();
  document.body.removeChild(alink);
};

export const removeHtmlTag = (tag: string) => {
  let str = tag
    .replace(/<style[\S\s]+?<\/style>/g, '')
    .replace(/<script[\S\s]+?<\/script>/g, '')
    .replace(/<\/[\S\s]+?>/g, '\n')
    .replace(/<[\S\s]+?>/g, '');
  str = str
    .replaceAll(' ', ' ')
    .replaceAll('&nbsp;', ' ')
    .replaceAll('&ensp;', ' ')
    .replaceAll('&emsp;', ' ');
  str = str
    .replace(/(\n *){2,}/g, '\n')
    .replace(/[ ]{2,}/g, ' ')
    .trim();

  return str;
};

export const getWordsCount = function (str: string) {
  let n = 0;
  for (let i = 0; i < str.length; i++) {
    let ch = str.charCodeAt(i);
    if (ch > 255) {
      // 中文字符集
      n += 2;
    } else {
      n++;
    }
  }
  return n;
};

export const case2Camel = function (str: string) {
  return str
    .replaceAll('_', ' ')
    .toLowerCase()
    .replace(/( |^)[a-z]/g, (L) => L.toUpperCase())
    .replaceAll(' ', '');
};

export const downloadFile = async (
  downUrl: string,
  params?: any,
  newName?: string,
) => {
  let hide = message.loading({
    content: getIntl().formatMessage({ id: 'utils.download.preparing' }),
    key: 'loading',
    duration: 0,
  });
  let progress = 0;

  try {
    let headers: any = {
      admin: getStore('adminToken'),
      'Content-Type': 'application/json',
    };
    const sessionToken = getSessionStore('adminToken');
    if (sessionToken) {
      headers.admin = sessionToken;
    }
    const siteId = getSessionStore('site-id');
    if (siteId) {
      headers['Site-Id'] = siteId;
    }
    const subSiteId = getSessionStore('sub-site-id');
    if (subSiteId) {
      headers['Sub-Site-Id'] = subSiteId;
    }

    const response = await fetch(config.baseUrl + downUrl, {
      headers: headers,
      method: 'post',
      body: JSON.stringify(params),
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    let fileName = newName || 'file';
    const contentDisposition = response.headers.get('Content-Disposition');
    if (contentDisposition) {
      // 处理 filename*=UTF-8''encoded-filename 格式
      let utf8Match = contentDisposition.match(/filename\*=UTF-8''([^;\n]+)/i);
      if (utf8Match && utf8Match[1]) {
        fileName = decodeURIComponent(utf8Match[1]);
      } else {
        // 处理 filename="filename" 或 filename=filename 格式
        let match = contentDisposition.match(/filename=["']?([^"';\n]+)["']?/i);
        if (match && match[1]) {
          fileName = match[1];
        }
      }
    }

    // 检查是否为JSON错误响应
    const contentType = response.headers.get('Content-Type');
    if (contentType && contentType.includes('application/json')) {
      const data = await response.json();
      throw new Error(
        data.msg ||
          getIntl().formatMessage({ id: 'utils.download.failed' }),
      );
    }

    const contentLength = response.headers.get('Content-Length');
    const total = parseInt(contentLength || '0', 10);
    const reader = response.body?.getReader();
    const chunks: Uint8Array[] = [];

    if (!reader) {
      throw new Error(
        getIntl().formatMessage({ id: 'utils.download.read-error' }),
      );
    }

    let receivedLength = 0;
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      chunks.push(value);
      receivedLength += value.length;

      if (total > 0) {
        progress = (receivedLength / total) * 100;
        hide = message.loading({
          content: getIntl().formatMessage(
            { id: 'utils.download.progress' },
            { percent: Math.round(progress) },
          ),
          key: 'loading',
          duration: 0,
        });
      }
    }

    const blob = new Blob(chunks);
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);

    message.success(
      getIntl().formatMessage({ id: 'utils.download.complete' }),
    );
  } catch (error: any) {
    console.error('Download error:', error);
    message.error(
      error.message ||
        getIntl().formatMessage({ id: 'utils.download.file-failed' }),
    );
  } finally {
    hide();
  }
};

export const calculateFileMd5 = (file: any) => {
  return new Promise((resolve, reject) => {
    const chunkSize = 2 * 1024 * 1024; // 每个分片大小 2MB
    const spark = new SparkMD5.ArrayBuffer();
    const fileReader = new FileReader();
    const chunks = Math.ceil(file.size / chunkSize);
    let currentChunk = 0;

    const loadNext = () => {
      const start = currentChunk * chunkSize;
      const end =
        start + chunkSize >= file.size ? file.size : start + chunkSize;
      fileReader.readAsArrayBuffer(file.slice(start, end));
    };

    fileReader.onload = (e: any) => {
      spark.append(e.target.result); // 向 MD5 对象添加数据
      currentChunk++;

      if (currentChunk < chunks) {
        loadNext(); // 继续读取下一个分片
      } else {
        const md5Value = spark.end(); // 计算最终的 MD5
        resolve(md5Value);
      }
    };

    fileReader.onerror = () => {
      reject('File reading error');
    };

    loadNext(); // 开始读取第一个分片
  });
};

export const supportLanguages: any[] = [
  {
    label: '英语(English)',
    value: 'en',
    icon: '🇺🇸',
  },
  {
    label: '中文(简体)',
    value: 'zh-CN',
    icon: '🇨🇳',
  },
  {
    label: '中文(繁体)',
    value: 'zh-TW',
    icon: '🇨🇳',
  },
  {
    label: '德语(Deutsch)',
    value: 'de',
    icon: '🇩🇪',
  },
  {
    label: '印地语(हिन्दी)',
    value: 'hi',
    icon: '🇮🇳',
  },
  {
    label: '立陶宛语(Lietuvių)',
    value: 'lt',
    icon: '🇱🇹',
  },
  {
    label: '克罗地亚语(Hrvatski)',
    value: 'hr',
    icon: '🇭🇷',
  },
  {
    label: '拉脱维亚语(Latviešu)',
    value: 'lv',
    icon: '🇱🇻',
  },
  {
    label: '海地克里奥尔语(Kreyòl Ayisyen)',
    value: 'ht',
    icon: '🇭🇹',
  },
  {
    label: '匈牙利语(Magyar)',
    value: 'hu',
    icon: '🇭🇺',
  },
  {
    label: '亚美尼亚语(Հայերեն)',
    value: 'hy',
    icon: '🇦🇲',
  },
  {
    label: '乌克兰语(Українська)',
    value: 'uk',
    icon: '🇺🇦',
  },
  {
    label: '马尔加什语(Malagasy)',
    value: 'mg',
    icon: '🇲🇬',
  },
  {
    label: '印度尼西亚语(Bahasa Indonesia)',
    value: 'id',
    icon: '🇮🇩',
  },
  {
    label: '乌尔都语(اردو)',
    value: 'ur',
    icon: '🇵🇰',
  },
  {
    label: '马其顿语(Македонски)',
    value: 'mk',
    icon: '🇲🇰',
  },
  {
    label: '马拉雅拉姆语(മലയാളം)',
    value: 'ml',
    icon: '🇮🇳',
  },
  {
    label: '蒙古语(Монгол)',
    value: 'mn',
    icon: '🇲🇳',
  },
  {
    label: '南非荷兰语(Afrikaans)',
    value: 'af',
    icon: '🇿🇦',
  },
  {
    label: '马拉地语(मराठी)',
    value: 'mr',
    icon: '🇮🇳',
  },
  {
    label: '乌兹别克语(Oʻzbekcha)',
    value: 'uz',
    icon: '🇺🇿',
  },
  {
    label: '马来语(Bahasa Melayu)',
    value: 'ms',
    icon: '🇲🇾',
  },
  {
    label: '希腊语(Ελληνικά)',
    value: 'el',
    icon: '🇬🇷',
  },
  {
    label: '马耳他语(Malti)',
    value: 'mt',
    icon: '🇲🇹',
  },
  {
    label: '冰岛语(Íslenska)',
    value: 'is',
    icon: '🇮🇸',
  },
  {
    label: '意大利语(Italiano)',
    value: 'it',
    icon: '🇮🇹',
  },
  {
    label: '缅甸语(မြန်မာဘာသာ)',
    value: 'my',
    icon: '🇲🇲',
  },
  {
    label: '西班牙语(Español)',
    value: 'es',
    icon: '🇪🇸',
  },
  {
    label: '爱沙尼亚语(Eesti)',
    value: 'et',
    icon: '🇪🇪',
  },
  {
    label: '巴斯克语(Euskara)',
    value: 'eu',
    icon: '🇪🇸',
  },
  {
    label: '阿拉伯语(العربية)',
    value: 'ar',
    icon: '🇸🇦',
  },
  {
    label: '葡萄牙语(欧洲)(Português)',
    value: 'pt-PT',
    icon: '🇵🇹',
  },
  {
    label: '日语(日本語)',
    value: 'ja',
    icon: '🇯🇵',
  },
  {
    label: '尼泊尔语(नेपाली)',
    value: 'ne',
    icon: '🇳🇵',
  },
  {
    label: '阿塞拜疆语(Azərbaycan)',
    value: 'az',
    icon: '🇦🇿',
  },
  {
    label: '波斯语(فارسی)',
    value: 'fa',
    icon: '🇮🇷',
  },
  {
    label: '罗马尼亚语(Română)',
    value: 'ro',
    icon: '🇷🇴',
  },
  {
    label: '荷兰语(Nederlands)',
    value: 'nl',
    icon: '🇳🇱',
  },
  {
    label: '英语(英国)(English)',
    value: 'en-GB',
    icon: '🇬🇧',
  },
  {
    label: '挪威语(Norsk)',
    value: 'no',
    icon: '🇳🇴',
  },
  {
    label: '白俄罗斯语(Беларуская)',
    value: 'be',
    icon: '🇧🇾',
  },
  {
    label: '芬兰语(Suomi)',
    value: 'fi',
    icon: '🇫🇮',
  },
  {
    label: '俄语(Русский)',
    value: 'ru',
    icon: '🇷🇺',
  },
  {
    label: '保加利亚语(Български)',
    value: 'bg',
    icon: '🇧🇬',
  },
  {
    label: '法语(Français)',
    value: 'fr',
    icon: '🇫🇷',
  },
  {
    label: '波斯尼亚语(Bosanski)',
    value: 'bs',
    icon: '🇧🇦',
  },
  {
    label: '信德语(سنڌي)',
    value: 'sd',
    icon: '🇵🇰',
  },
  {
    label: '北萨米语(Davvisámegiella)',
    value: 'se',
    icon: '🇳🇴',
  },
  {
    label: '僧伽罗语(සිංහල)',
    value: 'si',
    icon: '🇱🇰',
  },
  {
    label: '斯洛伐克语(Slovenčina)',
    value: 'sk',
    icon: '🇸🇰',
  },
  {
    label: '斯洛文尼亚语(Slovenščina)',
    value: 'sl',
    icon: '🇸🇮',
  },
  {
    label: '爱尔兰语(Gaeilge)',
    value: 'ga',
    icon: '🇮🇪',
  },
  {
    label: '修纳语(Shona)',
    value: 'sn',
    icon: '🇿🇼',
  },
  {
    label: '索马里语(Soomaali)',
    value: 'so',
    icon: '🇸🇴',
  },
  {
    label: '苏格兰盖尔语(Gàidhlig)',
    value: 'gd',
    icon: '🇬🇧',
  },
  {
    label: '加泰罗尼亚语(Català)',
    value: 'ca',
    icon: '🇪🇸',
  },
  {
    label: '阿尔巴尼亚语(Shqip)',
    value: 'sq',
    icon: '🇦🇱',
  },
  {
    label: '塞尔维亚语(Српски)',
    value: 'sr',
    icon: '🇷🇸',
  },
  {
    label: '哈萨克语(Қазақша)',
    value: 'kk',
    icon: '🇰🇿',
  },
  {
    label: '塞索托语(Sesotho)',
    value: 'st',
    icon: '🇱🇸',
  },
  {
    label: '高棉语(ភាសាខ្មែរ)',
    value: 'km',
    icon: '🇰🇭',
  },
  {
    label: '卡纳达语(ಕನ್ನಡ)',
    value: 'kn',
    icon: '🇮🇳',
  },
  {
    label: '瑞典语(Svenska)',
    value: 'sv',
    icon: '🇸🇪',
  },
  {
    label: '韩语(한국어)',
    value: 'ko',
    icon: '🇰🇷',
  },
  {
    label: '斯瓦希里语(Kiswahili)',
    value: 'sw',
    icon: '🇹🇿',
  },
  {
    label: '加利西亚语(Galego)',
    value: 'gl',
    icon: '🇪🇸',
  },
  {
    label: '葡萄牙语(巴西)(Português)',
    value: 'pt-BR',
    icon: '🇧🇷',
  },
  {
    label: '科西嘉语(Corsu)',
    value: 'co',
    icon: '🇫🇷',
  },
  {
    label: '泰米尔语(தமிழ்)',
    value: 'ta',
    icon: '🇮🇳',
  },
  {
    label: '古吉拉特语(ગુજરાતી)',
    value: 'gu',
    icon: '🇮🇳',
  },
  {
    label: '吉尔吉斯语(Кыргызча)',
    value: 'ky',
    icon: '🇰🇬',
  },
  {
    label: '捷克语(Čeština)',
    value: 'cs',
    icon: '🇨🇿',
  },
  {
    label: '旁遮普语(ਪੰਜਾਬੀ)',
    value: 'pa',
    icon: '🇮🇳',
  },
  {
    label: '泰卢固语(తెలుగు)',
    value: 'te',
    icon: '🇮🇳',
  },
  {
    label: '塔吉克语(Тоҷикӣ)',
    value: 'tg',
    icon: '🇹🇯',
  },
  {
    label: '泰语(ไทย)',
    value: 'th',
    icon: '🇹🇭',
  },
  {
    label: '拉丁语(Latina)',
    value: 'la',
    icon: '🇻🇦',
  },
  {
    label: '威尔士语(Cymraeg)',
    value: 'cy',
    icon: '🇬🇧',
  },
  {
    label: '波兰语(Polski)',
    value: 'pl',
    icon: '🇵🇱',
  },
  {
    label: '丹麦语(Dansk)',
    value: 'da',
    icon: '🇩🇰',
  },
  {
    label: '土耳其语(Türkçe)',
    value: 'tr',
    icon: '🇹🇷',
  },
  {
    label: '越南语(Tiếng Việt)',
    value: 'vi',
    icon: '🇻🇳',
  },
  {
    label: '孟加拉语(Bengali)',
    value: 'bn',
    icon: '🇧🇩',
  },
  {
    label: '老挝语(Lao)',
    value: 'lo',
    icon: '🇱🇦',
  },
  {
    label: '爪哇语(Jawa)',
    value: 'jw',
    icon: '🇮🇩',
  },
  {
    label: '奥塞梯语(Ossetic)',
    value: 'os',
    icon: '🇬🇪',
  },
  {
    label: '藏语(Tibetan)',
    value: 'bo',
    icon: '🇨🇳',
  },
  {
    label: '菲律宾语(Filipino)',
    value: 'tl',
    icon: '🇵🇭',
  },
  {
    label: '刚果语(Kongo)',
    value: 'kg',
    icon: '🇨🇬',
  },
  {
    label: '格鲁吉亚语(Georgian)',
    value: 'ka',
    icon: '🇬🇪',
  },
  {
    label: '普什图语(Pashto)',
    value: 'ps',
    icon: '🇦🇫',
  },
  {
    label: '土库曼语(Turkmen)',
    value: 'tk',
    icon: '🇹🇲',
  },
  {
    label: '希伯来语(Hebrew)',
    value: 'iw',
    icon: '🇮🇱',
  },
];

export const getLanguageIcon = (lang: string) => {
  let icon = '';
  for (let item of supportLanguages) {
    if (item.value === lang) {
      icon = item.icon;
      break;
    }
  }
  return icon;
};
