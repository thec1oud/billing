import React, { useState } from 'react'
import { Account, Invoice, LineItem, CreateDraftInvoiceInput } from '../types/api'
import { billingApi } from '../services/apiClient'

interface InvoicesTabProps {
  accounts: Account[]
  invoices: Invoice[]
  onInvoiceCreated: (inv: Invoice) => void
  onInvoiceUpdated: (inv: Invoice) => void
}

export const InvoicesTab: React.FC<InvoicesTabProps> = ({
  accounts,
  invoices,
  onInvoiceCreated,
  onInvoiceUpdated,
}) => {
  const [accountId, setAccountId] = useState<number>(accounts[0]?.id || 1)
  const [currency, setCurrency] = useState<string>('ETB')
  const [lines, setLines] = useState<LineItem[]>([
    { item_id: 1, description: 'API Requests (1,000 units)', quantity: 2, unit_price_minor: 5000 },
  ])
  const [loading, setLoading] = useState(false)
  const [finalizingId, setFinalizingId] = useState<number | null>(null)
  const [fetchingId, setFetchingId] = useState<number | null>(null)
  const [searchId, setSearchId] = useState<string>('')
  const [error, setError] = useState<string | null>(null)
  const [copiedId, setCopiedId] = useState<number | null>(null)

  const handleAddLine = () => {
    setLines([
      ...lines,
      { item_id: 1, description: 'Additional Usage Package', quantity: 1, unit_price_minor: 2500 },
    ])
  }

  const handleRemoveLine = (index: number) => {
    setLines(lines.filter((_, i) => i !== index))
  }

  const handleLineChange = (index: number, field: keyof LineItem, value: any) => {
    const next = [...lines]
    next[index] = { ...next[index], [field]: value }
    setLines(next)
  }

  const handleCreateDraft = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      const input: CreateDraftInvoiceInput = {
        account_id: Number(accountId),
        currency,
        line_items: lines.map((l) => ({
          item_id: Number(l.item_id),
          description: l.description,
          quantity: Number(l.quantity),
          unit_price_minor: Number(l.unit_price_minor),
        })),
      }
      const res = await billingApi.createDraftInvoice(input)
      onInvoiceCreated(res.data)
    } catch (err: any) {
      setError(err.message || 'Failed to create draft invoice')
    } finally {
      setLoading(false)
    }
  }

  const handleFinalize = async (id: number) => {
    setFinalizingId(id)
    setError(null)
    try {
      const res = await billingApi.finalizeInvoice(id)
      onInvoiceUpdated(res.data)
    } catch (err: any) {
      setError(err.message || 'Failed to finalize invoice')
    } finally {
      setFinalizingId(null)
    }
  }

  const handleRefreshStatus = async (id: number) => {
    setFetchingId(id)
    setError(null)
    try {
      const res = await billingApi.getInvoice(id)
      onInvoiceUpdated(res.data)
    } catch (err: any) {
      setError(err.message || 'Failed to get invoice')
    } finally {
      setFetchingId(null)
    }
  }

  const handleSearch = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!searchId) return
    setError(null)
    try {
      const res = await billingApi.getInvoice(Number(searchId))
      onInvoiceUpdated(res.data)
    } catch (err: any) {
      setError(err.message || `Invoice #${searchId} not found`)
    }
  }

  const handleCopy = (id: number) => {
    navigator.clipboard.writeText(id.toString())
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 1500)
  }

  const getStatusStyle = (status: string) => {
    switch (status) {
      case 'DRAFT':
        return 'bg-stone-800 text-amber-300 border-amber-800/60'
      case 'ISSUED':
        return 'bg-stone-800 text-stone-200 border-stone-600'
      case 'PAYMENT_PENDING':
        return 'bg-stone-800 text-amber-200 border-amber-800'
      case 'PAID':
        return 'bg-stone-800 text-emerald-300 border-emerald-800/60'
      default:
        return 'bg-stone-800 text-stone-400 border-stone-700'
    }
  }

  const safeAccounts = Array.isArray(accounts) ? accounts : []
  const safeInvoices = Array.isArray(invoices) ? invoices : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">Invoices</h2>
      </div>

      {error && (
        <div className="p-3 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Draft Builder */}
        <div className="lg:col-span-5 bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            New Draft Invoice
          </h3>

          <form onSubmit={handleCreateDraft} className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-stone-400 mb-1">Account ID</label>
                {safeAccounts.length > 0 ? (
                  <select
                    value={accountId}
                    onChange={(e) => setAccountId(Number(e.target.value))}
                    className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                  >
                    {safeAccounts.map((acc) => (
                      <option key={acc.id} value={acc.id}>
                        #{acc.id} - {acc.name}
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="number"
                    value={accountId}
                    onChange={(e) => setAccountId(Number(e.target.value))}
                    required
                    className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                  />
                )}
              </div>

              <div>
                <label className="block text-xs text-stone-400 mb-1">Currency</label>
                <select
                  value={currency}
                  onChange={(e) => setCurrency(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                >
                  <option value="ETB">ETB</option>
                  <option value="USD">USD</option>
                  <option value="EUR">EUR</option>
                </select>
              </div>
            </div>

            {/* Line Items */}
            <div>
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs text-stone-400">Line Items</span>
                <button
                  type="button"
                  onClick={handleAddLine}
                  className="text-xs text-[#fed7aa] hover:underline cursor-pointer"
                >
                  + Add Line
                </button>
              </div>

              <div className="space-y-2 max-h-52 overflow-y-auto pr-1">
                {lines.map((line, idx) => (
                  <div key={idx} className="p-2.5 bg-[#12100e] border border-stone-800 rounded space-y-2">
                    <div className="flex items-center justify-between gap-2">
                      <input
                        type="text"
                        value={line.description}
                        onChange={(e) => handleLineChange(idx, 'description', e.target.value)}
                        placeholder="Description"
                        required
                        className="flex-1 px-2 py-1 bg-[#191614] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                      />
                      {lines.length > 1 && (
                        <button
                          type="button"
                          onClick={() => handleRemoveLine(idx)}
                          className="text-stone-500 hover:text-red-400 text-xs px-1"
                        >
                          Remove
                        </button>
                      )}
                    </div>

                    <div className="grid grid-cols-3 gap-2">
                      <div>
                        <label className="block text-[10px] text-stone-500">Item ID</label>
                        <input
                          type="number"
                          value={line.item_id}
                          onChange={(e) => handleLineChange(idx, 'item_id', Number(e.target.value))}
                          className="w-full px-1.5 py-1 bg-[#191614] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                        />
                      </div>
                      <div>
                        <label className="block text-[10px] text-stone-500">Qty</label>
                        <input
                          type="number"
                          value={line.quantity}
                          min={1}
                          onChange={(e) => handleLineChange(idx, 'quantity', Number(e.target.value))}
                          className="w-full px-1.5 py-1 bg-[#191614] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                        />
                      </div>
                      <div>
                        <label className="block text-[10px] text-stone-500">Price (cents)</label>
                        <input
                          type="number"
                          value={line.unit_price_minor}
                          onChange={(e) => handleLineChange(idx, 'unit_price_minor', Number(e.target.value))}
                          className="w-full px-1.5 py-1 bg-[#191614] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                        />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full mt-2 py-2 px-3 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
            >
              {loading ? 'Creating...' : 'Create Draft'}
            </button>
          </form>
        </div>

        {/* Invoices List */}
        <div className="lg:col-span-7 space-y-4">
          <div className="bg-[#191614] border border-stone-800 rounded p-3">
            <form onSubmit={handleSearch} className="flex gap-2">
              <input
                type="number"
                placeholder="Lookup invoice by ID"
                value={searchId}
                onChange={(e) => setSearchId(e.target.value)}
                className="flex-1 px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
              />
              <button
                type="submit"
                className="px-3 py-1.5 bg-[#422515] hover:bg-[#57311c] text-[#fed7aa] border border-[#78350f] rounded text-xs transition-colors cursor-pointer"
              >
                Inspect
              </button>
            </form>
          </div>

          <div className="bg-[#191614] border border-stone-800 rounded p-4">
            <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
              Invoices ({safeInvoices.length})
            </h3>

            {safeInvoices.length === 0 ? (
              <div className="py-8 text-center text-stone-500 text-xs">
                No invoices created yet.
              </div>
            ) : (
              <div className="space-y-2 max-h-[440px] overflow-y-auto pr-1">
                {safeInvoices.map((inv) => {
                  const isDraft = inv.status === 'DRAFT'
                  const total = (inv.total_minor || 0) / 100
                  return (
                    <div
                      key={inv.id}
                      className="p-3 rounded bg-[#12100e] border border-stone-800 text-xs font-mono"
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center space-x-2">
                          <button
                            onClick={() => handleCopy(inv.id)}
                            className="font-semibold text-stone-200 hover:text-amber-300 cursor-pointer"
                            title="Copy ID"
                          >
                            Invoice #{inv.id} {copiedId === inv.id ? '(copied)' : ''}
                          </button>
                          <span
                            className={`px-1.5 py-0.5 rounded text-[11px] border ${getStatusStyle(
                              inv.status
                            )}`}
                          >
                            {inv.status}
                          </span>
                        </div>

                        <div className="flex items-center space-x-2">
                          <button
                            onClick={() => handleRefreshStatus(inv.id)}
                            disabled={fetchingId === inv.id}
                            className="px-2 py-1 bg-stone-900 hover:bg-stone-800 text-stone-300 border border-stone-700 rounded text-[11px] cursor-pointer"
                          >
                            {fetchingId === inv.id ? '...' : 'Refresh'}
                          </button>

                          {isDraft && (
                            <button
                              onClick={() => handleFinalize(inv.id)}
                              disabled={finalizingId === inv.id}
                              className="px-2 py-1 bg-[#5c351c] hover:bg-[#734323] text-[#fed7aa] border border-[#854d0e] rounded text-[11px] cursor-pointer"
                            >
                              {finalizingId === inv.id ? 'Finalizing...' : 'Finalize'}
                            </button>
                          )}
                        </div>
                      </div>

                      <div className="mt-2 text-stone-400 text-[11px] flex gap-4">
                        <span>Account: #{inv.account_id}</span>
                        <span>
                          Total: <strong className="text-stone-200">{total.toFixed(2)} {inv.currency}</strong>
                        </span>
                        <span>Time: {new Date(inv.created_at).toLocaleTimeString()}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
