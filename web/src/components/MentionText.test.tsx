import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import MentionText, { mentionSegments } from './MentionText';

describe('MentionText', () => {
  it('uses Unicode offsets and does not interpret message markup', () => {
    expect(mentionSegments('hi 😀 @sam', [{ kind: 'user', user_id: 'id', offset: 5, length: 4 }])).toEqual([{ text: 'hi 😀 ', mention: false }, { text: '@sam', mention: true }]);
    render(<MentionText content={'<b>@sam</b>'} mentions={[{ kind: 'user', user_id: 'id', offset: 3, length: 4 }]} />);
    expect(screen.getByText('@sam').tagName).toBe('MARK');
    expect(document.querySelector('b')).toBeNull();
  });
});
