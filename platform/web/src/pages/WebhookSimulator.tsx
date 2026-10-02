import React, { useState } from 'react';
import { webhookService } from '../api/services';
import { useAppState } from '../context/AppState';

export const WebhookSimulator: React.FC = () => {
  const { pendingWebhooks, removePendingWebhook } = useAppState();
  const [processingId, setProcessingId] = useState<string | null>(null);
  const [message, setMessage] = useState<{ type: 'success' | 'error', text: string } | null>(null);

  const handleSimulateWebhook = async (webhook: any) => {
    setProcessingId(webhook.internal_tx_id);
    setMessage(null);

    try {
      await webhookService.simulateFakeProvider({
        event_id: `wh_evt_${Date.now()}`,
        event: 'charge.success',
        tx_ref: webhook.internal_tx_id,
        reference: webhook.provider_reference,
        status: 'success',
        amount_minor: webhook.amount,
        currency: 'ETB'
      });

      setMessage({ type: 'success', text: `Webhook sent successfully for TX: ${webhook.internal_tx_id}` });
      removePendingWebhook(webhook.internal_tx_id);
    } catch (err: any) {
      setMessage({ type: 'error', text: err.message || 'Failed to send webhook' });
    } finally {
      setProcessingId(null);
    }
  };

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Dev Webhook Simulator</h2>
          <p>Fire success webhooks for pending payments</p>
        </div>

        {message && (
          <div className={message.type === 'success' ? 'alert-success' : 'alert-error'}>
            {message.text}
          </div>
        )}

        {pendingWebhooks.length === 0 ? (
          <div className="empty-state">
            No pending webhooks. Complete a checkout first!
          </div>
        ) : (
          <div className="webhook-list">
            {pendingWebhooks.map((wh) => (
              <div className="webhook-item" key={wh.internal_tx_id}>
                <div className="webhook-info">
                  <strong>Internal TX:</strong> {wh.internal_tx_id}<br/>
                  <strong>Provider Ref:</strong> {wh.provider_reference}
                </div>
                <button 
                  className="btn btn-primary"
                  onClick={() => handleSimulateWebhook(wh)}
                  disabled={processingId === wh.internal_tx_id}
                >
                  {processingId === wh.internal_tx_id ? 'Sending...' : 'Fire Webhook'}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};
