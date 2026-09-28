import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate, useParams } from 'react-router-dom';
import { addGroupMembers, canManageMember, createGroupInvite, getConversation, leaveGroup, listGroupInvites, listGroupMembers, removeGroupMember, revokeGroupInvite, updateGroup, updateGroupMemberRole, type GroupConversation, type GroupInvite, type GroupMember, type GroupRole } from '../api/conversations';
import { listFriends, type Friend } from '../api/friends';
import { useAuth } from '../context/AuthContext';

export default function GroupSettingsPage() {
  const { conversationId } = useParams();
  const navigate = useNavigate();
  const { user } = useAuth();
  const [group, setGroup] = useState<GroupConversation | null>(null);
  const [members, setMembers] = useState<GroupMember[]>([]);
  const [friends, setFriends] = useState<Friend[]>([]);
  const [invites, setInvites] = useState<GroupInvite[]>([]);
  const [selected, setSelected] = useState<string[]>([]);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [expiresInMinutes, setExpiresInMinutes] = useState(1440);
  const [maxUses, setMaxUses] = useState('');
  const [newInviteURL, setNewInviteURL] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!conversationId) return;
    setLoading(true);
    try {
      const [conversation, memberResult, friendResult] = await Promise.all([getConversation(conversationId), listGroupMembers(conversationId), listFriends(100, 0)]);
      if (!conversation.data || conversation.data.kind !== 'group') throw new Error('Group unavailable');
      setGroup(conversation.data);
      setName(conversation.data.name);
      setDescription(conversation.data.description ?? '');
      setMembers(memberResult.data?.members ?? []);
      setFriends(friendResult.data?.friends ?? []);
      if (conversation.data.capabilities.manage_invites) {
        const inviteResult = await listGroupInvites(conversationId);
        setInvites(inviteResult.data?.invites ?? []);
      }
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Group unavailable');
    } finally {
      setLoading(false);
    }
  }, [conversationId]);

  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => { void load(); }, [load]);

  if (!conversationId) return <Navigate to="/conversations" replace />;
  if (loading) return <div className="app-shell flex min-h-screen items-center justify-center">Loading settings...</div>;
  if (!group) return <div className="app-shell flex min-h-screen items-center justify-center text-red-300">{error}</div>;

  const save = async (event: FormEvent) => { event.preventDefault(); const result = await updateGroup(conversationId, { name: name.trim(), description: description.trim() }); if (result.data) setGroup(result.data); else setError(result.error || 'Could not update group'); };
  const add = async () => { const result = await addGroupMembers(conversationId, selected); if (result.success) { setSelected([]); await load(); } else setError(result.error || 'Could not add members'); };
  const remove = async (member: GroupMember) => { if (!window.confirm(`Remove ${member.username}?`)) return; const result = await removeGroupMember(conversationId, member.user_id); if (result.success) await load(); else setError(result.error || 'Could not remove member'); };
  const role = async (member: GroupMember, next: GroupRole) => { const result = await updateGroupMemberRole(conversationId, member.user_id, next); if (result.success) await load(); else setError(result.error || 'Could not update role'); };
  const leave = async () => { if (!window.confirm('Leave this group?')) return; const result = await leaveGroup(conversationId); if (result.success) navigate('/conversations', { replace: true }); else setError(result.error || 'Could not leave group'); };
  const createInvite = async (event: FormEvent) => { event.preventDefault(); const parsedMaxUses = maxUses ? Number(maxUses) : undefined; const result = await createGroupInvite(conversationId, expiresInMinutes, parsedMaxUses); if (result.data) { setNewInviteURL(`${window.location.origin}/invites/${result.data.token}`); setInvites((current) => [result.data!.invite, ...current]); } else setError(result.error || 'Could not create invite'); };
  const revokeInvite = async (inviteId: string) => { const result = await revokeGroupInvite(conversationId, inviteId); if (result.success) await load(); else setError(result.error || 'Could not revoke invite'); };
  const copyInvite = async () => { try { await navigator.clipboard.writeText(newInviteURL); } catch { setError('Could not copy the invite link'); } };
  const available = friends.filter((friend) => !members.some((member) => member.user_id === friend.FriendID));

  return <div className="app-shell min-h-screen"><header className="app-header"><div className="mx-auto max-w-4xl px-4 py-4"><Link to={`/conversations/${conversationId}`} className="text-blue-400">Back to group</Link></div></header><main className="mx-auto max-w-4xl space-y-6 px-4 py-8"><h1 className="text-3xl font-bold text-white">Group settings</h1>{error && <p className="rounded-xl bg-red-950/40 p-3 text-red-300">{error}</p>}
    {group.capabilities.edit_group && <form onSubmit={save} className="app-card space-y-4 p-5"><h2 className="text-lg font-semibold">Group info</h2><input aria-label="Group name" value={name} onChange={(event) => setName(event.target.value)} maxLength={100} className="app-input w-full px-4 py-3" /><textarea aria-label="Description" value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} className="app-input min-h-24 w-full px-4 py-3" /><button className="primary-button px-4 py-2">Save changes</button></form>}
    {group.capabilities.manage_invites && <section className="app-card space-y-4 p-5"><h2 className="text-lg font-semibold">Invite links</h2><form onSubmit={createInvite} className="flex flex-wrap items-end gap-3"><label className="text-sm">Expires<select aria-label="Invite expiry" value={expiresInMinutes} onChange={(event) => setExpiresInMinutes(Number(event.target.value))} className="app-input mt-1 block px-3 py-2"><option value={60}>1 hour</option><option value={1440}>1 day</option><option value={10080}>7 days</option><option value={43200}>30 days</option></select></label><label className="text-sm">Maximum uses<input aria-label="Maximum uses" type="number" min="1" max="10000" value={maxUses} onChange={(event) => setMaxUses(event.target.value)} placeholder="Unlimited" className="app-input mt-1 block w-32 px-3 py-2" /></label><button className="primary-button px-4 py-2">Create invite</button></form>{newInviteURL && <div className="flex gap-2"><input aria-label="New invite link" readOnly value={newInviteURL} className="app-input min-w-0 flex-1 px-3 py-2" /><button type="button" onClick={() => void copyInvite()} className="soft-button px-4 py-2">Copy</button></div>}<ul className="divide-y divide-[#25364d]">{invites.map((invite) => <li key={invite.id} className="flex flex-wrap items-center justify-between gap-3 py-3"><p className="text-sm text-slate-300">{invite.use_count}{invite.max_uses ? ` / ${invite.max_uses}` : ''} uses · expires {new Date(invite.expires_at).toLocaleString()}{invite.revoked_at && ' · revoked'}</p>{!invite.revoked_at && <button type="button" onClick={() => void revokeInvite(invite.id)} className="soft-button px-3 py-2 text-sm">Revoke</button>}</li>)}</ul></section>}
    {group.capabilities.add_members && <section className="app-card p-5"><h2 className="text-lg font-semibold">Add members</h2><div className="mt-3 flex flex-wrap gap-2">{available.map((friend) => <label key={friend.FriendID} className="soft-button flex items-center gap-2 px-3 py-2 text-sm"><input type="checkbox" checked={selected.includes(friend.FriendID)} onChange={() => setSelected((ids) => ids.includes(friend.FriendID) ? ids.filter((id) => id !== friend.FriendID) : [...ids, friend.FriendID])} />{friend.FriendName}</label>)}</div><button onClick={() => void add()} disabled={!selected.length} className="primary-button mt-4 px-4 py-2 disabled:opacity-50">Add selected</button></section>}
    <section className="app-card overflow-hidden"><h2 className="p-5 text-lg font-semibold">Members ({members.length})</h2><ul className="divide-y divide-[#25364d]">{members.map((member) => <li key={member.user_id} className="flex flex-wrap items-center justify-between gap-3 p-4"><div><p className="font-medium text-white">{member.username}{member.user_id === user?.id && ' (you)'}</p><p className="text-sm capitalize text-slate-400">{member.role}</p></div><div className="flex gap-2">{group.capabilities.manage_roles && member.user_id !== user?.id && <select aria-label={`Role for ${member.username}`} value={member.role} onChange={(event) => void role(member, event.target.value as GroupRole)} className="app-input px-2 py-1"><option value="member">Member</option><option value="admin">Admin</option><option value="owner">Owner</option></select>}{canManageMember(group, user?.id ?? '', member) && <button onClick={() => void remove(member)} className="soft-button px-3 py-2 text-sm">Remove</button>}</div></li>)}</ul></section>
    <button onClick={() => void leave()} className="soft-button px-4 py-2 text-red-300">Leave group</button></main></div>;
}
