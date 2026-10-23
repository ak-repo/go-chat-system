import { act, cleanup, renderHook, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useImageUpload } from './useImageUpload';
import * as api from '../api/attachments';
vi.mock('../api/attachments', () => ({ initiateImage: vi.fn(), uploadImage: vi.fn(), completeImage: vi.fn(), imageStatus: vi.fn(), cancelImage: vi.fn() }));
const file = new File(['image'], 'photo.png', { type: 'image/png' });
beforeEach(() => {
  vi.resetAllMocks();
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:preview'), revokeObjectURL: vi.fn() }));
  vi.mocked(api.cancelImage).mockResolvedValue();
  vi.mocked(api.initiateImage).mockResolvedValue({ attachment: { id: 'image' }, upload: { url: '', fields: {} }, expires_at: '' } as Awaited<ReturnType<typeof api.initiateImage>>);
  vi.mocked(api.uploadImage).mockResolvedValue();
  vi.mocked(api.completeImage).mockResolvedValue({ id: 'image', state: 'ready' } as api.Attachment);
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
it('clears and cancels a ready draft when switching conversations', async () => {
  const { result, rerender } = renderHook(({ id }) => useImageUpload(id), { initialProps: { id: 'one' } });
  await act(() => result.current.select(file));
  expect(result.current.draft?.phase).toBe('ready');
  rerender({ id: 'two' });
  expect(result.current.draft).toBeNull();
  expect(api.cancelImage).toHaveBeenCalledWith('image');
  expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview');
});
it('does not restore a stale draft when initiation completes after navigation', async () => {
  let resolve!: (value: Awaited<ReturnType<typeof api.initiateImage>>) => void;
  vi.mocked(api.initiateImage).mockImplementation(() => new Promise((done) => { resolve = done; }));
  const { result, rerender } = renderHook(({ id }) => useImageUpload(id), { initialProps: { id: 'one' } });
  act(() => { void result.current.select(file); });
  rerender({ id: 'two' });
  await act(async () => resolve({ attachment: { id: 'late' } } as Awaited<ReturnType<typeof api.initiateImage>>));
  await waitFor(() => expect(api.cancelImage).toHaveBeenCalledWith('late'));
  expect(result.current.draft).toBeNull();
  expect(api.uploadImage).not.toHaveBeenCalled();
});
it('consumes a sent image without cancelling and retries failed uploads', async () => {
  vi.mocked(api.uploadImage).mockRejectedValueOnce(new Error('offline'));
  const { result } = renderHook(() => useImageUpload('one'));
  await act(() => result.current.select(file));
  expect(result.current.draft?.phase).toBe('failed');
  act(() => result.current.retry());
  await waitFor(() => expect(result.current.draft?.phase).toBe('ready'));
  vi.mocked(api.cancelImage).mockClear();
  act(() => result.current.consume());
  expect(result.current.draft).toBeNull();
  expect(api.cancelImage).not.toHaveBeenCalled();
});
