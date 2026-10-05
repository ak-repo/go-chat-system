import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DialogProvider, useDialogs } from './DialogContext';
afterEach(cleanup);
function Trigger({ done }: { done: (result: boolean | string | null) => void }) {
  const { confirm, askText } = useDialogs();
  return <><button onClick={async () => done(await confirm('Hide chat?'))}>Hide</button><button onClick={async () => done(await askText('Edit message', 'Original'))}>Edit</button></>;
}
describe('dialogs', () => {
  it('traps focus, cancels on Escape, and restores focus', async () => {
    const done = vi.fn();
    render(<DialogProvider><Trigger done={done} /></DialogProvider>);
    const trigger = screen.getByRole('button', { name: 'Hide' });
    trigger.focus(); fireEvent.click(trigger);
    expect(screen.getByRole('dialog')).toHaveAttribute('aria-modal', 'true');
    const close = screen.getByRole('button', { name: 'Close dialog' });
    expect(close).toHaveFocus();
    fireEvent.keyDown(close, { key: 'Tab', shiftKey: true });
    expect(screen.getByRole('button', { name: 'Confirm' })).toHaveFocus();
    fireEvent.keyDown(document, { key: 'Escape' });
    await waitFor(() => expect(done).toHaveBeenCalledWith(false));
    expect(trigger).toHaveFocus();
  });
  it('saves edited text through the accessible dialog', async () => {
    const done = vi.fn();
    render(<DialogProvider><Trigger done={done} /></DialogProvider>);
    fireEvent.click(screen.getByRole('button', { name: 'Edit' }));
    const input = screen.getByRole('textbox', { name: 'Edit message' });
    expect(input).toHaveFocus();
    fireEvent.change(input, { target: { value: 'Changed\nmessage' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(done).toHaveBeenCalledWith('Changed\nmessage'));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
