import React, { useEffect, useState } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { invoiceService } from '../api/services';
import { useAppState } from '../context/AppState';
import type { Invoice } from '../api/types';

export const DevInvoice: React.FC = () => {
  const { accountId, setInvoiceId } = useAppState();
  const navigate = useNavigate();
  const location = useLocation();
  const planId = location.state?.planId;

  const [invoice, setInvoice] = useState<Invoice | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!accountId) {
      navigate('/');
      return;
    }

    if (!planId) {
      setError('No plan ID found. Please subscribe to a plan first.');
      setLoading(false);
      return;
    }

    const generateInvoice = async () => {
      try {
        const newInvoice = await invoiceService.generateDevInvoice({
          account_id: accountId,
          plan_id: planId,
        });
        setInvoice(newInvoice);
        setInvoiceId(newInvoice.invoice_id);
      } catch (err: any) {
        setError(err.message || 'Failed to generate dev invoice');
      } finally {
        setLoading(false);
      }
    };

    generateInvoice();
  }, [accountId, navigate, setInvoiceId]);

  if (loading) {
    return <div className="container"><div className="card">Generating your invoice...</div></div>;
  }

  if (error) {
    return <div className="container"><div className="card alert-error">{error}</div></div>;
  }

  if (!invoice) return null;

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Invoice #{invoice.invoice_id}</h2>
          <span className="badge badge-open">{invoice.status}</span>
        </div>

        <div className="invoice-details">
          {invoice.line_items.map((item, idx) => (
            <div className="line-item" key={idx}>
              <div>
                <strong>{item.description}</strong>
                <p>Qty: {item.quantity_value} {item.quantity_unit}</p>
              </div>
              <div>
                {item.total_amount.amount_minor} {item.total_amount.currency}
              </div>
            </div>
          ))}
          
          <div className="totals">
            <div className="total-row">
              <span>Subtotal:</span>
              <span>{invoice.subtotal.amount_minor} {invoice.subtotal.currency}</span>
            </div>
            <div className="total-row grand-total">
              <span>Total Due:</span>
              <span>{invoice.amount_due.amount_minor} {invoice.amount_due.currency}</span>
            </div>
          </div>
        </div>

        <button 
          className="btn btn-primary"
          onClick={() => navigate('/checkout')}
        >
          Proceed to Checkout
        </button>
      </div>
    </div>
  );
};
