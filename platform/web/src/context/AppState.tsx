import React, { createContext, useContext, useState } from 'react';
import type { ReactNode } from 'react';

export interface PendingWebhook {
  internal_tx_id: string;
  provider_reference: string;
  amount: number;
}

export interface SavedAccount {
  accountId: number;
  externalId: string;
  currency: string;
}

interface AppState {
  accessToken: string | null;
  accountId: number | null;
  subscriptionId: number | null;
  invoiceId: number | null;
  pendingWebhooks: PendingWebhook[];
  savedAccounts: SavedAccount[];
  setAccessToken: (token: string | null) => void;
  setAccountId: (id: number | null) => void;
  setSubscriptionId: (id: number | null) => void;
  setInvoiceId: (id: number | null) => void;
  addPendingWebhook: (webhook: PendingWebhook) => void;
  removePendingWebhook: (internalTxId: string) => void;
  addSavedAccount: (account: SavedAccount) => void;
  logout: () => void;
}

const AppStateContext = createContext<AppState | undefined>(undefined);

export const AppStateProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [accessToken, setAccessTokenState] = useState<string | null>(() =>
    sessionStorage.getItem('accessToken')
  );
  const [accountId, setAccountIdState] = useState<number | null>(() => {
    const saved = localStorage.getItem('activeAccountId');
    return saved ? parseInt(saved, 10) : null;
  });
  
  const [savedAccounts, setSavedAccountsState] = useState<SavedAccount[]>(() => {
    const saved = localStorage.getItem('savedAccounts');
    return saved ? JSON.parse(saved) : [];
  });

  const [subscriptionId, setSubscriptionId] = useState<number | null>(null);
  const [invoiceId, setInvoiceId] = useState<number | null>(null);
  const [pendingWebhooks, setPendingWebhooks] = useState<PendingWebhook[]>([]);

  const setAccountId = (id: number | null) => {
    setAccountIdState(id);
    if (id !== null) {
      localStorage.setItem('activeAccountId', id.toString());
    } else {
      localStorage.removeItem('activeAccountId');
    }
  };

  const setAccessToken = (token: string | null) => {
    setAccessTokenState(token);
    if (token) {
      sessionStorage.setItem('accessToken', token);
    } else {
      sessionStorage.removeItem('accessToken');
    }
  };

  const addSavedAccount = (account: SavedAccount) => {
    setSavedAccountsState(prev => {
      const exists = prev.some(a => a.accountId === account.accountId);
      if (exists) return prev;
      const updated = [...prev, account];
      localStorage.setItem('savedAccounts', JSON.stringify(updated));
      return updated;
    });
  };

  const logout = () => {
    setAccessToken(null);
    setAccountId(null);
    setSubscriptionId(null);
    setInvoiceId(null);
    setPendingWebhooks([]);
  };

  const addPendingWebhook = (webhook: PendingWebhook) => {
    setPendingWebhooks((prev) => [...prev, webhook]);
  };

  const removePendingWebhook = (internalTxId: string) => {
    setPendingWebhooks((prev) => prev.filter(w => w.internal_tx_id !== internalTxId));
  };

  return (
    <AppStateContext.Provider
      value={{
        accessToken,
        setAccessToken,
        accountId,
        setAccountId,
        subscriptionId,
        setSubscriptionId,
        invoiceId,
        setInvoiceId,
        pendingWebhooks,
        addPendingWebhook,
        removePendingWebhook,
        savedAccounts,
        addSavedAccount,
        logout,
      }}
    >
      {children}
    </AppStateContext.Provider>
  );
};

export const useAppState = () => {
  const context = useContext(AppStateContext);
  if (context === undefined) {
    throw new Error('useAppState must be used within an AppStateProvider');
  }
  return context;
};
