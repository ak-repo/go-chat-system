import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { getPublicUser, type PublicUser } from '../api/users';

export default function PublicProfilePage() {
  const { userId = '' } = useParams<{ userId: string }>();
  const [profile, setProfile] = useState<PublicUser | null>(null);
  const [error, setError] = useState('');
  useEffect(() => { let active = true; void getPublicUser(userId).then((r) => { if (!active) return; if (r.success && r.data) setProfile(r.data); else setError(r.error || 'Profile not found'); }).catch(() => { if (active) setError('Profile not found'); }); return () => { active = false; }; }, [userId]);
  return <div className="app-shell min-h-screen p-4 sm:p-8"><div className="app-card mx-auto max-w-lg p-6 sm:p-8"><Link to="/friends" className="text-sm font-semibold text-blue-400">← Back</Link><h1 className="my-6 text-2xl font-bold text-white">User profile</h1>{error && <p className="text-red-300">{error}</p>}{profile && <div className="flex items-center gap-4"><div className="avatar h-14 w-14">{profile.username.charAt(0).toUpperCase()}</div><div><p className="font-semibold text-white">{profile.username}</p><p className="text-sm text-slate-400">Public profile</p></div></div>}</div></div>;
}
