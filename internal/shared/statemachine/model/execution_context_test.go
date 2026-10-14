package model

import (
	"encoding/json"
	"testing"
)

func TestExecutionContextMergeContextPreservesExistingValues(t *testing.T) {
	ec := ExecutionContext{Context: json.RawMessage(`{"account_id":9007199254740993,"status":"PENDING"}`)}
	if err := ec.MergeContext(map[string]any{"status": "ACTIVE", "name": "Ada"}); err != nil {
		t.Fatalf("merge context: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(ec.Context, &got); err != nil {
		t.Fatalf("unmarshal context: %v", err)
	}
	if string(got["account_id"]) != "9007199254740993" {
		t.Fatalf("expected existing large integer to remain exact, got %s", got["account_id"])
	}
	var status, name string
	if err := json.Unmarshal(got["status"], &status); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got["name"], &name); err != nil {
		t.Fatal(err)
	}
	if status != "ACTIVE" || name != "Ada" {
		t.Fatalf("unexpected merged values: status=%q name=%q", status, name)
	}
}

func TestExecutionContextSubjectIDInt64(t *testing.T) {
	ec := ExecutionContext{SubjectID: "9223372036854775807"}
	got, err := ec.SubjectIDInt64()
	if err != nil || got != 9223372036854775807 {
		t.Fatalf("SubjectIDInt64() = %d, %v", got, err)
	}
	ec.SubjectID = "42junk"
	if _, err := ec.SubjectIDInt64(); err == nil {
		t.Fatal("expected malformed subject ID to fail")
	}
}
