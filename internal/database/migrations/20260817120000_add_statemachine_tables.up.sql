-- ==========================================
-- Dynamic state machine engine schema
-- ==========================================

CREATE TABLE sm_definitions (
    definition_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    machine_type VARCHAR(64) NOT NULL,
    version INT NOT NULL,
    initial_state VARCHAR(64) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (machine_type, version)
);

-- At most one active (publishable-for-new-instances) version per machine type.
CREATE UNIQUE INDEX idx_sm_definitions_active_per_type
    ON sm_definitions (machine_type)
    WHERE is_active;

CREATE TABLE sm_states (
    definition_id UUID NOT NULL REFERENCES sm_definitions (definition_id) ON DELETE CASCADE,
    state_name VARCHAR(64) NOT NULL,
    is_final BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (definition_id, state_name)
);

CREATE TABLE sm_transitions (
    transition_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id UUID NOT NULL REFERENCES sm_definitions (definition_id) ON DELETE CASCADE,
    from_state VARCHAR(64) NOT NULL,
    event_name VARCHAR(64) NOT NULL,
    to_state VARCHAR(64) NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    emit_event_type VARCHAR(128),
    FOREIGN KEY (definition_id, from_state) REFERENCES sm_states (definition_id, state_name),
    FOREIGN KEY (definition_id, to_state) REFERENCES sm_states (definition_id, state_name),
    UNIQUE (definition_id, from_state, event_name, priority)
);

-- Engine hot path: find candidate transitions for (definition, current_state, event), priority-ordered.
CREATE INDEX idx_sm_transitions_lookup
    ON sm_transitions (definition_id, from_state, event_name, priority);

CREATE TABLE sm_guard_bindings (
    transition_id UUID NOT NULL REFERENCES sm_transitions (transition_id) ON DELETE CASCADE,
    seq INT NOT NULL,
    implementation_kind VARCHAR(12) NOT NULL DEFAULT 'GO_REGISTRY'
        CHECK (implementation_kind IN ('GO_REGISTRY', 'SCRIPT')),
    guard_name VARCHAR(128),
    script TEXT,
    params JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (transition_id, seq),
    CHECK (
        (implementation_kind = 'GO_REGISTRY' AND guard_name IS NOT NULL AND script IS NULL)
        OR
        (implementation_kind = 'SCRIPT' AND script IS NOT NULL AND guard_name IS NULL)
    )
);

CREATE TABLE sm_action_bindings (
    binding_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id UUID NOT NULL REFERENCES sm_definitions (definition_id) ON DELETE CASCADE,
    hook_type VARCHAR(20) NOT NULL CHECK (hook_type IN ('ON_ENTER', 'ON_EXIT', 'ON_TRANSITION')),
    state_name VARCHAR(64),
    transition_id UUID REFERENCES sm_transitions (transition_id) ON DELETE CASCADE,
    seq INT NOT NULL,
    action_name VARCHAR(128) NOT NULL,
    params_kind VARCHAR(12) NOT NULL DEFAULT 'STATIC' CHECK (params_kind IN ('STATIC', 'SCRIPT')),
    params JSONB,
    params_script TEXT,
    mode VARCHAR(10) NOT NULL CHECK (mode IN ('SYNC', 'ASYNC')),
    on_error VARCHAR(10) NOT NULL DEFAULT 'ABORT' CHECK (on_error IN ('ABORT', 'CONTINUE')),
    FOREIGN KEY (definition_id, state_name) REFERENCES sm_states (definition_id, state_name),
    CHECK (
        (hook_type IN ('ON_ENTER', 'ON_EXIT') AND state_name IS NOT NULL AND transition_id IS NULL)
        OR
        (hook_type = 'ON_TRANSITION' AND transition_id IS NOT NULL AND state_name IS NULL)
    ),
    CHECK (
        (params_kind = 'STATIC' AND params_script IS NULL)
        OR
        (params_kind = 'SCRIPT' AND params_script IS NOT NULL)
    )
);

CREATE INDEX idx_sm_action_bindings_state_hook
    ON sm_action_bindings (definition_id, state_name, hook_type, seq)
    WHERE state_name IS NOT NULL;

CREATE INDEX idx_sm_action_bindings_transition_hook
    ON sm_action_bindings (transition_id, hook_type, seq)
    WHERE transition_id IS NOT NULL;

CREATE TABLE sm_instances (
    instance_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id UUID NOT NULL REFERENCES sm_definitions (definition_id) ON DELETE RESTRICT,
    machine_type VARCHAR(64) NOT NULL,
    subject_type VARCHAR(64) NOT NULL,
    subject_id VARCHAR(128) NOT NULL,
    current_state VARCHAR(64) NOT NULL,
    context JSONB NOT NULL DEFAULT '{}',
    version BIGINT NOT NULL DEFAULT 0,
    status VARCHAR(12) NOT NULL DEFAULT 'RUNNING' CHECK (status IN ('RUNNING', 'COMPLETED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (subject_type, subject_id, machine_type),
    FOREIGN KEY (definition_id, current_state) REFERENCES sm_states (definition_id, state_name)
);

CREATE INDEX idx_sm_instances_subject ON sm_instances (subject_type, subject_id);

CREATE TABLE sm_transition_history (
    history_id BIGSERIAL PRIMARY KEY,
    instance_id UUID NOT NULL REFERENCES sm_instances (instance_id) ON DELETE CASCADE,
    transition_id UUID REFERENCES sm_transitions (transition_id),
    from_state VARCHAR(64),
    to_state VARCHAR(64) NOT NULL,
    event_name VARCHAR(64),
    event_payload JSONB,
    context_before JSONB,
    context_after JSONB,
    triggered_by VARCHAR(128),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sm_transition_history_instance ON sm_transition_history (instance_id, occurred_at);

CREATE TABLE sm_scheduled_transitions (
    schedule_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    instance_id UUID NOT NULL REFERENCES sm_instances (instance_id) ON DELETE CASCADE,
    event_name VARCHAR(64) NOT NULL,
    event_payload JSONB,
    fire_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(12) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'FIRED', 'CANCELED', 'FAILED')),
    attempts INT NOT NULL DEFAULT 0,
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(128),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sm_scheduled_transitions_pending
    ON sm_scheduled_transitions (fire_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_sm_scheduled_transitions_processing
    ON sm_scheduled_transitions (claimed_at)
    WHERE status = 'PROCESSING';

CREATE TABLE sm_action_outbox (
    outbox_id BIGSERIAL PRIMARY KEY,
    instance_id UUID NOT NULL REFERENCES sm_instances (instance_id) ON DELETE CASCADE,
    history_id BIGINT REFERENCES sm_transition_history (history_id),
    binding_id UUID REFERENCES sm_action_bindings (binding_id),
    action_name VARCHAR(128) NOT NULL,
    params JSONB NOT NULL,
    status VARCHAR(12) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'PROCESSING', 'PUBLISHED', 'FAILED')),
    attempts INT NOT NULL DEFAULT 0,
    claimed_at TIMESTAMPTZ,
    claimed_by VARCHAR(128),
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sm_action_outbox_pending
    ON sm_action_outbox (created_at)
    WHERE status = 'PENDING';

CREATE INDEX idx_sm_action_outbox_processing
    ON sm_action_outbox (claimed_at)
    WHERE status = 'PROCESSING';
