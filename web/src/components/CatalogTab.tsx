import React, { useState, useEffect } from 'react'
import { Plan, PurchasableItem } from '../types/api'
import { billingApi } from '../services/apiClient'

export const CatalogTab: React.FC = () => {
  const [plans, setPlans] = useState<Plan[]>([])
  const [items, setItems] = useState<PurchasableItem[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchCatalog = async () => {
    setLoading(true)
    setError(null)
    try {
      const [plansRes, itemsRes] = await Promise.all([
        billingApi.listPlans().catch(() => ({ data: [] as Plan[], status: 200 })),
        billingApi.listPurchasableItems().catch(() => ({ data: [] as PurchasableItem[], status: 200 })),
      ])
      setPlans(Array.isArray(plansRes?.data) ? plansRes.data : [])
      setItems(Array.isArray(itemsRes?.data) ? itemsRes.data : [])
    } catch (err: any) {
      setError(err.message || 'Failed to fetch catalog')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchCatalog()
  }, [])

  const safePlans = Array.isArray(plans) ? plans : []
  const safeItems = Array.isArray(items) ? items : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">Catalog & Plans</h2>
        <button
          onClick={fetchCatalog}
          disabled={loading}
          className="px-3 py-1.5 bg-[#422515] hover:bg-[#57311c] text-[#fed7aa] border border-[#78350f] rounded text-xs transition-colors cursor-pointer"
        >
          {loading ? 'Refreshing...' : 'Refresh'}
        </button>
      </div>

      {error && (
        <div className="p-3 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Plans */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            Plans ({safePlans.length})
          </h3>

          {safePlans.length === 0 ? (
            <div className="py-8 text-center text-stone-500 text-xs">
              No plans available.
            </div>
          ) : (
            <div className="space-y-2">
              {safePlans.map((p) => (
                <div
                  key={p.id}
                  className="p-3 rounded bg-[#12100e] border border-stone-800 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-mono font-medium text-stone-200">{p.plan_code}</span>
                    <span className="px-1.5 py-0.5 rounded text-[11px] bg-stone-800 text-stone-300 border border-stone-700">
                      v{p.version}
                    </span>
                  </div>
                  <div className="mt-2 text-stone-400 font-mono text-[11px] flex gap-4">
                    <span>ID: #{p.id}</span>
                    <span>Policy: {p.legacy_price_policy_code || 'KEEP_FOREVER'}</span>
                    <span>From: {new Date(p.effective_from).toLocaleDateString()}</span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Purchasable Items */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider mb-3">
            Purchasable Items ({safeItems.length})
          </h3>

          {safeItems.length === 0 ? (
            <div className="py-8 text-center text-stone-500 text-xs">
              No purchasable items available.
            </div>
          ) : (
            <div className="space-y-2">
              {safeItems.map((item) => (
                <div
                  key={item.id}
                  className="p-3 rounded bg-[#12100e] border border-stone-800 text-xs"
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-stone-200">{item.name}</span>
                    <span className="px-1.5 py-0.5 rounded text-[11px] bg-stone-800 text-stone-300 border border-stone-700">
                      {item.is_active ? 'ACTIVE' : 'INACTIVE'}
                    </span>
                  </div>
                  <div className="mt-2 text-stone-400 font-mono text-[11px] flex gap-4">
                    <span>Code: {item.item_code}</span>
                    <span>Type: {item.item_type_code}</span>
                    {item.plan_id && <span>Plan ID: #{item.plan_id}</span>}
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
