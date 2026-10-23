import { afterEach, expect, it, vi } from 'vitest';
import axios from 'axios';
import apiClient from './client';
import { imageURL, mediaCapabilities, uploadImage } from './attachments';
afterEach(() => vi.restoreAllMocks());
it('uploads through bare axios without authentication headers', async () => {
  const post = vi.spyOn(axios, 'post').mockResolvedValue({ status: 204 });
  const authenticated = vi.spyOn(apiClient, 'post');
  const controller = new AbortController();
  await uploadImage({ attachment: {} as never, upload: { url: 'https://storage.test', fields: { key: 'staging/image' } }, expires_at: '' }, new File(['image'], 'a.png'), controller.signal, vi.fn());
  expect(authenticated).not.toHaveBeenCalled();
  const [, body, options] = post.mock.calls[0];
  expect((body as FormData).get('key')).toBe('staging/image');
  expect(options).not.toHaveProperty('headers.Authorization');
});
it('requests capabilities and renews cached URLs explicitly', async () => {
  const get = vi.spyOn(apiClient, 'get').mockResolvedValueOnce({ data: { success: true, data: { images_enabled: false } } });
  expect((await mediaCapabilities(new AbortController().signal)).images_enabled).toBe(false);
  get.mockResolvedValue({ data: { success: true, data: { url: 'signed', expires_at: new Date(Date.now() + 300000).toISOString() } } });
  await imageURL('cache-test', 'image', 'full');
  await imageURL('cache-test', 'image', 'full');
  expect(get).toHaveBeenCalledTimes(2);
  await imageURL('cache-test', 'image', 'full', false, true);
  expect(get).toHaveBeenCalledTimes(3);
});
