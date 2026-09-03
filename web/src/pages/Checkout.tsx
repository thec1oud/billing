import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { invoiceService } from '../api/services';
import { useAppState } from '../context/AppState';

export const Checkout: React.FC = () => {
  const { invoiceId, addPendingWebhook } = useAppState();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handlePay = async (providerCode: string) => {
    if (!invoiceId) return;

    setLoading(true);
    setError(null);
    try {
      // 1. Pay the invoice
      const response = await invoiceService.pay(invoiceId, {
        provider_code: providerCode,
        idempotency_key: `req_${Date.now()}_${Math.floor(Math.random() * 1000)}`,
      });

      // 2. Save the critical UI state for webhook simulation
      addPendingWebhook({
        internal_tx_id: response.internal_tx_id,
        provider_reference: response.provider_reference,
        amount: 1000, // Normally we'd get this from the invoice state, hardcoding for simplicity
      });

      // 3. Simulate redirect to payment provider, then return to our webhook simulator
      setTimeout(() => {
        navigate('/webhook-simulator');
      }, 800);

    } catch (err: any) {
      setError(err.message || 'Payment failed');
      setLoading(false);
    }
  };

  if (!invoiceId) {
    return (
      <div className="container">
        <div className="card">
          <p>No active invoice found.</p>
          <button className="btn btn-secondary" onClick={() => navigate('/')}>Go Home</button>
        </div>
      </div>
    );
  }

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Checkout</h2>
          <p>Select your payment method</p>
        </div>

        {error && <div className="alert-error">{error}</div>}

        <div className="payment-methods">
          <button 
            className="btn btn-secondary payment-btn"
            onClick={() => handlePay('fake')}
            disabled={loading}
          >
            {loading ? 'Processing...' : 'Pay with Fake Provider (Test)'}
          </button>
        </div>
      </div>
    </div>
  );
};
