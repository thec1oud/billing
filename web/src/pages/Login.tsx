import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { accountService } from '../api/services';
import { useAppState } from '../context/AppState';
import type { CreateAccountInput } from '../api/types';

export const Login: React.FC = () => {
  const { savedAccounts, setAccountId, addSavedAccount } = useAppState();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [externalId, setExternalId] = useState('');
  const [currency, setCurrency] = useState('ETB');

  const handleCreateAccount = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!externalId.trim()) {
      setError("External ID is required");
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const createInput: CreateAccountInput = {
        external_id: externalId.trim(),
        currency,
        timezone: "UTC",
        net_terms: 0,
      };
      
      const newAccount = await accountService.create(createInput);
      await accountService.activate(newAccount.id);

      const savedAcc = {
        accountId: newAccount.id,
        externalId: newAccount.external_id,
        currency: newAccount.currency,
      };

      addSavedAccount(savedAcc);
      setAccountId(savedAcc.accountId);

      navigate('/plans');
    } catch (err: any) {
      setError(err.message || "Failed to create account");
    } finally {
      setLoading(false);
    }
  };

  const handleLogin = (accountId: number) => {
    setAccountId(accountId);
    navigate('/plans');
  };

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Login / Create Account</h2>
          <p>Select a saved account or create a new one.</p>
        </div>
        
        {error && <div className="alert-error">{error}</div>}

        {savedAccounts.length > 0 && (
          <div style={{ marginBottom: '2rem' }}>
            <h3>Saved Accounts</h3>
            <div className="webhook-list" style={{ marginTop: '1rem' }}>
              {savedAccounts.map(acc => (
                <div key={acc.accountId} className="webhook-item">
                  <div className="webhook-info">
                    <strong>ID:</strong> {acc.accountId} <br/>
                    <strong>Ext ID:</strong> {acc.externalId} <br/>
                    <strong>Currency:</strong> {acc.currency}
                  </div>
                  <button 
                    className="btn btn-secondary"
                    onClick={() => handleLogin(acc.accountId)}
                  >
                    Login
                  </button>
                </div>
              ))}
            </div>
            <hr style={{ margin: '2rem 0', borderColor: 'var(--border-color)' }} />
          </div>
        )}

        <h3>Create New Account</h3>
        <form onSubmit={handleCreateAccount} style={{ display: 'flex', flexDirection: 'column', gap: '1rem', marginTop: '1rem' }}>
          <div>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.9rem', color: 'var(--text-muted)' }}>External ID</label>
            <input 
              type="text" 
              value={externalId}
              onChange={e => setExternalId(e.target.value)}
              placeholder="e.g., user_123"
              style={{ width: '100%', padding: '0.75rem', borderRadius: '4px', border: '1px solid var(--border-color)', backgroundColor: 'var(--bg-main)', color: 'white' }}
            />
          </div>
          <div>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.9rem', color: 'var(--text-muted)' }}>Currency</label>
            <select 
              value={currency}
              onChange={e => setCurrency(e.target.value)}
              style={{ width: '100%', padding: '0.75rem', borderRadius: '4px', border: '1px solid var(--border-color)', backgroundColor: 'var(--bg-main)', color: 'white' }}
            >
              <option value="ETB">ETB</option>
              <option value="USD">USD</option>
            </select>
          </div>
          <button 
            type="submit"
            className="btn btn-primary"
            disabled={loading}
          >
            {loading ? 'Processing...' : 'Create & Login'}
          </button>
        </form>
      </div>
    </div>
  );
};
