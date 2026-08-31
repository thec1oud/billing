import React, { useState } from 'react'
import { Account, CreateAccountInput } from '../types/api'
import { billingApi } from '../services/apiClient'

interface AccountsTabProps {
  accounts: Account[]
  onAccountCreated: (account: Account) => void
  onAccountUpdated: (account: Account) => void
}

export const AccountsTab: React.FC<AccountsTabProps> = ({
  accounts,
  onAccountCreated,
  onAccountUpdated,
}) => {
  const [name, setName] = useState('Lelisa Hailu')
  const [email, setEmail] = useState(`user_${Math.floor(Math.random() * 1000)}@gebeta.com`)
  const [currency, setCurrency] = useState('ETB')
  const [countryCode, setCountryCode] = useState('ET')
  const [loading, setLoading] = useState(false)
  const [activatingId, setActivatingId] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copiedId, setCopiedId] = useState<number | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      const input: CreateAccountInput = {
        name,
        email,
        currency,
        country_code: countryCode,
      }
      const res = await billingApi.createAccount(input)
      onAccountCreated(res.data)
      setEmail(`user_${Math.floor(Math.random() * 1000)}@gebeta.com`)
    } catch (err: any) {
      setError(err.message || 'Failed to create account')
    } finally {
      setLoading(false)
    }
  }

  const handleActivate = async (id: number) => {
    setActivatingId(id)
    setError(null)
    try {
      const res = await billingApi.activateAccount(id)
      onAccountUpdated(res.data)
    } catch (err: any) {
      setError(err.message || 'Failed to activate account')
    } finally {
      setActivatingId(null)
    }
  }

  const handleCopy = (id: number) => {
    navigator.clipboard.writeText(id.toString())
    setCopiedId(id)
    setTimeout(() => setCopiedId(null), 1500)
  }

  const safeAccounts = Array.isArray(accounts) ? accounts : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">Accounts</h2>
      </div>

      {error && (
        <div className="p-3 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Creation Form */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            New Account
          </h3>

          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-xs text-stone-400 mb-1">Name</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
              />
            </div>

            <div>
              <label className="block text-xs text-stone-400 mb-1">Email</label>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
              />
            </div>

            <div className="grid grid-cols-2 gap-2">
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

              <div>
                <label className="block text-xs text-stone-400 mb-1">Country</label>
                <input
                  type="text"
                  value={countryCode}
                  maxLength={2}
                  onChange={(e) => setCountryCode(e.target.value.toUpperCase())}
                  required
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 uppercase focus:outline-none focus:border-[#854d0e]"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full mt-2 py-2 px-3 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
            >
              {loading ? 'Creating...' : 'Create Account'}
            </button>
          </form>
        </div>

        {/* Account Table */}
        <div className="lg:col-span-2 bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            Accounts List ({safeAccounts.length})
          </h3>

          {safeAccounts.length === 0 ? (
            <div className="py-8 text-center text-stone-500 text-xs">
              No accounts registered yet.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-stone-800 text-stone-400 font-mono">
                    <th className="pb-2">ID</th>
                    <th className="pb-2">Name</th>
                    <th className="pb-2">Email</th>
                    <th className="pb-2">Currency</th>
                    <th className="pb-2">Status</th>
                    <th className="pb-2 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-stone-800/60 font-mono">
                  {safeAccounts.map((acc) => {
                    const isPending = acc.status === 'PENDING_ACTIVATION'
                    return (
                      <tr key={acc.id} className="hover:bg-stone-850">
                        <td className="py-2.5 text-stone-300">
                          <button
                            onClick={() => handleCopy(acc.id)}
                            className="hover:text-amber-300 cursor-pointer"
                            title="Copy ID"
                          >
                            #{acc.id} {copiedId === acc.id ? '(copied)' : ''}
                          </button>
                        </td>
                        <td className="py-2.5 text-stone-200 font-sans">{acc.name}</td>
                        <td className="py-2.5 text-stone-400 font-sans">{acc.email}</td>
                        <td className="py-2.5 text-stone-400">{acc.currency}</td>
                        <td className="py-2.5">
                          <span
                            className={`px-1.5 py-0.5 rounded text-[11px] ${
                              isPending
                                ? 'bg-stone-800 text-amber-300 border border-amber-800/60'
                                : 'bg-stone-800 text-emerald-300 border border-emerald-800/60'
                            }`}
                          >
                            {acc.status}
                          </span>
                        </td>
                        <td className="py-2.5 text-right">
                          {isPending && (
                            <button
                              onClick={() => handleActivate(acc.id)}
                              disabled={activatingId === acc.id}
                              className="px-2 py-1 bg-[#422515] hover:bg-[#57311c] text-[#fed7aa] border border-[#78350f] rounded text-[11px] transition-colors cursor-pointer"
                            >
                              {activatingId === acc.id ? 'Activating...' : 'Activate'}
                            </button>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
