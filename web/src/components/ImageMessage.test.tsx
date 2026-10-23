import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import ImageMessage from './ImageMessage';
import { imageURL, type Attachment } from '../api/attachments';
vi.mock('../api/attachments', () => ({ imageURL: vi.fn() }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });
it('bounds full-image URL renewal and allows manual retry', async () => {
  vi.mocked(imageURL).mockImplementation(async (_message, _id, variant, _download, renew) => `${variant}-${renew ? 'new' : 'old'}`);
  HTMLDialogElement.prototype.showModal = vi.fn();
  render(<ImageMessage messageId="message" attachment={{ id: 'image', filename: 'photo', width: 10, height: 10 } as Attachment} />);
  await waitFor(() => expect(imageURL).toHaveBeenCalled());
  fireEvent.click(screen.getByRole('button', { name: 'View image: photo' }));
  const fullImage = await waitFor(() => {
    const element = document.querySelector('dialog img');
    expect(element).not.toBeNull();
    return element!;
  });
  fireEvent.error(fullImage);
  await waitFor(() => expect(fullImage).toHaveAttribute('src', 'full-new'));
  fireEvent.error(fullImage);
  await waitFor(() => expect(screen.getByText('Retry full image')).toBeInTheDocument());
  expect(vi.mocked(imageURL).mock.calls.filter((call) => call[2] === 'full')).toHaveLength(2);
  fireEvent.click(screen.getByText('Retry full image'));
  await waitFor(() => expect(document.querySelector('dialog img')).not.toBeNull());
});
