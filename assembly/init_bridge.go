package assembly

import "encoding/json"

// buildRegistrationEvent marshals topology fields from opts into the JSON
// payload sent to the sidecar as the first event after connect. The sidecar
// forwards this as a RegisterRequest to the gateway.
func buildRegistrationEvent(opts runtimeOptions) string {
	type event struct {
		EventType        string          `json:"event_type"`
		ParentAgentID    string          `json:"parent_agent_id,omitempty"`
		TeamID           string          `json:"team_id,omitempty"`
		DelegationReason string          `json:"delegation_reason,omitempty"`
		SpawnedByTool    string          `json:"spawned_by_tool,omitempty"`
		EnforcementMode  EnforcementMode `json:"enforcement_mode,omitempty"`
	}

	payload, err := json.Marshal(event{
		EventType:        "register",
		ParentAgentID:    opts.parentAgentID,
		TeamID:           opts.teamID,
		DelegationReason: opts.delegationReason,
		SpawnedByTool:    opts.spawnedByTool,
		EnforcementMode:  opts.enforcementMode,
	})
	if err != nil {
		return `{"event_type":"register"}`
	}
	return string(payload)
}

// eventTypeToolResult is the native event-channel tag for a governed tool call's
// outcome. It is the audit counterpart of the "register" tag above; the runtime
// keys its enrichment off the tag and carries the JSON body through as details.
const eventTypeToolResult = "tool_result"

// buildToolResultEvent marshals one governed call's outcome into the JSON
// payload [ffiGovernanceClient.RecordResult] ships to the runtime (AAASM-5750).
//
// Field names are snake_case to match the register payload above and the
// gateway's wire convention. Empty fields are omitted rather than sent as ""
// so a denied call — which carries no Result — produces a payload that says so
// by shape, and a reader is never left deciding whether "" meant empty or
// unset. The one field always present is event_type, so a payload that failed
// to marshal is still a well-formed event rather than a truncated one.
//
// There is deliberately no allowed/denied discriminator: [RecordRequest] does
// not carry one, and inventing it here would put a decision on the wire that
// the struct the caller passes cannot express. A denied call is recognisable by
// an empty result plus the denial reason in error, which is exactly what
// [AssemblyTool.recordOutcome] hands over.
func buildToolResultEvent(request RecordRequest) string {
	type event struct {
		EventType string `json:"event_type"`
		ToolName  string `json:"tool_name,omitempty"`
		TraceID   string `json:"trace_id,omitempty"`
		RunID     string `json:"run_id,omitempty"`
		Result    string `json:"result,omitempty"`
		Error     string `json:"error,omitempty"`
	}

	payload, err := json.Marshal(event{
		EventType: eventTypeToolResult,
		ToolName:  request.ToolName,
		TraceID:   request.TraceID,
		RunID:     request.RunID,
		Result:    request.Result,
		Error:     request.Error,
	})
	if err != nil {
		return `{"event_type":"tool_result"}`
	}
	return string(payload)
}
