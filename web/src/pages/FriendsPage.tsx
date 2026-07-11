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
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [friends, setFriends] = useState<Friend[]>([]);
  const [requests, setRequests] = useState<FriendRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

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
        setFriends(friendsRes.data.friends);
      }
      if (requestsRes.success && requestsRes.data) {
        setRequests(requestsRes.data.requests);
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
        alert('Friend request sent!');
      }
    } catch {
      alert('Failed to send request');
    }
  };

  const handleAcceptRequest = async (request: FriendRequest) => {
    try {
      await acceptFriendRequest(request.id, request.sender_id);
      loadData();
    } catch {
      alert('Failed to accept request');
    }
  };

  const handleRejectRequest = async (request: FriendRequest) => {
    try {
      await rejectFriendRequest(request.id, request.sender_id);
      loadData();
    } catch {
      alert('Failed to reject request');
    }
  };

  const handleLogout = () => {
    logout();
    navigate('/login', { replace: true });
  };

  const handleChat = (friendId: string) => {
    navigate(`/chat/${friendId}`);
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white shadow">
        <div className="max-w-4xl mx-auto px-4 py-4 flex justify-between items-center">
          <h1 className="text-xl font-bold">Chat App</h1>
          <div className="flex items-center gap-4">
            <span className="text-gray-600">{user?.username}</span>
            <button
              onClick={handleLogout}
              className="text-sm text-red-600 hover:underline"
            >
              Logout
            </button>
          </div>
        </div>
      </header>

      <div className="max-w-4xl mx-auto px-4 py-6">
        {/* Tabs */}
        <div className="flex gap-2 mb-6">
          <button
            onClick={() => setActiveTab('friends')}
            className={`px-4 py-2 rounded-md ${
              activeTab === 'friends'
                ? 'bg-purple-600 text-white'
                : 'bg-gray-200 text-gray-700'
            }`}
          >
            Friends ({friends.length})
          </button>
          <button
            onClick={() => setActiveTab('requests')}
            className={`px-4 py-2 rounded-md ${
              activeTab === 'requests'
                ? 'bg-purple-600 text-white'
                : 'bg-gray-200 text-gray-700'
            }`}
          >
            Requests ({requests.filter((r) => r.status === 'pending').length})
          </button>
          <button
            onClick={() => setActiveTab('search')}
            className={`px-4 py-2 rounded-md ${
              activeTab === 'search'
                ? 'bg-purple-600 text-white'
                : 'bg-gray-200 text-gray-700'
            }`}
          >
            Find Friends
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-100 text-red-700 rounded">
            {error}
          </div>
        )}

        {/* Friends List */}
        {activeTab === 'friends' && (
          <div className="bg-white rounded-lg shadow">
            {friends.length === 0 ? (
              <div className="p-8 text-center text-gray-500">
                No friends yet. Find some friends!
              </div>
            ) : (
              <ul className="divide-y">
                {friends.map((friend) => (
                  <li
                    key={friend.id}
                    className="p-4 flex items-center justify-between"
                  >
                    <div>
                      <div className="font-medium">{friend.friend?.username}</div>
                      <div className="text-sm text-gray-500">
                        {friend.friend?.email}
                      </div>
                    </div>
                    <button
                      onClick={() => handleChat(friend.friend_id)}
                      className="px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700"
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
          <div className="bg-white rounded-lg shadow">
            {requests.length === 0 ? (
              <div className="p-8 text-center text-gray-500">
                No friend requests
              </div>
            ) : (
              <ul className="divide-y">
                {requests
                  .filter((r) => r.status === 'pending')
                  .map((request) => (
                    <li
                      key={request.id}
                      className="p-4 flex items-center justify-between"
                    >
                      <div>
                        <div className="font-medium">
                          {request.sender?.username || request.sender_id}
                        </div>
                        <div className="text-sm text-gray-500">
                          Wants to be your friend
                        </div>
                      </div>
                      <div className="flex gap-2">
                        <button
                          onClick={() => handleAcceptRequest(request)}
                          className="px-3 py-1 bg-green-600 text-white rounded hover:bg-green-700"
                        >
                          Accept
                        </button>
                        <button
                          onClick={() => handleRejectRequest(request)}
                          className="px-3 py-1 bg-red-600 text-white rounded hover:bg-red-700"
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
            <div className="flex gap-2">
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                placeholder="Search by username or email..."
                className="flex-1 px-4 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-purple-500"
              />
              <button
                onClick={handleSearch}
                disabled={searching}
                className="px-6 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700 disabled:opacity-50"
              >
                {searching ? 'Searching...' : 'Search'}
              </button>
            </div>

            <div className="bg-white rounded-lg shadow">
              {searchResults.length === 0 ? (
                <div className="p-8 text-center text-gray-500">
                  {searchQuery ? 'No users found' : 'Enter a search term'}
                </div>
              ) : (
                <ul className="divide-y">
                  {searchResults.map((result) => (
                    <li
                      key={result.id}
                      className="p-4 flex items-center justify-between"
                    >
                      <div>
                        <div className="font-medium">{result.username}</div>
                        <div className="text-sm text-gray-500">{result.email}</div>
                      </div>
                      <button
                        onClick={() => handleSendRequest(result.id)}
                        className="px-4 py-2 bg-purple-600 text-white rounded-md hover:bg-purple-700"
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
