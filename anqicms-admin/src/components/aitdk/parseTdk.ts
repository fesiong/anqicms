/**
 * parseTdkJson 从 AI 返回的文本中提取 TDK JSON 对象。
 *
 * 兼容以下 AI 返回格式：
 * 1. 纯 JSON：{"title": "...", ...}
 * 2. ```json 代码块包裹：
 *    ```json
 *    {"title": "...", ...}
 *    ```
 * 3. 重复内容：AI 先返回纯 JSON，又返回 ```json 代码块（取第一个有效 JSON）
 * 4. 前后多余说明文字：取第一个能成功 parse 的 {...} 片段
 * 5. keywords 为数组时自动 join 为逗号分隔字符串
 *
 * 返回值：
 *   - 成功：{ title, description, keywords, title_ok, description_ok, keywords_ok }
 *   - 失败：null
 */
export function parseTdkJson(content: string): any | null {
  if (!content || (!content.includes('{') && !content.includes('}'))) {
    return null;
  }

  // 策略 1：提取所有 ```json ... ``` 代码块，逐个尝试 parse
  const codeBlockRegex = /```(?:json)?\s*\n?([\s\S]*?)```/g;
  let match: RegExpExecArray | null;
  while ((match = codeBlockRegex.exec(content)) !== null) {
    const jsonStr = match[1].trim();
    const parsed = tryParseTdk(jsonStr);
    if (parsed) return parsed;
  }

  // 策略 2：提取所有 {...} 片段（贪婪匹配最外层花括号），逐个尝试 parse
  // 用栈匹配找到平衡的花括号对
  const fragments = extractBalancedJsonFragments(content);
  for (const frag of fragments) {
    const parsed = tryParseTdk(frag);
    if (parsed) return parsed;
  }

  return null;
}

/**
 * tryParseTdk 尝试 parse JSON 字符串并规范化 TDK 字段。
 * 返回规范化的 TDK 对象，或 null（parse 失败或缺少 title）。
 */
function tryParseTdk(jsonStr: string): any | null {
  try {
    const parsed: any = JSON.parse(jsonStr);
    if (!parsed || typeof parsed !== 'object') return null;
    // 必须至少有 title 字段才算有效的 TDK
    if (!parsed.title) return null;

    // keywords 为数组时 join 为逗号分隔
    if (typeof parsed.keywords === 'object' && Array.isArray(parsed.keywords)) {
      parsed.keywords = parsed.keywords.join(',');
    }

    // 统一设置 _ok 标志（前端 Checkbox 用）
    parsed.title_ok = true;
    parsed.description_ok = true;
    parsed.keywords_ok = true;

    return parsed;
  } catch (e) {
    return null;
  }
}

/**
 * extractBalancedJsonFragments 从文本中提取所有平衡的 {...} 片段。
 *
 * 用栈匹配花括号，找到所有合法的 JSON 对象片段。
 * 返回片段数组，按出现顺序排列。
 */
function extractBalancedJsonFragments(text: string): string[] {
  const fragments: string[] = [];
  const stack: number[] = []; // 记录 '{' 的位置
  let inString = false; // 是否在字符串内
  let escape = false; // 前一个字符是否为转义符 '\'

  for (let i = 0; i < text.length; i++) {
    const ch = text[i];

    if (escape) {
      escape = false;
      continue;
    }

    if (ch === '\\') {
      escape = true;
      continue;
    }

    if (ch === '"') {
      inString = !inString;
      continue;
    }

    // 字符串内的花括号不参与栈匹配
    if (inString) continue;

    if (ch === '{') {
      stack.push(i);
    } else if (ch === '}') {
      if (stack.length > 0) {
        const start = stack.pop()!;
        // 栈空时表示找到一对完整的平衡花括号
        if (stack.length === 0) {
          fragments.push(text.slice(start, i + 1));
        }
      }
    }
  }

  return fragments;
}
