export interface Account {
  id: number;
  name: string;
  email: string;
  currency: string;
  country_code: string;
  status: 'PENDING_ACTIVATION' | 'ACTIVE' | 'SUSPENDED' | 'CLOSED';
  created_at: string;
}

export interface CreateAccountInput {
  name: string;
  email: string;
  currency: string;
  country_code: string;
}

export interface Plan {
  id: number;
  plan_code: string;
  version: number;
  effective_from: string;
  effective_until?: string | null;
  legacy_price_policy_code: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface PurchasableItem {
  id: number;
  item_code: string;
  item_type_code: string;
  name: string;
  description?: string | null;
  plan_id?: number | null;
  is_active: boolean;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export interface Subscription {
  id: number;
  account_id: number;
  plan_id: number;
  plan_duration_id: number;
  status: string;
  auto_renew: boolean;
  current_period_start: string;
  current_period_end: string;
  created_at: string;
}

export interface CreateSubscriptionInput {
  account_id: number;
  plan_id: number;
  plan_duration_id: number;
  auto_renew: boolean;
}

export interface Money {
  amount?: number;
  amount_minor?: number;
  currency: string;
}

export interface LineItem {
  line_item_id?: number;
  item_id: number;
  description: string;
  quantity_value: number;
  quantity_unit: string;
  unit_amount: Money;
  total_amount: Money;
  metadata?: Record<string, any>;
  subscription_id?: number;
}

export interface CreateDraftInvoiceInput {
  account_id: number;
  subscription_id?: number;
  currency: string;
  line_items: LineItem[];
}

export interface Invoice {
  id: number;
  account_id: number;
  invoice_number?: string;
  status: 'DRAFT' | 'ISSUED' | 'PAYMENT_PENDING' | 'PAID' | 'VOID' | 'UNCOLLECTIBLE';
  currency: string;
  subtotal_minor: number;
  tax_minor: number;
  total_minor: number;
  due_date?: string;
  created_at: string;
  lines?: LineItem[];
}

export interface CreateDraftInvoiceInput {
  account_id: number;
  currency: string;
  line_items: LineItem[];
}

export interface ChargeRequest {
  provider: string;
  invoice_id: number;
  amount_minor: number;
  currency: string;
}

export interface ChargeResult {
  provider_tx_id?: string;
  checkout_url?: string;
  status: string;
  error?: string;
}

export interface ApiLogEntry {
  id: string;
  timestamp: string;
  method: string;
  url: string;
  status?: number;
  statusText?: string;
  durationMs: number;
  requestBody?: any;
  responseBody?: any;
  isError: boolean;
}
