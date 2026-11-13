import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { planService, subscriptionService, tariffService } from '../api/services';
import { useAppState } from '../context/AppState';
import type { Plan, Subscription } from '../api/types';

export const Plans: React.FC = () => {
  const { accountId, setSubscriptionId } = useAppState();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const [availablePlans, setAvailablePlans] = useState<Plan[]>([]);
  const [subscribedPlans, setSubscribedPlans] = useState<Subscription[]>([]);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [planCode, setPlanCode] = useState('');
  const [planName, setPlanName] = useState('');
  const [amount, setAmount] = useState('1000');
  const [durationDays, setDurationDays] = useState('30');

  useEffect(() => {
    if (!accountId) {
      navigate('/');
      return;
    }

    const fetchData = async () => {
      try {
        const [allPlans, activeSubs] = await Promise.all([
          planService.list(),
          subscriptionService.listForAccount(accountId),
        ]);

        setSubscribedPlans(activeSubs || []);
        
        // Filter out plans the user is already subscribed to
        const subbedPlanIds = new Set((activeSubs || []).map(s => s.plan_id));
        
        // Ensure plans is an array (in case backend returns null for empty list)
        const safePlans = allPlans || [];
        const filtered = safePlans.filter(p => !subbedPlanIds.has(p.plan_id!));
        
        setAvailablePlans(filtered);
      } catch (err: any) {
        setError(err.message || 'Failed to fetch plans');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [accountId, navigate]);

  const refreshPlans = async () => {
    const [allPlans, activeSubs] = await Promise.all([
      planService.list(),
      accountId ? subscriptionService.listForAccount(accountId) : Promise.resolve([]),
    ]);
    setSubscribedPlans(activeSubs);
    const subbedPlanIds = new Set(activeSubs.map(subscription => subscription.plan_id));
    setAvailablePlans((allPlans || []).filter(plan => !subbedPlanIds.has(plan.plan_id!)));
  };

  const handleCreatePlan = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!planCode.trim() || !planName.trim()) {
      setError('Plan code and name are required');
      return;
    }

    const amountMinor = Number(amount);
    const days = Number(durationDays);
    if (!Number.isInteger(amountMinor) || amountMinor <= 0 || !Number.isInteger(days) || days <= 0) {
      setError('Amount and duration must be positive whole numbers');
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const createdTariff = await tariffService.create({
        code: `${planCode.trim()}_tariff`,
        name: `${planName.trim()} pricing`,
        description: `${planName.trim()} pricing`,
        tariff_type_code: 'FLAT_FEE',
        amount: { amount_minor: amountMinor, currency: 'ETB' },
      });
      await planService.create({
        plan_code: planCode.trim(),
        legacy_price_policy_code: 'KEEP_FOREVER',
        durations: [{
          tariff_id: createdTariff.tariff_id,
          duration: days * 24 * 60 * 60 * 1_000_000_000,
          is_active: true,
        }],
      });
      setPlanCode('');
      setPlanName('');
      setShowCreateForm(false);
      await refreshPlans();
    } catch (err: any) {
      setError(err.message || 'Failed to create plan');
    } finally {
      setLoading(false);
    }
  };

  const handleSubscribe = async (plan: Plan) => {
    if (!accountId || !plan.plan_id || !plan.version) return;
    setLoading(true);
    setError(null);

    try {
      const subInput = {
        account_id: accountId,
        plan_id: plan.plan_id,
        plan_version: plan.version,
      };
      
      const subscription = await subscriptionService.create(subInput);
      setSubscriptionId(subscription.id);
      
      navigate('/invoice', { state: { planId: plan.plan_id } });
    } catch (err: any) {
      setError(err.message || "Failed to subscribe to plan");
      setLoading(false);
    }
  };

  if (loading) {
    return <div className="container"><div className="card">Loading plans...</div></div>;
  }

  return (
    <div className="container">
      <div className="card">
        <div className="card-header">
          <h2>Available Plans</h2>
          <p>Select a plan to subscribe.</p>
        </div>

        {error && <div className="alert-error">{error}</div>}

        {subscribedPlans.length > 0 && (
          <div style={{ marginBottom: '2rem' }}>
            <h3>Your Subscriptions</h3>
            <div className="webhook-list" style={{ marginTop: '1rem' }}>
              {subscribedPlans.map(sub => (
                <div key={sub.id} className="webhook-item">
                  <div className="webhook-info">
                    <strong>Plan ID:</strong> {sub.plan_id} <br/>
                    <strong>Status:</strong> <span className="badge badge-open">{sub.status}</span>
                  </div>
                </div>
              ))}
            </div>
            <hr style={{ margin: '2rem 0', borderColor: 'var(--border-color)' }} />
          </div>
        )}

        <h3>Plans</h3>
        {availablePlans.length === 0 ? (
          <div className="empty-state" style={{ marginTop: '1rem' }}>
            No plans are available yet. Create the first plan below.
          </div>
        ) : (
          <div className="webhook-list" style={{ marginTop: '1rem' }}>
            {availablePlans.map(plan => (
              <div 
                key={plan.plan_id} 
                className="plan-card" 
                style={{ marginBottom: '0', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}
              >
                <div>
                  <h3>{plan.plan_code}</h3>
                  <p style={{ marginBottom: 0 }}>Version {plan.version}</p>
                </div>
                <button 
                  className="btn btn-primary"
                  style={{ width: 'auto' }}
                  onClick={() => handleSubscribe(plan)}
                >
                  Subscribe
                </button>
              </div>
            ))}
          </div>
        )}

        <div style={{ marginTop: '2rem' }}>
          <button className="btn btn-secondary" onClick={() => setShowCreateForm(value => !value)}>
            {showCreateForm ? 'Cancel' : 'Create Plan'}
          </button>
          {showCreateForm && (
            <form onSubmit={handleCreatePlan} style={{ display: 'grid', gap: '1rem', marginTop: '1rem' }}>
              <input value={planCode} onChange={event => setPlanCode(event.target.value)} placeholder="Plan code, e.g. starter" />
              <input value={planName} onChange={event => setPlanName(event.target.value)} placeholder="Plan name" />
              <input type="number" min="1" value={amount} onChange={event => setAmount(event.target.value)} placeholder="Amount in minor units" />
              <input type="number" min="1" value={durationDays} onChange={event => setDurationDays(event.target.value)} placeholder="Duration in days" />
              <button className="btn btn-primary" type="submit" disabled={loading}>Create plan</button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
};
