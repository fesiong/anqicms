import { Message, Segment } from './types';

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

/** 将后端历史 ChatMessage[] 转换为前端 Message[]（含 segments） */
export function convertHistoryMessages(raw: any[]): Message[] {
  const result: Message[] = [];
  let pending: { msg: Message; segIdx: number } | null = null;

  for (const item of raw) {
    if (!item || !item.role) continue;

    // user → 直接产出完整 message
    if (item.role === 'user') {
      pending = null;
      result.push({
        role: 'user',
        segments: [{ type: 'text' as const, content: item.content || '' }],
        timestamp: item.created_time || item.timestamp || Date.now(),
        files: item.files,
      });
      continue;
    }

    // assistant → 新建一个 assistant message，后续 tool 消息会追加到它
    if (item.role === 'assistant') {
      const segs: Segment[] = [];
      const ts = item.created_time || item.timestamp || Date.now();

      // 如果有 tool_calls（JSON 字符串），解析为 tool_call segment
      if (item.tool_calls) {
        try {
          const calls =
            typeof item.tool_calls === 'string'
              ? JSON.parse(item.tool_calls)
              : item.tool_calls;
          if (Array.isArray(calls)) {
            calls.forEach((tc: any) => {
              const fn = tc.function || tc;
              segs.push({
                type: 'tool_call',
                toolName: fn.name || '',
                toolCallId: tc.id || tc.tool_call_id || '',
                arguments:
                  typeof fn.arguments === 'object'
                    ? JSON.stringify(fn.arguments)
                    : fn.arguments || '',
                status: 'completed',
                result: undefined,
              });
            });
          }
        } catch {
          /* ignore parse errors */
        }
      }

      // 如果有 thinking/推理内容
      if (item.thinking) {
        segs.push({ type: 'reasoning', content: item.thinking });
      }

      // 如果有文本内容
      if (item.content) {
        segs.push({ type: 'text', content: item.content });
      }

      const msg: Message = { role: 'assistant', segments: segs, timestamp: ts };
      result.push(msg);

      // 记住当前 assistant message，后续 tool 结果需要更新它
      const lastToolSegIdx = segs.findLastIndex((s) => s.type === 'tool_call');
      pending = lastToolSegIdx >= 0 ? { msg, segIdx: lastToolSegIdx } : null;
      continue;
    }

    // tool → 将结果回填到上一个 pending 的 tool_call segment 上
    if (item.role === 'tool' && pending) {
      const seg = pending.msg.segments[pending.segIdx];
      if (
        seg &&
        seg.type === 'tool_call' &&
        seg.toolCallId === item.tool_call_id
      ) {
        seg.status = 'completed';
        seg.result =
          typeof item.result === 'object'
            ? JSON.stringify(item.result)
            : item.result || item.content || '';
      }
      // 继续向前寻找同一个 pending.msg 里的下一个 tool_call
      const nextIdx = pending.msg.segments.findLastIndex(
        (s, i) => s.type === 'tool_call' && i > pending!.segIdx,
      );
      if (nextIdx >= 0) {
        pending.segIdx = nextIdx;
      } else {
        // 重新从头找第一个未完成的
        const firstPending = pending.msg.segments.findIndex(
          (s) => s.type === 'tool_call' && s.status === 'calling',
        );
        pending.segIdx = firstPending >= 0 ? firstPending : pending.segIdx;
      }
    }
  }

  return result;
}

/** 格式化 Token 数量 */
export function formatTokenCount(total: number): string {
  if (total <= 0) return '';
  if (total < 1000) return `${total} tokens`;
  if (total < 1_000_000) return `${(total / 1000).toFixed(1)}k tokens`;
  return `${(total / 1_000_000).toFixed(2)}m tokens`;
}
