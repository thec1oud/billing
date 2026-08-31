import React, { useState } from 'react'
import { Invoice, ChargeRequest, ChargeResult } from '../types/api'
import { billingApi } from '../services/apiClient'

interface PaymentsTabProps {
  invoices: Invoice[]
  onInvoiceUpdated: (inv: Invoice) => void
}

export const PaymentsTab: React.FC<PaymentsTabProps> = ({ invoices }) => {
  // Outbound Charge State
  const [provider, setProvider] = useState('fake')
  const [invoiceId, setInvoiceId] = useState<number>(invoices[0]?.id || 1)
  const [amountMinor, setAmountMinor] = useState<number>(10000)
  const [currency, setCurrency] = useState('ETB')
  const [chargeLoading, setChargeLoading] = useState(false)
  const [chargeResult, setChargeResult] = useState<ChargeResult | null>(null)
  const [chargeError, setChargeError] = useState<string | null>(null)

  // Inbound Webhook Simulator State
  const [webhookProvider, setWebhookProvider] = useState('fake')
  const [webhookPayload, setWebhookPayload] = useState<string>(
    JSON.stringify(
      {
        event_id: `evt_sim_${Math.floor(Math.random() * 10000)}`,
        event: 'charge.success',
        tx_ref: `tx_inv_${invoices[0]?.id || 1}`,
        reference: `fake_ref_${Math.floor(Math.random() * 10000)}`,
        status: 'success',
        amount_minor: 10000,
        currency: 'ETB',
      },
      null,
      2
    )
  )
  const [webhookLoading, setWebhookLoading] = useState(false)
  const [webhookResult, setWebhookResult] = useState<Record<string, unknown> | null>(null)
  const [webhookError, setWebhookError] = useState<string | null>(null)

  const handleCharge = async (e: React.FormEvent) => {
    e.preventDefault()
    setChargeLoading(true)
    setChargeError(null)
    setChargeResult(null)
    try {
      const req: ChargeRequest = {
        provider,
        invoice_id: Number(invoiceId),
        amount_minor: Number(amountMinor),
        currency,
      }
      const res = await billingApi.attemptPayment(req)
      setChargeResult(res.data)
    } catch (err: any) {
      setChargeError(err.message || 'Charge request failed')
    } finally {
      setChargeLoading(false)
    }
  }

  const handleApplyPreset = (preset: 'success' | 'failed' | 'duplicate') => {
    const invId = invoiceId || 1
    if (preset === 'success') {
      setWebhookPayload(
        JSON.stringify(
          {
            event_id: `evt_sim_${Date.now()}`,
            event: 'charge.success',
            tx_ref: `tx_inv_${invId}`,
            reference: `fake_ref_${Date.now()}`,
            status: 'success',
            amount_minor: amountMinor,
            currency: currency,
          },
          null,
          2
        )
      )
    } else if (preset === 'failed') {
      setWebhookPayload(
        JSON.stringify(
          {
            event_id: `evt_sim_${Date.now()}`,
            event: 'charge.failed',
            tx_ref: `tx_inv_${invId}`,
            reference: `fake_ref_${Date.now()}`,
            status: 'failed',
            error: 'INSUFFICIENT_FUNDS',
          },
          null,
          2
        )
      )
    } else if (preset === 'duplicate') {
      setWebhookPayload(
        JSON.stringify(
          {
            event_id: `evt_dup_same_id`,
            event: 'charge.success',
            tx_ref: `tx_inv_${invId}`,
            reference: `fake_ref_duplicate`,
            status: 'success',
          },
          null,
          2
        )
      )
    }
  }

  const handleSendWebhook = async (e: React.FormEvent) => {
    e.preventDefault()
    setWebhookLoading(true)
    setWebhookError(null)
    setWebhookResult(null)
    try {
      const parsed = JSON.parse(webhookPayload)
      const res = await billingApi.sendWebhook(webhookProvider, parsed)
      setWebhookResult(res.data)
    } catch (err: any) {
      setWebhookError(err.message || 'Failed to trigger webhook')
    } finally {
      setWebhookLoading(false)
    }
  }

  const safeInvoices = Array.isArray(invoices) ? invoices : []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">Payments & Webhooks</h2>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Outbound Charge */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4 space-y-3">
          <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider">
            1. Outbound Payment Charge
          </h3>

          {chargeError && (
            <div className="p-2.5 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
              {chargeError}
            </div>
          )}

          <form onSubmit={handleCharge} className="space-y-3">
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-stone-400 mb-1">Provider</label>
                <select
                  value={provider}
                  onChange={(e) => setProvider(e.target.value)}
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                >
                  <option value="fake">fake</option>
                  <option value="chapa">chapa</option>
                  <option value="telebirr">telebirr</option>
                </select>
              </div>

              <div>
                <label className="block text-xs text-stone-400 mb-1">Invoice</label>
                {safeInvoices.length > 0 ? (
                  <select
                    value={invoiceId}
                    onChange={(e) => setInvoiceId(Number(e.target.value))}
                    className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                  >
                    {safeInvoices.map((inv) => (
                      <option key={inv.id} value={inv.id}>
                        Invoice #{inv.id} ({inv.status})
                      </option>
                    ))}
                  </select>
                ) : (
                  <input
                    type="number"
                    value={invoiceId}
                    onChange={(e) => setInvoiceId(Number(e.target.value))}
                    required
                    className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                  />
                )}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-stone-400 mb-1">Amount Minor (cents)</label>
                <input
                  type="number"
                  value={amountMinor}
                  onChange={(e) => setAmountMinor(Number(e.target.value))}
                  required
                  className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e]"
                />
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
                </select>
              </div>
            </div>

            <button
              type="submit"
              disabled={chargeLoading}
              className="w-full mt-2 py-2 px-3 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
            >
              {chargeLoading ? 'Initiating...' : 'Initiate Charge'}
            </button>
          </form>

          {chargeResult && (
            <div className="p-3 rounded bg-[#12100e] border border-stone-800 space-y-1 text-xs font-mono">
              <div className="text-emerald-400 font-semibold">
                Status: {chargeResult.status} ({chargeResult.provider_tx_id})
              </div>
              {chargeResult.checkout_url && (
                <div className="text-stone-400 text-[11px] truncate">
                  URL: <a href={chargeResult.checkout_url} target="_blank" rel="noreferrer" className="text-[#fed7aa] underline">{chargeResult.checkout_url}</a>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Webhook Simulator */}
        <div className="bg-[#191614] border border-stone-800 rounded p-4 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-xs font-semibold text-stone-300 uppercase tracking-wider">
              2. Inbound Webhook Simulator
            </h3>
          </div>

          <div className="flex items-center gap-1.5">
            <span className="text-xs text-stone-400">Presets:</span>
            <button
              type="button"
              onClick={() => handleApplyPreset('success')}
              className="px-2 py-0.5 bg-stone-900 hover:bg-stone-800 text-stone-300 border border-stone-700 rounded text-[11px] cursor-pointer"
            >
              Success
            </button>
            <button
              type="button"
              onClick={() => handleApplyPreset('failed')}
              className="px-2 py-0.5 bg-stone-900 hover:bg-stone-800 text-stone-300 border border-stone-700 rounded text-[11px] cursor-pointer"
            >
              Failed
            </button>
            <button
              type="button"
              onClick={() => handleApplyPreset('duplicate')}
              className="px-2 py-0.5 bg-stone-900 hover:bg-stone-800 text-stone-300 border border-stone-700 rounded text-[11px] cursor-pointer"
            >
              Duplicate
            </button>
          </div>

          {webhookError && (
            <div className="p-2.5 bg-red-950/40 border border-red-800 text-red-300 text-xs rounded">
              {webhookError}
            </div>
          )}

          <form onSubmit={handleSendWebhook} className="space-y-3">
            <div>
              <label className="block text-xs text-stone-400 mb-1">Provider Code</label>
              <input
                type="text"
                value={webhookProvider}
                onChange={(e) => setWebhookProvider(e.target.value)}
                required
                className="w-full px-2.5 py-1.5 bg-[#12100e] border border-stone-800 rounded text-xs text-stone-200 focus:outline-none focus:border-[#854d0e] font-mono"
              />
            </div>

            <div>
              <label className="block text-xs text-stone-400 mb-1">Payload (JSON)</label>
              <textarea
                value={webhookPayload}
                onChange={(e) => setWebhookPayload(e.target.value)}
                rows={5}
                required
                className="w-full p-2 bg-[#12100e] border border-stone-800 rounded text-xs font-mono text-stone-300 focus:outline-none focus:border-[#854d0e]"
              />
            </div>

            <button
              type="submit"
              disabled={webhookLoading}
              className="w-full py-2 px-3 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
            >
              {webhookLoading ? 'Sending...' : 'Send Webhook'}
            </button>
          </form>

          {webhookResult && (
            <div className="p-3 rounded bg-[#12100e] border border-stone-800 space-y-1 text-xs font-mono">
              <div className="text-stone-400 font-semibold">Response:</div>
              <pre className="text-stone-300 text-[11px] overflow-x-auto">
                {JSON.stringify(webhookResult, null, 2)}
              </pre>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
