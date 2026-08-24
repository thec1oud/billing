-- name: InsertDefinition :one
INSERT INTO sm_definitions (
    machine_type,
    version,
    initial_state,
    is_active
) VALUES (
    $1, $2, $3, $4
)
RETURNING definition_id, created_at;

-- name: DeactivateDefinitionsForMachineType :exec
UPDATE sm_definitions
SET is_active = false
WHERE machine_type = $1 AND is_active;

-- name: GetActiveDefinition :one
SELECT definition_id, machine_type, version, initial_state, is_active, created_at
FROM sm_definitions
WHERE machine_type = $1 AND is_active;

-- name: GetDefinitionByID :one
SELECT definition_id, machine_type, version, initial_state, is_active, created_at
FROM sm_definitions
WHERE definition_id = $1;

-- name: InsertState :exec
INSERT INTO sm_states (
    definition_id,
    state_name,
    is_final,
    metadata
) VALUES (
    $1, $2, $3, $4
);

-- name: ListStatesByDefinition :many
SELECT definition_id, state_name, is_final, metadata
FROM sm_states
WHERE definition_id = $1;

-- name: InsertTransition :one
INSERT INTO sm_transitions (
    definition_id,
    from_state,
    event_name,
    to_state,
    priority,
    emit_event_type
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING transition_id;

-- name: ListTransitionsByDefinition :many
SELECT transition_id, definition_id, from_state, event_name, to_state, priority, emit_event_type
FROM sm_transitions
WHERE definition_id = $1;

-- name: InsertGuardBinding :exec
INSERT INTO sm_guard_bindings (
    transition_id,
    seq,
    implementation_kind,
    guard_name,
    script,
    params
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: ListGuardBindingsByDefinition :many
SELECT gb.transition_id, gb.seq, gb.implementation_kind, gb.guard_name, gb.script, gb.params
FROM sm_guard_bindings gb
JOIN sm_transitions t ON t.transition_id = gb.transition_id
WHERE t.definition_id = $1;

-- name: InsertActionBinding :one
INSERT INTO sm_action_bindings (
    definition_id,
    hook_type,
    state_name,
    transition_id,
    seq,
    action_name,
    params_kind,
    params,
    params_script,
    mode,
    on_error
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING binding_id;

-- name: ListActionBindingsByDefinition :many
SELECT binding_id, definition_id, hook_type, state_name, transition_id, seq, action_name,
       params_kind, params, params_script, mode, on_error
FROM sm_action_bindings
WHERE definition_id = $1;

-- name: CreateInstance :one
INSERT INTO sm_instances (
    definition_id,
    machine_type,
    subject_type,
    subject_id,
    current_state,
    context
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING instance_id, version, status, created_at, updated_at;

-- name: GetInstanceForUpdate :one
SELECT instance_id, definition_id, machine_type, subject_type, subject_id, current_state,
       context, version, status, created_at, updated_at
FROM sm_instances
WHERE instance_id = $1
FOR UPDATE;

-- name: GetInstanceByID :one
SELECT instance_id, definition_id, machine_type, subject_type, subject_id, current_state,
       context, version, status, created_at, updated_at
FROM sm_instances
WHERE instance_id = $1;

-- name: GetInstanceBySubject :one
SELECT instance_id, definition_id, machine_type, subject_type, subject_id, current_state,
       context, version, status, created_at, updated_at
FROM sm_instances
WHERE subject_type = $1 AND subject_id = $2 AND machine_type = $3;

-- name: UpdateInstanceState :execrows
UPDATE sm_instances
SET current_state = $3,
    context = $4,
    status = $5,
    version = version + 1,
    updated_at = now()
WHERE instance_id = $1 AND version = $2;

-- name: InsertTransitionHistoryPending :one
INSERT INTO sm_transition_history (
    instance_id,
    transition_id,
    from_state,
    to_state,
    event_name,
    event_payload,
    context_before,
    triggered_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING history_id, occurred_at;

-- name: FinalizeTransitionHistory :exec
UPDATE sm_transition_history
SET context_after = $2
WHERE history_id = $1;

-- name: InsertOutboxRow :exec
INSERT INTO sm_action_outbox (
    instance_id,
    history_id,
    binding_id,
    action_name,
    params
) VALUES (
    $1, $2, $3, $4, $5
);

-- name: ClaimOutboxRows :many
WITH candidates AS (
    SELECT outbox_id
    FROM sm_action_outbox
    WHERE status = 'PENDING'
    ORDER BY created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE sm_action_outbox o
SET status = 'PROCESSING',
    claimed_at = now(),
    claimed_by = $2,
    attempts = attempts + 1
FROM candidates c
WHERE o.outbox_id = c.outbox_id
RETURNING o.outbox_id, o.instance_id, o.history_id, o.binding_id, o.action_name, o.params,
          o.status, o.attempts, o.claimed_at, o.claimed_by, o.last_error, o.created_at;

-- name: MarkOutboxPublished :exec
UPDATE sm_action_outbox
SET status = 'PUBLISHED'
WHERE outbox_id = $1;

-- name: MarkOutboxRetry :exec
UPDATE sm_action_outbox
SET status = 'PENDING',
    last_error = $2
WHERE outbox_id = $1;

-- name: MarkOutboxFailed :exec
UPDATE sm_action_outbox
SET status = 'FAILED',
    last_error = $2
WHERE outbox_id = $1;

-- name: ReapStuckOutbox :execrows
UPDATE sm_action_outbox
SET status = 'PENDING'
WHERE status = 'PROCESSING' AND claimed_at < $1;

-- name: InsertScheduledTransition :one
INSERT INTO sm_scheduled_transitions (
    instance_id,
    event_name,
    event_payload,
    fire_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING schedule_id;

-- name: ClaimScheduledTransitions :many
WITH candidates AS (
    SELECT schedule_id
    FROM sm_scheduled_transitions
    WHERE status = 'PENDING' AND fire_at <= now()
    ORDER BY fire_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE sm_scheduled_transitions s
SET status = 'PROCESSING',
    claimed_at = now(),
    claimed_by = $2,
    attempts = attempts + 1
FROM candidates c
WHERE s.schedule_id = c.schedule_id
RETURNING s.schedule_id, s.instance_id, s.event_name, s.event_payload, s.fire_at, s.status,
          s.attempts, s.claimed_at, s.claimed_by, s.last_error, s.created_at;

-- name: MarkScheduleFired :exec
UPDATE sm_scheduled_transitions
SET status = 'FIRED'
WHERE schedule_id = $1;

-- name: MarkScheduleRetry :exec
UPDATE sm_scheduled_transitions
SET status = 'PENDING',
    last_error = $2
WHERE schedule_id = $1;

-- name: MarkScheduleFailed :exec
UPDATE sm_scheduled_transitions
SET status = 'FAILED',
    last_error = $2
WHERE schedule_id = $1;

-- name: ReapStuckSchedules :execrows
UPDATE sm_scheduled_transitions
SET status = 'PENDING'
WHERE status = 'PROCESSING' AND claimed_at < $1;
