// Shared Types
export interface Money {
  amount_minor: number;
  currency: string;
}

// ----------------------
// Account Types
// ----------------------
export type AccountStatus = "PENDING_VERIFICATION" | "ACTIVE" | "SUSPENDED" | "CLOSED";

export interface Account {
  id: number;
  external_id: string;
  status: AccountStatus;
  currency: string;
  timezone: string;
  locale: string;
  net_terms: number;
  dunning_profile_id?: number | null;
  tax_identifiers: any;
  billing_address: any;
  compliance_flags: any;
  metadata: any;
  payment_methods: string[] | null;
  created_at: string; // ISO 8601 Date string
}

export interface CreateAccountInput {
  external_id?: string;
  currency: string;
  timezone?: string;
  locale?: string;
  net_terms: number;
  dunning_profile_id?: number | null;
  tax_identifiers?: any;
  billing_address?: any;
  compliance_flags?: any;
  metadata?: any;
}

// ----------------------
// Tariff Types
// ----------------------
export type TariffTypeCode = 
  | "FLAT_FEE" 
  | "PER_UNIT" 
  | "TIERED_USAGE" 
  | "STAIRSTEP" 
  | "PACKAGE" 
  | "MATRIX" 
  | "COMPOSITE";

export type TierStrategy = "VOLUME" | "GRADUATED";

export interface Tier {
  up_to?: number;
  unit_price: Money;
  flat_fee: Money;
}

export interface Tariff {
  tariff_id: number;
  tariff_code: string;
  version: number;
  name: string;
  description: string;
  tariff_type_code: TariffTypeCode;
  tier_strategy?: TierStrategy;
  amount: Money;
  tiers?: Tier[];
  metadata: any;
  created_at: string; // ISO 8601 Date string
  is_active: boolean;
}

export interface CreateTariffInput {
  code: string;
  name: string;
  description: string;
  tariff_type_code: TariffTypeCode;
  amount: Money;
  tiers?: Tier[];
  metadata?: any;
}

// ----------------------
// Plan Types
// ----------------------
export type LegacyPricePolicy = "KEEP_FOREVER" | "MIGRATE_IMMEDIATELY" | "MIGRATE_ON_RENEWAL";

export interface PlanDuration {
  plan_duration_id?: number;
  plan_id?: number;
  tariff_id: number;
  duration: number; // time.Duration (nanoseconds)
  is_active?: boolean;
  created_at?: string;
}

export interface Plan {
  plan_id?: number;
  plan_code: string;
  version?: number;
  effective_from?: string; // ISO 8601 Date string
  effective_until?: string | null;
  legacy_price_policy_code: LegacyPricePolicy;
  migration_path?: any;
  metadata?: any;
  created_at?: string;
  durations?: PlanDuration[];
}

export interface CreatePlanInput {
  plan_code: string;
  legacy_price_policy_code: LegacyPricePolicy;
  durations: PlanDuration[];
}

// ----------------------
// Subscription Types
// ----------------------
export type SubscriptionStatus = "ACTIVE";

export interface Subscription {
  id: number;
  version: number;
  account_id: number;
  plan_id: number;
  plan_version: number;
  status: SubscriptionStatus;
  quantity: number;
  current_period_start: string;
  current_period_end: string;
  billing_cycle_anchor: string;
  cancel_at_period_end: boolean;
}

export interface CreateSubscriptionInput {
  account_id: number;
  plan_id: number;
  plan_version: number;
  quantity?: number;
  current_period_start?: string;
  current_period_end?: string;
  billing_cycle_anchor?: string;
}

// ----------------------
// Invoice Types
// ----------------------
export type InvoiceStatus = "DRAFT" | "OPEN" | "PAID" | "UNCOLLECTIBLE" | "VOID";

export interface LineItem {
  line_item_id?: number;
  item_id: number;
  description: string;
  quantity_value: number;
  quantity_unit: string;
  unit_amount: Money;
  total_amount: Money;
  metadata?: any;
  subscription_id?: number;
}

export interface Invoice {
  invoice_id: number;
  account_id: number;
  invoice_number?: string;
  status: InvoiceStatus;
  currency: string;
  subtotal: Money;
  tax: Money;
  discount: Money;
  total: Money;
  amount_paid: Money;
  amount_due: Money;
  due_at?: string;
  finalized_at?: string;
  paid_at?: string;
  line_items: LineItem[];
}

export interface PayInvoiceInput {
  provider_code: string;
  idempotency_key: string;
}

export interface PayInvoiceResponse {
  checkout_url: string;
  internal_tx_id: string;
  provider_reference: string;
}

export interface DevGenerateInvoiceInput {
  account_id: number;
  plan_id: number;
}

export interface FakeWebhookPayload {
  event_id: string;
  event: string;
  tx_ref: string;
  reference: string;
  status: string;
  amount_minor: number;
  currency: string;
}

export interface APIErrorResponse {

  code: string;
  message: string;
}
