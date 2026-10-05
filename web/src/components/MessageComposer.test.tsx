import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import MessageComposer from './MessageComposer';
afterEach(cleanup);
function mount(overrides = {}) {
  const onSubmit = vi.fn((event) => event.preventDefault());
  const onChange = vi.fn();
  render(<MessageComposer value="Hello" disabled={false} onChange={onChange} onSubmit={onSubmit} onBlur={() => undefined} {...overrides} />);
  return { input: screen.getByLabelText('Message'), onSubmit, onChange };
}
describe('MessageComposer', () => {
  it('sends on Enter and preserves Shift+Enter for multiline input', () => {
    const { input, onSubmit } = mount();
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true });
    expect(onSubmit).not.toHaveBeenCalled();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSubmit).toHaveBeenCalledOnce();
  });
  it('lets mention selection consume Enter before sending', () => {
    const { input, onSubmit } = mount({ mentionCount: 2, activeMention: 1, onKeyDown: (event: React.KeyboardEvent) => event.preventDefault() });
    expect(input).toHaveAttribute('aria-activedescendant', 'mention-option-1');
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSubmit).not.toHaveBeenCalled();
  });
  it('does not send during IME composition or send empty messages', () => {
    const { input, onSubmit } = mount();
    fireEvent.keyDown(input, { key: 'Enter', isComposing: true });
    expect(onSubmit).not.toHaveBeenCalled();
    cleanup();
    const empty = mount({ value: '  ' });
    fireEvent.keyDown(empty.input, { key: 'Enter' });
    expect(empty.onSubmit).not.toHaveBeenCalled();
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
  });
  it('retains the draft while disconnected and disables sending', () => {
    const { input, onSubmit } = mount({ value: 'Draft\nsecond line', disabled: true });
    expect(input).toHaveValue('Draft\nsecond line');
    expect(input).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Send message' })).toBeDisabled();
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
