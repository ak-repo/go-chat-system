import { useState, type FormEvent } from 'react';
import { useAuth } from '../context/AuthContext';
import { Link, useLocation, useNavigate } from 'react-router-dom';

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);

    const result = await login({ email, password });
    setLoading(false);

    if (result.success) {
      const from = (location.state as { from?: unknown } | null)?.from;
      navigate(typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/conversations', { replace: true });
    } else {
      setError(result.error || 'Login failed');
    }
  };

  return (
    <div className="app-shell flex min-h-screen items-center justify-center p-4">
      <div className="app-card w-full max-w-md p-7 sm:p-9">
        <div className="mb-8 text-center"><div className="avatar mx-auto mb-4 h-14 w-14 text-xl">C</div><p className="text-sm font-medium uppercase tracking-[0.2em] text-accent">Welcome back</p><h1 className="mt-2 text-2xl font-bold text-primary">Sign in</h1></div>

        {error && (
          <div className="mb-4 rounded-xl border border-danger bg-danger-soft p-3 text-danger">
            {error}
          </div>

        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-secondary">
              Email
            </label>
            <input
              type="email"
              aria-label="Email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="app-input mt-2 w-full px-3 py-3"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-secondary">
              Password
            </label>
            <input
              type="password"
              aria-label="Password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              className="app-input mt-2 w-full px-3 py-3"
            />
          </div>

          <button
            type="submit"
            disabled={loading}
            className="primary-button w-full px-4 py-3 font-semibold disabled:opacity-50"
          >
            {loading ? 'Logging in...' : 'Login'}
          </button>
        </form>

        <div className="mt-4 text-right"><Link to="/recover" className="text-sm text-accent hover-accent">Forgot password?</Link></div>

        <p className="mt-6 text-center text-sm text-secondary">
          Don't have an account?{' '}
          <Link to="/register" state={location.state} className="font-semibold text-accent hover-accent">
            Register
          </Link>
        </p>
        <p className="mt-3 text-center text-sm"><Link to="/verify" state={location.state} className="font-semibold text-accent hover-accent">Need to verify your email?</Link></p>
      </div>
    </div>
  );
}
