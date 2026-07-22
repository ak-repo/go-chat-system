import { useState, useEffect, useCallback, useRef } from 'react';
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
import { useNotifications } from '../context/NotificationContext';
import type { FriendRequestData } from '../api/websocket';

export default function FriendsPage() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const { notifications, unreadCount, markAsRead, markAllAsRead, deleteNotification, onFriendRequest } = useNotifications();
  const [friends, setFriends] = useState<Friend[]>([]);
  const [requests, setRequests] = useState<FriendRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Notification panel state
  const [showNotifications, setShowNotifications] = useState(false);
  const notificationPanelRef = useRef<HTMLDivElement>(null);

  // User search state
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<User[]>([]);
  const [searching, setSearching] = useState(false);

  // Tab state
  const [activeTab, setActiveTab] = useState<'friends' | 'requests' | 'search'>('friends');

  // Close notification panel when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (notificationPanelRef.current && !notificationPanelRef.current.contains(event.target as Node)) {
        setShowNotifications(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

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

  // Listen for real-time friend request events and refresh data
  useEffect(() => {
    if (!user) return;

    const unsubscribe = onFriendRequest((data: FriendRequestData) => {
      console.log('New friend request received:', data);
      // Refresh friend requests when a new one arrives
      loadData();
      // Optionally auto-switch to requests tab if user is not currently on it
      setActiveTab('requests');
    });

    return () => {
      unsubscribe();
    };
  }, [user, loadData, onFriendRequest]);

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
      await acceptFriendRequest(request.ID, request.ReceiverID);
      loadData();
    } catch {
      alert('Failed to accept request');
    }
  };

  const handleRejectRequest = async (request: FriendRequest) => {
    try {
      await rejectFriendRequest(request.ID, request.ReceiverID);
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
            {/* Notification Bell */}
            <div className="relative" ref={notificationPanelRef}>
              <button
                onClick={() => setShowNotifications(!showNotifications)}
                className="relative p-2 text-gray-600 hover:text-gray-800 transition-colors"
                aria-label="Notifications"
              >
                <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
                {unreadCount > 0 && (
                  <span className="absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full h-5 w-5 flex items-center justify-center">
                    {unreadCount > 9 ? '9+' : unreadCount}
                  </span>
                )}
              </button>

              {/* Notification Panel */}
              {showNotifications && (
                <div className="absolute right-0 mt-2 w-80 bg-white rounded-lg shadow-lg border border-gray-200 z-50 max-h-96 overflow-y-auto">
                  <div className="p-3 border-b border-gray-200 flex justify-between items-center">
                    <h3 className="font-semibold text-gray-800">Notifications</h3>
                    {unreadCount > 0 && (
                      <button
                        onClick={markAllAsRead}
                        className="text-sm text-purple-600 hover:text-purple-800"
                      >
                        Mark all read
                      </button>
                    )}
                  </div>
                  {notifications.length === 0 ? (
                    <div className="p-4 text-center text-gray-500">
                      No notifications
                    </div>
                  ) : (
                    <ul className="divide-y divide-gray-100">
                      {notifications.slice(0, 10).map((notification) => (
                        <li
                          key={notification.id}
                          className={`p-3 hover:bg-gray-50 cursor-pointer ${
                            !notification.is_read ? 'bg-purple-50' : ''
                          }`}
                          onClick={() => !notification.is_read && markAsRead(notification.id)}
                        >
                          <div className="flex justify-between items-start">
                            <div className="flex-1">
                              <p className={`text-sm ${!notification.is_read ? 'font-semibold' : 'font-normal'}`}>
                                {notification.title}
                              </p>
                              {notification.body && (
                                <p className="text-xs text-gray-500 mt-1">{notification.body}</p>
                              )}
                              <p className="text-xs text-gray-400 mt-1">
                                {new Date(notification.created_at).toLocaleString()}
                              </p>
                            </div>
                            <button
                              onClick={(e) => {
                                e.stopPropagation();
                                deleteNotification(notification.id);
                              }}
                              className="text-gray-400 hover:text-gray-600 ml-2"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                              </svg>
                            </button>
                          </div>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
              )}
            </div>

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
            Requests ({requests.filter((r) => r.Status === 'pending').length})
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
                    key={friend.FriendID}
                    className="p-4 flex items-center justify-between"
                  >
                    <div>
                      <div className="font-medium">{friend.FriendName}</div>
                      <div className="text-sm text-gray-500">
                        {friend.FriendEmail}
                      </div>
                    </div>
                    <button
                      onClick={() => handleChat(friend.FriendID)}
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
                  .filter((r) => r.Status === 'pending')
                  .map((request) => (
                    <li
                      key={request.ID}
                      className="p-4 flex items-center justify-between"
                    >
                      <div>
                        <div className="font-medium">
                          {request.FriendName}
                        </div>
                        <div className="text-sm text-gray-500">
                          {request.FriendEmail}
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
