export type VideoInstallFrame = {
  event: string;
  data: any;
};

export function parseSSEFrames(buffer: string): {
  events: VideoInstallFrame[];
  rest: string;
} {
  const blocks = buffer.split(/\r?\n\r?\n/);
  const rest = blocks.pop() || '';
  const events: VideoInstallFrame[] = [];
  for (const block of blocks) {
    let event = 'message';
    const data: string[] = [];
    for (const line of block.split(/\r?\n/)) {
      if (line.startsWith('event:')) {
        event = line.slice(6).trim() || event;
      } else if (line.startsWith('data:')) {
        data.push(line.slice(5).replace(/^ /, ''));
      }
    }
    if (!data.length) continue;
    try {
      events.push({ event, data: JSON.parse(data.join('\n')) });
    } catch {
      continue;
    }
  }
  return { events, rest };
}
