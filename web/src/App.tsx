import React, { useState, useEffect } from 'react'
import { Navigation, TabType } from './components/Navigation'
import { AccountsTab } from './components/AccountsTab'
import { CatalogTab } from './components/CatalogTab'
import { SubscriptionsTab } from './components/SubscriptionsTab'
import { InvoicesTab } from './components/InvoicesTab'
import { PaymentsTab } from './components/PaymentsTab'
import { E2EFlowTab } from './components/E2EFlowTab'
import { ApiInspector } from './components/ApiInspector'
import { Account, Invoice, Subscription, ApiLogEntry } from './types/api'
import { subscribeLogs } from './services/apiClient'

export const App: React.FC = () => {
  const [activeTab, setActiveTab] = useState<TabType>('e2e')
  const [inspectorOpen, setInspectorOpen] = useState(false)
  const [logs, setLogs] = useState<ApiLogEntry[]>([])

  const [accounts, setAccounts] = useState<Account[]>([])
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([])
  const [invoices, setInvoices] = useState<Invoice[]>([])

  useEffect(() => {
    const unsubscribe = subscribeLogs((nextLogs) => {
      setLogs(nextLogs)
    })
    return unsubscribe
  }, [])

  const handleAccountCreated = (acc: Account) => {
    if (!acc || typeof acc !== 'object' || !acc.id) return
    setAccounts((prev) => {
      const arr = Array.isArray(prev) ? prev : []
      const idx = arr.findIndex((a) => a.id === acc.id)
      if (idx >= 0) {
        const next = [...arr]
        next[idx] = acc
        return next
      }
      return [acc, ...arr]
    })
  }

  const handleAccountUpdated = (acc: Account) => {
    if (!acc || typeof acc !== 'object' || !acc.id) return
    setAccounts((prev) => (Array.isArray(prev) ? prev.map((a) => (a.id === acc.id ? acc : a)) : []))
  }

  const handleSubscriptionCreated = (sub: Subscription) => {
    if (!sub || typeof sub !== 'object' || !sub.id) return
    setSubscriptions((prev) => [sub, ...(Array.isArray(prev) ? prev : [])])
  }

  const handleInvoiceCreated = (inv: Invoice) => {
    if (!inv || typeof inv !== 'object' || !inv.id) return
    setInvoices((prev) => {
      const arr = Array.isArray(prev) ? prev : []
      const idx = arr.findIndex((i) => i.id === inv.id)
      if (idx >= 0) {
        const next = [...arr]
        next[idx] = inv
        return next
      }
      return [inv, ...arr]
    })
  }

  const handleInvoiceUpdated = (inv: Invoice) => {
    if (!inv || typeof inv !== 'object' || !inv.id) return
    setInvoices((prev) => (Array.isArray(prev) ? prev.map((i) => (i.id === inv.id ? inv : i)) : []))
  }

  return (
    <div className="min-h-screen bg-[#141210] text-stone-200 flex flex-col font-sans">
      <Navigation
        activeTab={activeTab}
        onSelectTab={setActiveTab}
        inspectorOpen={inspectorOpen}
        onToggleInspector={() => setInspectorOpen((prev) => !prev)}
        logCount={logs.length}
      />

      <main className="flex-1 max-w-6xl w-full mx-auto px-4 py-6">
        {activeTab === 'e2e' && (
          <E2EFlowTab
            onAccountCreated={handleAccountCreated}
            onSubscriptionCreated={handleSubscriptionCreated}
            onInvoiceCreated={handleInvoiceCreated}
            onInvoiceUpdated={handleInvoiceUpdated}
          />
        )}

        {activeTab === 'accounts' && (
          <AccountsTab
            accounts={accounts}
            onAccountCreated={handleAccountCreated}
            onAccountUpdated={handleAccountUpdated}
          />
        )}

        {activeTab === 'catalog' && <CatalogTab />}

        {activeTab === 'subscriptions' && (
          <SubscriptionsTab
            accounts={accounts}
            subscriptions={subscriptions}
            onSubscriptionCreated={handleSubscriptionCreated}
          />
        )}

        {activeTab === 'invoices' && (
          <InvoicesTab
            accounts={accounts}
            invoices={invoices}
            onInvoiceCreated={handleInvoiceCreated}
            onInvoiceUpdated={handleInvoiceUpdated}
          />
        )}

        {activeTab === 'payments' && (
          <PaymentsTab invoices={invoices} onInvoiceUpdated={handleInvoiceUpdated} />
        )}
      </main>

      <footer className="border-t border-stone-800 py-3 text-center text-xs text-stone-500 font-mono">
        gebeta_billing workbench
      </footer>

      <ApiInspector
        logs={logs}
        isOpen={inspectorOpen}
        onClose={() => setInspectorOpen(false)}
      />
    </div>
  )
}

export default App
