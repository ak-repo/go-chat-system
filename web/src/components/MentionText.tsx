/* eslint-disable react-refresh/only-export-components */
import type { MessageMention } from '../api/messages';

export function mentionSegments(content: string, mentions: MessageMention[]) {
  const characters = Array.from(content);
  const result: { text: string; mention: boolean }[] = [];
  let cursor = 0;
  for (const mention of [...mentions].sort((a, b) => a.offset - b.offset)) {
    if (mention.offset < cursor || mention.length <= 0 || mention.offset + mention.length > characters.length) continue;
    if (mention.offset > cursor) result.push({ text: characters.slice(cursor, mention.offset).join(''), mention: false });
    result.push({ text: characters.slice(mention.offset, mention.offset + mention.length).join(''), mention: true });
    cursor = mention.offset + mention.length;
  }
  if (cursor < characters.length) result.push({ text: characters.slice(cursor).join(''), mention: false });
  return result;
}

export default function MentionText({ content, mentions = [] }: { content: string; mentions?: MessageMention[] }) {
  return <>{mentionSegments(content, mentions).map((segment, index) => segment.mention ? <mark key={index} className="rounded bg-accent-soft px-0.5 text-accent">{segment.text}</mark> : <span key={index}>{segment.text}</span>)}</>;
}
