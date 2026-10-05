import { BrowserRouter, Routes, Route, Navigate, useLocation, Outlet } from 'react-router-dom';
import { AuthProvider, useAuth } from './context/AuthContext';
import { SocketProvider } from './context/SocketContext';
import LoginPage from './pages/LoginPage';
import RegisterPage from './pages/RegisterPage';
import FriendsPage from './pages/FriendsPage';
import ChatPage from './pages/ChatPage';
import ProfilePage from './pages/ProfilePage';
import RecoveryPage from './pages/RecoveryPage';
import VerificationPage from './pages/VerificationPage';
import ConversationsPage from './pages/ConversationsPage';
import PublicProfilePage from './pages/PublicProfilePage';
import CreateGroupPage from './pages/CreateGroupPage';
import GroupSettingsPage from './pages/GroupSettingsPage';
import DirectChatResolver from './pages/DirectChatResolver';
import NotificationsPage from './pages/NotificationsPage';
import InviteAcceptancePage from './pages/InviteAcceptancePage';
import AppShell from './components/AppShell';
import { ConversationProvider } from './context/ConversationContext';
import { ThemeProvider } from './context/ThemeContext';
import { DialogProvider } from './context/DialogContext';

// Protected route wrapper
function ProtectedRoute() {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace state={{ from: `${location.pathname}${location.search}${location.hash}` }} />;
  }

  return <ConversationProvider><AppShell><Outlet /></AppShell></ConversationProvider>;
}

// Public route wrapper (redirect if already logged in)
function PublicRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();
  const location = useLocation();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    );
  }

  if (isAuthenticated) {
    const from = (location.state as { from?: unknown } | null)?.from;
    return <Navigate to={typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/conversations'} replace />;
  }

  return <>{children}</>;
}

function RootRedirect() {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-lg">Loading...</div>
      </div>
    );
  }

  return <Navigate to={isAuthenticated ? '/conversations' : '/login'} replace />;
}

function AppRoutes() {
  return (
    <Routes>
      <Route
        path="/login"
        element={
          <PublicRoute>
            <LoginPage />
          </PublicRoute>
        }
      />
      <Route
        path="/register"
        element={
          <PublicRoute>
            <RegisterPage />
          </PublicRoute>
        }
      />
      <Route path="/recover" element={<PublicRoute><RecoveryPage /></PublicRoute>} />
      <Route path="/verify" element={<VerificationPage />} />
      <Route element={<ProtectedRoute />}>
        <Route path="/friends" element={<FriendsPage />} />
        <Route path="/chat/:userId" element={<DirectChatResolver />} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="/conversations" element={<ConversationsPage />} />
        <Route path="/conversations/new-group" element={<CreateGroupPage />} />
        <Route path="/conversations/:conversationId" element={<ChatPage />} />
        <Route path="/conversations/:conversationId/settings" element={<GroupSettingsPage />} />
        <Route path="/users/:userId" element={<PublicProfilePage />} />
        <Route path="/notifications" element={<NotificationsPage />} />
        <Route path="/invites/:token" element={<InviteAcceptancePage />} />
      </Route>
      <Route path="/" element={<RootRedirect />} />
      <Route path="*" element={<RootRedirect />} />
    </Routes>
  );
}

function App() {
  return (
    <ThemeProvider><DialogProvider><BrowserRouter>
      <AuthProvider>
        <SocketProvider>
          <AppRoutes />
        </SocketProvider>
      </AuthProvider>
    </BrowserRouter></DialogProvider></ThemeProvider>
  );
}

export default App;
