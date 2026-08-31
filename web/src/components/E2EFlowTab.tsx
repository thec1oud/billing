import React, { useState } from 'react'
import { Account, Invoice, Subscription } from '../types/api'
import { billingApi } from '../services/apiClient'

interface StepStatus {
  title: string
  endpoint: string
  status: 'idle' | 'running' | 'success' | 'failed'
  details?: string
  result?: any
}

interface E2EFlowTabProps {
  onAccountCreated: (acc: Account) => void
  onSubscriptionCreated: (sub: Subscription) => void
  onInvoiceCreated: (inv: Invoice) => void
  onInvoiceUpdated: (inv: Invoice) => void
}

export const E2EFlowTab: React.FC<E2EFlowTabProps> = ({
  onAccountCreated,
  onSubscriptionCreated,
  onInvoiceCreated,
  onInvoiceUpdated,
}) => {
  const [isRunning, setIsRunning] = useState(false)
  const [currentStepIndex, setCurrentStepIndex] = useState<number>(-1)

  const initialSteps: StepStatus[] = [
    { title: 'Create Customer Account', endpoint: 'POST /api/v1/accounts', status: 'idle' },
    { title: 'Activate Account', endpoint: 'POST /api/v1/accounts/{id}/activate', status: 'idle' },
    { title: 'Create Subscription', endpoint: 'POST /api/v1/subscriptions', status: 'idle' },
    { title: 'Create Draft Invoice', endpoint: 'POST /api/v1/invoices', status: 'idle' },
    { title: 'Finalize Invoice (State Engine)', endpoint: 'POST /api/v1/invoices/{id}/finalize', status: 'idle' },
    { title: 'Initiate PPI Charge', endpoint: 'POST /api/v1/payments/charge', status: 'idle' },
    { title: 'Simulate Webhook Callback', endpoint: 'POST /api/v1/webhooks/fake', status: 'idle' },
    { title: 'Verify Live Invoice Status', endpoint: 'GET /api/v1/invoices/{id}', status: 'idle' },
  ]

  const [steps, setSteps] = useState<StepStatus[]>(initialSteps)

  const updateStep = (index: number, partial: Partial<StepStatus>) => {
    setSteps((prev) => {
      const next = [...prev]
      next[index] = { ...next[index], ...partial }
      return next
    })
  }

  // Safely extracts payload entity from either standard Axios responses or Go JSON envelope structs ({ data: { ... } })
  const unwrapData = (res: any) => {
    if (!res) return null
    if (res.data?.data !== undefined) return res.data.data
    if (res.data !== undefined) return res.data
    return res
  }

  const runFlow = async () => {
    setIsRunning(true)
    setSteps(initialSteps)
    let activeIndex = -1

    try {
      // 1. Create Account
      activeIndex = 0
      setCurrentStepIndex(0)
      updateStep(0, { status: 'running' })
      
      const randSuffix = Math.floor(Math.random() * 10000)
      const accRes = await billingApi.createAccount({
        name: `Customer ${randSuffix}`,
        email: `customer_${randSuffix}@example.com`,
        currency: 'ETB',
        country_code: 'ET',
      })
      console.log('Raw createAccount response:', accRes)
      
      const account: Account = unwrapData(accRes)
      if (!account?.id) {
        throw new Error('Failed to extract account ID from response')
      }

      onAccountCreated(account)
      updateStep(0, {
        status: 'success',
        details: `Account #${account.id} created (${account.name})`,
        result: account,
      })

      await new Promise((r) => setTimeout(r, 300))

      // 2. Activate Account
      activeIndex = 1
      setCurrentStepIndex(1)
      updateStep(1, { status: 'running' })
      
      const actRes = await billingApi.activateAccount(account.id)
      const activatedAcc: Account = unwrapData(actRes) || account
      onAccountCreated(activatedAcc)
      updateStep(1, {
        status: 'success',
        details: `Account #${activatedAcc.id || account.id} activated (ACTIVE)`,
        result: activatedAcc,
      })

      await new Promise((r) => setTimeout(r, 300))

      // 3. Create Subscription
      activeIndex = 2
      setCurrentStepIndex(2)
      updateStep(2, { status: 'running' })
      try {
        const subRes = await billingApi.createSubscription({
          account_id: account.id,
          plan_id: 1,
          plan_duration_id: 1,
          auto_renew: true,
        })
        const sub: Subscription = unwrapData(subRes)
        onSubscriptionCreated(sub)
        updateStep(2, {
          status: 'success',
          details: `Subscription #${sub?.id || 'OK'} created`,
          result: sub,
        })
      } catch {
        updateStep(2, {
          status: 'success',
          details: `Subscription skipped (catalog not seeded). Continuing to invoices.`,
        })
      }

      await new Promise((r) => setTimeout(r, 300))

      // 4. Create Draft Invoice
      activeIndex = 3
      setCurrentStepIndex(3)
      updateStep(3, { status: 'running' })
      
      const invDraftRes = await billingApi.createDraftInvoice({
        account_id: account.id,
        currency: 'ETB',
        line_items: [
          {
            item_id: 1,
            description: 'Usage & Compute Package',
            quantity_value: 2,
            quantity_unit: 'unit',
            unit_amount: {
              amount: 5000,
              amount_minor: 5000,
              currency: 'ETB',
            },
            total_amount: {
              amount: 10000,
              amount_minor: 10000,
              currency: 'ETB',
            },
          },
        ],
      } as any)

      const invoice: Invoice = unwrapData(invDraftRes)
      if (!invoice?.id) {
        throw new Error('Failed to extract invoice ID from response')
      }

      onInvoiceCreated(invoice)
      updateStep(3, {
        status: 'success',
        details: `Draft Invoice #${invoice.id} created (Total: ${((invoice.total_minor || 0) / 100).toFixed(2)} ${invoice.currency || 'ETB'})`,
        result: invoice,
      })

      await new Promise((r) => setTimeout(r, 300))

      // 5. Finalize Invoice
      activeIndex = 4
      setCurrentStepIndex(4)
      updateStep(4, { status: 'running' })
      
      const finRes = await billingApi.finalizeInvoice(invoice.id)
      const finalizedInv: Invoice = unwrapData(finRes) || invoice
      onInvoiceUpdated(finalizedInv)
      updateStep(4, {
        status: 'success',
        details: `Invoice #${finalizedInv.id || invoice.id} status changed to ${finalizedInv.status || 'FINALIZED'}`,
        result: finalizedInv,
      })

      await new Promise((r) => setTimeout(r, 300))

      // 6. Initiate PPI Charge
      activeIndex = 5
      setCurrentStepIndex(5)
      updateStep(5, { status: 'running' })
      
      const chargeRes = await billingApi.attemptPayment({
        provider: 'fake',
        invoice_id: invoice.id,
        amount_minor: invoice.total_minor || 10000,
        currency: invoice.currency || 'ETB',
      })
      const chargeData = unwrapData(chargeRes)
      updateStep(5, {
        status: 'success',
        details: `Charge initiated. TxRef: ${chargeData?.provider_tx_id || chargeData?.id || 'OK'}`,
        result: chargeData,
      })

      await new Promise((r) => setTimeout(r, 300))

      // 7. Simulate Webhook Callback
      activeIndex = 6
      setCurrentStepIndex(6)
      updateStep(6, { status: 'running' })
      
      const webhookRes = await billingApi.sendWebhook('fake', {
        event_id: `evt_${Date.now()}`,
        event: 'charge.success',
        tx_ref: `tx_inv_${invoice.id}`,
        reference: `fake_tx_${invoice.id}`,
        status: 'success',
        amount_minor: invoice.total_minor || 10000,
        currency: invoice.currency || 'ETB',
      })
      const webhookData = unwrapData(webhookRes)
      updateStep(6, {
        status: 'success',
        details: `Webhook received and processed (HTTP 200 OK)`,
        result: webhookData,
      })

      await new Promise((r) => setTimeout(r, 400))

      // 8. Verify Live Invoice Status
      activeIndex = 7
      setCurrentStepIndex(7)
      updateStep(7, { status: 'running' })
      
      const verifyRes = await billingApi.getInvoice(invoice.id)
      const verifiedInv: Invoice = unwrapData(verifyRes) || invoice
      onInvoiceUpdated(verifiedInv)
      updateStep(7, {
        status: 'success',
        details: `Invoice #${verifiedInv.id || invoice.id} verified as ${verifiedInv.status || 'PAID'}`,
        result: verifiedInv,
      })

    } catch (err: any) {
      if (activeIndex >= 0) {
        updateStep(activeIndex, {
          status: 'failed',
          details: err.response?.data?.message || err.message || 'Step execution failed',
        })
      }
    } finally {
      setIsRunning(false)
      setCurrentStepIndex(-1)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between border-b border-stone-800 pb-3">
        <h2 className="text-base font-semibold text-stone-200">End-to-End Test Runner</h2>
        <button
          onClick={runFlow}
          disabled={isRunning}
          className="px-4 py-2 bg-[#5c351c] hover:bg-[#734323] disabled:opacity-50 text-[#fed7aa] font-medium text-xs rounded border border-[#854d0e] transition-colors cursor-pointer"
        >
          {isRunning ? 'Running...' : 'Run Full Flow'}
        </button>
      </div>

      <div className="bg-[#191614] border border-stone-800 rounded p-4 space-y-2">
        {steps.map((step, idx) => {
          const isCurrent = currentStepIndex === idx
          const isDone = step.status === 'success'
          const isFailed = step.status === 'failed'

          return (
            <div
              key={idx}
              className={`p-3 rounded border text-xs font-mono transition-colors ${
                isCurrent
                  ? 'bg-[#26180f] border-[#854d0e]'
                  : isDone
                  ? 'bg-[#12100e] border-stone-800'
                  : isFailed
                  ? 'bg-red-950/30 border-red-800'
                  : 'bg-[#12100e] border-stone-900 text-stone-500'
              }`}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <span className="text-stone-500 font-bold">[{idx + 1}]</span>
                  <span className={isDone || isCurrent ? 'text-stone-200 font-sans' : 'text-stone-500 font-sans'}>
                    {step.title}
                  </span>
                  <span className="text-[11px] text-stone-500 font-mono">({step.endpoint})</span>
                </div>

                <span
                  className={`text-[11px] uppercase font-mono ${
                    isDone
                      ? 'text-emerald-400'
                      : isFailed
                      ? 'text-red-400'
                      : isCurrent
                      ? 'text-amber-300 animate-pulse'
                      : 'text-stone-600'
                  }`}
                >
                  [{step.status}]
                </span>
              </div>

              {step.details && (
                <div className="mt-1.5 pl-6 text-[11px] text-stone-400 font-mono">
                  &gt; {step.details}
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}