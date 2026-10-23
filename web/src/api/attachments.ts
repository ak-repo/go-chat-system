import axios from 'axios';
import apiClient, { toApiResponse, type ApiResponse } from './client';

export interface Attachment {
  id: string; filename: string; mime_type: string; size_bytes: number;
  width: number; height: number;
  state: 'pending' | 'queued' | 'processing' | 'ready' | 'failed' | 'deleting' | 'deleted';
  error?: string;
}
interface UploadTicket { attachment: Attachment; upload: { url: string; fields: Record<string, string> }; expires_at: string }
function unwrap<T>(response: ApiResponse<T>): T {
  const result = toApiResponse(response);
  if (!result.success || !result.data) throw new Error(result.error || 'Image request failed');
  return result.data;
}
export async function initiateImage(conversationId: string, file: File, signal: AbortSignal): Promise<UploadTicket> {
  return unwrap((await apiClient.post(`/conversations/${encodeURIComponent(conversationId)}/attachments`, { filename: file.name, mime_type: file.type, size_bytes: file.size }, { signal })).data);
}
// Deliberately separate from apiClient: storage must never receive bearer tokens.
export async function uploadImage(ticket: UploadTicket, file: File, signal: AbortSignal, progress: (percent: number) => void): Promise<void> {
  const form = new FormData();
  Object.entries(ticket.upload.fields).forEach(([key, value]) => form.append(key, value));
  form.append('file', file);
  await axios.post(ticket.upload.url, form, { signal, onUploadProgress: (event) => progress(Math.min(100, Math.round(event.loaded / (event.total || file.size) * 100))) });
}
export async function completeImage(id: string, signal: AbortSignal): Promise<Attachment> {
  return unwrap((await apiClient.post(`/attachments/${encodeURIComponent(id)}/complete`, {}, { signal })).data);
}
export async function imageStatus(id: string, signal: AbortSignal): Promise<Attachment> {
  return unwrap((await apiClient.get(`/attachments/${encodeURIComponent(id)}`, { signal })).data);
}
export async function cancelImage(id: string): Promise<void> { await apiClient.delete(`/attachments/${encodeURIComponent(id)}`); }
const urls = new Map<string, { url: string; expires_at: string }>();
export async function imageURL(messageId: string, id: string, variant: 'full' | 'thumbnail', download = false, renew = false): Promise<string> {
  const key = `${messageId}/${id}/${variant}/${download}`;
  const cached = urls.get(key);
  if (!renew && cached && Date.parse(cached.expires_at) > Date.now() + 30_000) return cached.url;
  const result = unwrap<{ url: string; expires_at: string }>((await apiClient.get(`/messages/${encodeURIComponent(messageId)}/attachments/${encodeURIComponent(id)}/url`, { params: { variant, download } })).data);
  if (urls.size > 500) urls.clear();
  urls.set(key, result);
  return result.url;
}

export interface MediaCapabilities {
  images_enabled: boolean; mime_types: string[]; max_bytes: number; max_pixels: number; max_attachments: number;
}
export async function mediaCapabilities(signal: AbortSignal): Promise<MediaCapabilities> {
  return unwrap((await apiClient.get('/media/capabilities', { signal })).data);
}
