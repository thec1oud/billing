import React from 'react'

export type TabType = 'e2e' | 'accounts' | 'catalog' | 'subscriptions' | 'invoices' | 'payments'

interface NavigationProps {
  activeTab: TabType
  onSelectTab: (tab: TabType) => void
  inspectorOpen: boolean
  onToggleInspector: () => void
  logCount: number
}

export const Navigation: React.FC<NavigationProps> = ({
  activeTab,
  onSelectTab,
  inspectorOpen,
  onToggleInspector,
  logCount,
}) => {
  const tabs: { id: TabType; label: string }[] = [
    { id: 'e2e', label: 'E2E Flow' },
    { id: 'accounts', label: 'Accounts' },
    { id: 'catalog', label: 'Catalog' },
    { id: 'subscriptions', label: 'Subscriptions' },
    { id: 'invoices', label: 'Invoices' },
    { id: 'payments', label: 'Payments & Webhooks' },
  ]

  return (
    <header className="border-b border-stone-800 bg-[#171412]">
      <div className="max-w-6xl mx-auto px-4">
        <div className="flex items-center justify-between h-14">
          <div className="flex items-center space-x-6">
            <span className="font-mono text-sm font-semibold text-stone-200 tracking-tight">
              gebeta_billing
            </span>

            <nav className="flex space-x-1">
              {tabs.map((tab) => {
                const isActive = activeTab === tab.id
                return (
                  <button
                    key={tab.id}
                    onClick={() => onSelectTab(tab.id)}
                    className={`px-3 py-1.5 text-xs font-medium rounded transition-colors cursor-pointer ${
                      isActive
                        ? 'bg-[#452818] text-[#fed7aa] border border-[#78350f]'
                        : 'text-stone-400 hover:text-stone-200 hover:bg-stone-800/60'
                    }`}
                  >
                    {tab.label}
                  </button>
                )
              })}
            </nav>
          </div>

          <div>
            <button
              onClick={onToggleInspector}
              className={`px-3 py-1.5 text-xs font-mono font-medium rounded border transition-colors cursor-pointer ${
                inspectorOpen
                  ? 'bg-[#3b2314] text-[#fed7aa] border-[#78350f]'
                  : 'bg-stone-900 text-stone-400 border-stone-800 hover:text-stone-200 hover:bg-stone-850'
              }`}
            >
              API Inspector {logCount > 0 ? `(${logCount})` : ''}
            </button>
          </div>
        </div>
      </div>
    </header>
  )
}
