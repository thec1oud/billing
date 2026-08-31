import React, { useState } from 'react'
import { Account, Subscription, CreateSubscriptionInput } from '../types/api'
import { billingApi } from '../services/apiClient'

interface SubscriptionsTabProps {
  accounts: Account[]
  subscriptions: Subscription[]
  onSubscriptionCreated: (sub: Subscription) => void
}

export const SubscriptionsTab: React.FC<SubscriptionsTabProps> = ({
  accounts,
  subscriptions,
  onSubscriptionCreated,
}) => {
  const [accountId, setAccountId] = useState<number>(accounts[0]?.id || 1)
  const [planId, setPlanId] = useState<number>(1)
  const [planDurationId, setPlanDurationId] = useState<number>(1)
  const [autoRenew, setAutoRenew] = useState<boolean>(true)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    try {
      const input: CreateSubscriptionInput = {
        account_id: Number(accountId),
        plan_id: Number(planId),
        plan_duration_id: Number(planDurationId),
        auto_renew: autoRenew,
      }
      const res = await billingApi.createSubscription(input)
      onSubscriptionCreated(res.data)
    } catch (err: any) {
      setError(err.message || 'Failed to create subscription')
    } finally {
      setLoading(false)
    }
  }

  const safeAccounts = Array.isArray(accounts) ? accounts : []
  const safeSubscriptions = Array.isArray(subscriptions) ? subscriptions : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">Subscriptions</h2>
      </div>

      {error && (
        <div className="p-3 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Form */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            New Subscription
          </h3>

          <form onSubmit={handleCreate} className="space-y-3">
            <div>
              <label className="block text-xs text-stone-400 mb-1">Account</label>
              {safeAccounts.length > 0 ? (
                <select
                  value={accountId}
                  onChange={(e) => setAccountId(Number(e.target.value))}
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                >
                  {safeAccounts.map((acc) => (
                    <option key={acc.id} value={acc.id}>
                      #{acc.id} - {acc.name} ({acc.status})
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="number"
                  value={accountId}
                  onChange={(e) => setAccountId(Number(e.target.value))}
                  placeholder="Account ID"
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                />
              )}
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-stone-400 mb-1">Plan ID</label>
                <input
                  type="number"
                  value={planId}
                  onChange={(e) => setPlanId(Number(e.target.value))}
                  required
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                />
              </div>

              <div>
                <label className="block text-xs text-stone-400 mb-1">Duration ID</label>
                <input
                  type="number"
                  value={planDurationId}
                  onChange={(e) => setPlanDurationId(Number(e.target.value))}
                  required
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                />
              </div>
            </div>

            <div className="flex items-center space-x-2 pt-1">
              <input
                type="checkbox"
                id="autoRenew"
                checked={autoRenew}
                onChange={(e) => setAutoRenew(e.target.checked)}
                className="rounded border-stone-700 bg-[#12100e] accent-[#854d0e]"
              />
              <label htmlFor="autoRenew" className="text-xs text-stone-300 select-none cursor-pointer">
                Auto Renew
              </label>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full mt-2 py-2 px-3 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
            >
              {loading ? 'Creating...' : 'Create Subscription'}
            </button>
          </form>
        </div>

        {/* List */}
        <div className="lg:col-span-2 bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            Subscriptions List ({safeSubscriptions.length})
          </h3>

          {safeSubscriptions.length === 0 ? (
            <div className="py-8 text-center text-stone-500 text-xs">
              No subscriptions created yet.
            </div>
          ) : (
            <div className="space-y-2">
              {safeSubscriptions.map((sub) => (
                <div
                  key={sub.id}
                  className="p-3 rounded bg-[#12100e] border border-stone-800 text-xs flex items-center justify-between font-mono"
                >
                  <div>
                    <div className="text-stone-200 font-semibold">
                      Subscription #{sub.id}
                      <span className="ml-2 px-1.5 py-0.5 rounded text-[11px] bg-stone-800 text-emerald-300 border border-emerald-800/60 font-normal">
                        {sub.status || 'ACTIVE'}
                      </span>
                    </div>
                    <div className="mt-1 text-stone-400 text-[11px] flex gap-4">
                      <span>Account: #{sub.account_id}</span>
                      <span>Plan: #{sub.plan_id}</span>
                      <span>Duration: #{sub.plan_duration_id}</span>
                    </div>
                  </div>
                  <div className="text-stone-400 text-[11px]">
                    Auto-renew: {sub.auto_renew ? 'Yes' : 'No'}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
