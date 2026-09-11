import { useState, type FormEvent } from 'react';
import { useAuth } from '../context/AuthContext';
import { Link, useNavigate } from 'react-router-dom';

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
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
      navigate('/friends', { replace: true });
    } else {
      setError(result.error || 'Login failed');
    }
  };

  return (
    <div className="app-shell flex min-h-screen items-center justify-center p-4">
      <div className="app-card w-full max-w-md p-7 sm:p-9">
        <div className="mb-8 text-center"><div className="avatar mx-auto mb-4 h-14 w-14 text-xl">C</div><p className="text-sm font-medium uppercase tracking-[0.2em] text-blue-400">Welcome back</p><h1 className="mt-2 text-2xl font-bold text-white">Sign in</h1></div>

        {error && (
          <div className="mb-4 rounded-xl border border-red-900/60 bg-red-950/40 p-3 text-red-300">
            {error}
          </div>

        )}

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-slate-300">
              Email
            </label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              className="app-input mt-2 w-full px-3 py-3"
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300">
              Password
            </label>
            <input
              type="password"
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

        <div className="mt-4 text-right"><Link to="/recover" className="text-sm text-blue-400 hover:text-blue-300">Forgot password?</Link></div>

        <p className="mt-6 text-center text-sm text-slate-400">
          Don't have an account?{' '}
          <Link to="/register" className="font-semibold text-blue-400 hover:text-blue-300">
            Register
          </Link>
        </p>
      </div>
    </div>
  );
}
