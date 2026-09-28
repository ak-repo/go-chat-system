import apiClient, { toApiResponse } from './client';
import type { ApiResponse } from './client';

interface ConversationBase { id: string; created_at: string; modified_at: string; archived: boolean; pinned: boolean; muted: boolean; muted_until?: string }
export interface DirectConversation extends ConversationBase { kind: 'direct'; user_one_id: string; user_two_id: string }
export type GroupRole = 'owner' | 'admin' | 'member';
export interface GroupCapabilities { edit_group: boolean; add_members: boolean; remove_members: boolean; manage_roles: boolean; manage_invites: boolean }
export interface GroupConversation extends ConversationBase { kind: 'group'; name: string; description?: string; creator_id: string; member_count: number; current_role: GroupRole; capabilities: GroupCapabilities }
export type Conversation = DirectConversation | GroupConversation;
export interface GroupMember { user_id: string; username: string; role: GroupRole; joined_at: string; added_by?: string; modified_at: string }
export interface GroupInvite { id: string; conversation_id: string; creator_id: string; expires_at: string; max_uses?: number; use_count: number; revoked_at?: string; created_at: string }
export interface ConversationPage { conversations: Conversation[]; limit: number; offset: number }
export async function createOrGetConversation(userId: string): Promise<ApiResponse<Conversation>> { const r = await apiClient.post<ApiResponse<Conversation>>('/conversations', { user_id: userId }); return toApiResponse(r.data); }
export async function listConversations(limit = 50, offset = 0, archived = false): Promise<ApiResponse<ConversationPage>> { const r = await apiClient.get<ApiResponse<ConversationPage>>('/conversations', { params: { limit, offset, archived } }); return toApiResponse(r.data); }
export async function getConversation(id: string): Promise<ApiResponse<Conversation>> { const r = await apiClient.get<ApiResponse<Conversation>>(`/conversations/${encodeURIComponent(id)}`); return toApiResponse(r.data); }
export async function updateConversationPreferences(id: string, patch: { archived?: boolean; pinned?: boolean; mute_minutes?: number }): Promise<ApiResponse<Conversation>> { const r = await apiClient.patch<ApiResponse<Conversation>>(`/conversations/${encodeURIComponent(id)}/preferences`, patch); return toApiResponse(r.data); }
export async function hideConversation(id: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.delete<ApiResponse<{ status: string }>>(`/conversations/${encodeURIComponent(id)}`); return toApiResponse(r.data); }
export async function createGroup(data: { name: string; description?: string; member_ids: string[] }): Promise<ApiResponse<GroupConversation>> { const r = await apiClient.post<ApiResponse<GroupConversation>>('/groups', data); return toApiResponse(r.data); }
export async function updateGroup(id: string, data: { name?: string; description?: string }): Promise<ApiResponse<GroupConversation>> { const r = await apiClient.patch<ApiResponse<GroupConversation>>(`/groups/${encodeURIComponent(id)}`, data); return toApiResponse(r.data); }
export async function listGroupMembers(id: string): Promise<ApiResponse<{ members: GroupMember[] }>> { const r = await apiClient.get<ApiResponse<{ members: GroupMember[] }>>(`/groups/${encodeURIComponent(id)}/members`); return toApiResponse(r.data); }
export async function addGroupMembers(id: string, memberIds: string[]): Promise<ApiResponse<{ members: GroupMember[] }>> { const r = await apiClient.post<ApiResponse<{ members: GroupMember[] }>>(`/groups/${encodeURIComponent(id)}/members`, { member_ids: memberIds }); return toApiResponse(r.data); }
export async function removeGroupMember(id: string, userId: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.delete<ApiResponse<{ status: string }>>(`/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}`); return toApiResponse(r.data); }
export async function leaveGroup(id: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.post<ApiResponse<{ status: string }>>(`/groups/${encodeURIComponent(id)}/leave`); return toApiResponse(r.data); }
export async function updateGroupMemberRole(id: string, userId: string, role: GroupRole): Promise<ApiResponse<{ status: string; role: GroupRole }>> { const r = await apiClient.patch<ApiResponse<{ status: string; role: GroupRole }>>(`/groups/${encodeURIComponent(id)}/members/${encodeURIComponent(userId)}/role`, { role }); return toApiResponse(r.data); }
export async function createGroupInvite(id: string, expiresInMinutes: number, maxUses?: number): Promise<ApiResponse<{ invite: GroupInvite; token: string }>> { const r = await apiClient.post<ApiResponse<{ invite: GroupInvite; token: string }>>(`/groups/${encodeURIComponent(id)}/invites`, { expires_in_minutes: expiresInMinutes, max_uses: maxUses }); return toApiResponse(r.data); }
export async function listGroupInvites(id: string): Promise<ApiResponse<{ invites: GroupInvite[] }>> { const r = await apiClient.get<ApiResponse<{ invites: GroupInvite[] }>>(`/groups/${encodeURIComponent(id)}/invites`); return toApiResponse(r.data); }
export async function revokeGroupInvite(id: string, inviteId: string): Promise<ApiResponse<{ status: string }>> { const r = await apiClient.delete<ApiResponse<{ status: string }>>(`/groups/${encodeURIComponent(id)}/invites/${encodeURIComponent(inviteId)}`); return toApiResponse(r.data); }
export async function acceptGroupInvite(token: string): Promise<ApiResponse<{ conversation_id: string }>> { const r = await apiClient.post<ApiResponse<{ conversation_id: string }>>(`/group-invites/${encodeURIComponent(token)}/accept`); return toApiResponse(r.data); }

export function conversationInitials(name: string): string { return name.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || '?'; }
export function canManageMember(conversation: GroupConversation, actorId: string, member: GroupMember): boolean { return conversation.capabilities.remove_members && member.user_id !== actorId && member.role !== 'owner' && (conversation.current_role === 'owner' || member.role === 'member'); }
