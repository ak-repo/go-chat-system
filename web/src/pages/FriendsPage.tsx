import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../context/AuthContext';
import { useNavigate } from 'react-router-dom';
import {
  listFriends,
  getFriendRequests,
  createFriendRequest,
  acceptFriendRequest,
  rejectFriendRequest,
  searchUsers,
  type Friend,
  type FriendRequest,
  type User,
} from '../api';

export default function FriendsPage() {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [friends, setFriends] = useState<Friend[]>([]);
  const [requests, setRequests] = useState<FriendRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [feedback, setFeedback] = useState('');

  // User search state
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<User[]>([]);
  const [searching, setSearching] = useState(false);

  // Tab state
  const [activeTab, setActiveTab] = useState<'friends' | 'requests' | 'search'>('friends');

  const loadData = useCallback(async () => {
    setLoading(true);
    try {
      const [friendsRes, requestsRes] = await Promise.all([
        listFriends(50, 0),
        getFriendRequests(),
      ]);

      if (friendsRes.success && friendsRes.data) {
        setFriends(friendsRes.data.friends ?? []);
      }
      if (requestsRes.success && requestsRes.data) {
        setRequests(requestsRes.data.requests ?? []);
      }
      if (friendsRes.error) {
        setError('Failed to load data');
      }
    } catch {
      setError('Failed to load data');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!user) {
      navigate('/login');
      return;
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadData();
  }, [user, navigate, loadData]);

  const handleSearch = async () => {
    if (!searchQuery.trim()) return;
    setSearching(true);
    try {
      const response = await searchUsers(searchQuery, 20);
      if (response.success && response.data) {
        setSearchResults(response.data.users);
      }
    } catch {
      setError('Search failed');
    } finally {
      setSearching(false);
    }
  };

  const handleSendRequest = async (userId: string) => {
    try {
      const response = await createFriendRequest(userId);
      if (response.success) {
        setSearchResults((prev) => prev.filter((u) => u.id !== userId));
        setFeedback('Friend request sent.');
      }
    } catch {
      setError('Failed to send request');
    }
  };

  const handleAcceptRequest = async (request: FriendRequest) => {
    try {
      await acceptFriendRequest(request.ID);
      loadData();
    } catch {
      setError('Failed to accept request');
    }
  };

  const handleRejectRequest = async (request: FriendRequest) => {
    try {
      await rejectFriendRequest(request.ID);
      loadData();
    } catch {
      setError('Failed to reject request');
    }
  };

  const handleChat = (friendId: string) => {
    navigate(`/chat/${friendId}`);
  };

  if (loading) {
    return (
      <div className="app-shell flex items-center justify-center">
        <div className="text-lg text-secondary">Loading...</div>
      </div>
    );
  }

  return (
    <div className="app-shell">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 py-8">
        <div className="mb-8">
          <p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">Your network</p>
          <h2 className="mt-2 text-3xl font-bold text-primary">Stay connected</h2>
          <p className="mt-2 text-secondary">Manage your friends and start a conversation.</p>
        </div>
        {/* Tabs */}
        <div className="flex flex-wrap gap-2 mb-6 rounded-2xl border border-theme bg-raised p-2">
          <button
            onClick={() => setActiveTab('friends')}
            className={`px-4 py-2 rounded-xl text-sm font-semibold transition-colors ${
              activeTab === 'friends'
                ? 'primary-button '
                : 'text-secondary hover-surface hover-accent'
            }`}
          >
            Friends ({friends.length})
          </button>
          <button
            onClick={() => setActiveTab('requests')}
            className={`px-4 py-2 rounded-xl text-sm font-semibold transition-colors ${
              activeTab === 'requests'
                ? 'primary-button '
                : 'text-secondary hover-surface hover-accent'
            }`}
          >
            Requests ({requests.filter((r) => r.Status === 'pending').length})
          </button>
          <button
            onClick={() => setActiveTab('search')}
            className={`px-4 py-2 rounded-xl text-sm font-semibold transition-colors ${
              activeTab === 'search'
                ? 'primary-button '
                : 'text-secondary hover-surface hover-accent'
            }`}
          >
            Find Friends
          </button>
        </div>

        {feedback && <p role="status" className="mx-auto max-w-4xl rounded-xl bg-accent-soft p-3 text-sm text-accent">{feedback}</p>}
        {error && (
          <div className="mb-4 rounded-xl border border-danger bg-danger-soft p-3 text-danger">
            {error}
          </div>
        )}

        {/* Friends List */}
        {activeTab === 'friends' && (
          <div className="app-card overflow-hidden">
            {friends.length === 0 ? (
              <div className="p-12 text-center text-secondary">
                <div className="mx-auto mb-4 avatar h-14 w-14 text-2xl">+</div>
                <p className="font-semibold text-primary">Your friend list is empty</p>
                <p className="mt-1 text-sm">Find someone to start chatting.</p>
              </div>
            ) : (
              <ul className="divide-y divide-theme">
                {friends.map((friend) => (
                  <li
                    key={friend.FriendID}
                    className="flex items-center justify-between gap-4 p-5 transition-colors hover-surface"
                  >
                    <div>
                      <div className="flex items-center gap-3"><div className="avatar h-11 w-11">{friend.FriendName.charAt(0).toUpperCase()}</div><div><div className="font-semibold text-primary">{friend.FriendName}</div>
                      <button onClick={() => navigate(`/users/${friend.FriendID}`)} className="text-sm text-accent underline">View profile</button></div></div>
                    </div>
                    <button
                      onClick={() => handleChat(friend.FriendID)}
                      className="primary-button px-4 py-2 text-sm font-semibold"
                    >
                      Chat
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        {/* Friend Requests */}
        {activeTab === 'requests' && (
          <div className="app-card overflow-hidden">
            {requests.length === 0 ? (
              <div className="p-12 text-center text-secondary">
                No friend requests
              </div>
            ) : (
              <ul className="divide-y divide-theme">
                {requests
                  .filter((r) => r.Status === 'pending')
                  .map((request) => (
                    <li
                      key={request.ID}
                      className="flex items-center justify-between gap-4 p-5 hover-surface"
                    >
                      <div>
                        <div className="font-semibold text-primary">
                          {request.FriendName}
                        </div>
                        <div className="text-sm text-secondary">
                          {request.FriendEmail}
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleAcceptRequest(request)}
                          className="rounded-xl primary-button px-3 py-2 text-sm font-semibold text-primary hover-surface"
                        >
                          Accept
                        </button>
                        <button
                          onClick={() => handleRejectRequest(request)}
                          className="rounded-xl border border-danger px-3 py-2 text-sm font-semibold text-danger hover-surface"
                        >
                          Reject
                        </button>
                      </div>
                    </li>
                  ))}
              </ul>
            )}
          </div>
        )}

        {/* Search Users */}
        {activeTab === 'search' && (
          <div className="space-y-4">
            <div className="flex gap-2 rounded-2xl border border-theme bg-raised p-2">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                placeholder="Search by username or email..."
                className="app-input min-w-0 flex-1 px-4 py-3"
              />
              <button
                onClick={handleSearch}
                disabled={searching}
                className="primary-button px-5 py-3 text-sm font-semibold disabled:opacity-50"
              >
                {searching ? 'Searching...' : 'Search'}
              </button>
            </div>

            <div className="app-card overflow-hidden">
              {searchResults.length === 0 ? (
                <div className="p-12 text-center text-secondary">
                  {searchQuery ? 'No users found' : 'Enter a search term'}
                </div>
              ) : (
                <ul className="divide-y divide-theme">
                  {searchResults.map((result) => (
                    <li
                      key={result.id}
                      className="flex items-center justify-between gap-4 p-5 hover-surface"
                    >
                      <div>
                        <div className="font-semibold text-primary">{result.username}</div>
                        <button onClick={() => navigate(`/users/${result.id}`)} className="text-sm text-accent underline">View profile</button>
                      </div>
                      <button
                        onClick={() => handleSendRequest(result.id)}
                        className="primary-button px-4 py-2 text-sm font-semibold"
                      >
                        Add Friend
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
