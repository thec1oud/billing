import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { planService, subscriptionService } from '../api/services';
import { useAppState } from '../context/AppState';
import type { Plan, Subscription } from '../api/types';

export const Plans: React.FC = () => {
  const { accountId, setSubscriptionId } = useAppState();
  const navigate = useNavigate();

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  const [availablePlans, setAvailablePlans] = useState<Plan[]>([]);
  const [subscribedPlans, setSubscribedPlans] = useState<Subscription[]>([]);

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
        const subbedPlanIds = new Set((activeSubs || []).map(s => s.PlanID));
        
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

  const handleSubscribe = async (plan: Plan) => {
    if (!accountId || !plan.plan_id || !plan.version) return;
    setLoading(true);
    setError(null);

    try {
      const subInput = {
        AccountID: accountId,
        PlanID: plan.plan_id, 
        PlanVersion: plan.version,
      };
      
      const subscription = await subscriptionService.create(subInput);
      setSubscriptionId(subscription.SubscriptionID);
      
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
                <div key={sub.SubscriptionID} className="webhook-item">
                  <div className="webhook-info">
                    <strong>Plan ID:</strong> {sub.PlanID} <br/>
                    <strong>Status:</strong> <span className="badge badge-open">{sub.Status}</span>
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
            No new plans available. You are subscribed to all of them!
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
      </div>
    </div>
  );
};
