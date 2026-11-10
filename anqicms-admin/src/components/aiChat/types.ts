// ---------------------------------------------------------------------------
// 类型定义
// ---------------------------------------------------------------------------

export type SegmentType = 'reasoning' | 'text' | 'tool_call' | 'warning';

export interface Segment {
  type: SegmentType;
  /** 用于 reasoning / text / warning */
  content?: string;
  /** 用于 tool_call */
  toolName?: string;
  toolCallId?: string;
  arguments?: string;
  status?: 'calling' | 'completed';
  result?: string;
}

export interface Message {
  role: 'user' | 'assistant';
  segments: Segment[];
  timestamp: number;
  files?: { file_name: string; file_path: string }[];
}

export interface AiChatProps {
  visible: boolean;
  onClose: () => void;
  /** 前端 AI 编辑器：当前 iframe 页面 URL */
  iframeUrl?: string;
  /** 前端 AI 编辑器：管理员选中的 DOM 片段 */
  selectedDom?: string;
  /** 前端 AI 编辑器：AI 写模板成功后刷新 iframe */
  onIframeReload?: () => void;
}

export interface AiProviderConfig {
  name: string;
  api_key: string;
  base_url: string;
  model: string;
  enable_reasoning?: boolean;
  max_tokens?: number;
  timeout_seconds?: number;
  temperature?: number;
  max_retries?: number;
}

export interface TokenUsage {
  prompt: number;
  completion: number;
  total: number;
}
