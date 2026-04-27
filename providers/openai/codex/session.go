package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
)

type continuationManager struct {
	mu     sync.Mutex
	states map[continuationKey]continuationState
}

type continuationKey struct {
	session string
	branch  string
}

type continuationState struct {
	model            string
	instructionHash  string
	lastInputLen     int
	lastInputHashes  []string
	lastFullBodyHash string
	lastResponseID   string
	turnState        string
}

type continuationDecision struct {
	previousResponseID string
	inputStart         int
	turnState          string
	commit             func(string)
}

func newContinuationManager() *continuationManager {
	return &continuationManager{states: map[continuationKey]continuationState{}}
}

func (m *continuationManager) prepare(sessionID, branchID string, body []byte) continuationDecision {
	if sessionID == "" || branchID == "" {
		return continuationDecision{}
	}
	fp, ok := continuationFingerprint(body)
	if !ok {
		return continuationDecision{}
	}
	key := continuationKey{session: sessionID, branch: branchID}

	m.mu.Lock()
	state := m.states[key]
	previousID := ""
	if state.lastResponseID != "" &&
		state.model == fp.model &&
		state.instructionHash == fp.instructionHash &&
		hasInputPrefix(fp.inputHashes, state.lastInputHashes) &&
		fp.fullBodyHash != state.lastFullBodyHash {
		previousID = state.lastResponseID
	}
	m.mu.Unlock()

	return continuationDecision{
		previousResponseID: previousID,
		inputStart:         state.lastInputLen,
		turnState:          state.turnState,
		commit: func(responseID string) {
			m.commitResponse(key, fp, responseID, "")
		},
	}
}

func (m *continuationManager) commitResponse(key continuationKey, fp continuationFingerprintValue, responseID, turnState string) {
	if responseID == "" {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.states[key]
	if turnState == "" {
		turnState = state.turnState
	}
	m.states[key] = continuationState{
		model:            fp.model,
		instructionHash:  fp.instructionHash,
		lastInputLen:     fp.inputLen,
		lastInputHashes:  append([]string(nil), fp.inputHashes...),
		lastFullBodyHash: fp.fullBodyHash,
		lastResponseID:   responseID,
		turnState:        turnState,
	}
}

func (m *continuationManager) setTurnState(sessionID, branchID, turnState string) {
	if sessionID == "" || branchID == "" || turnState == "" {
		return
	}
	key := continuationKey{session: sessionID, branch: branchID}
	m.mu.Lock()
	state := m.states[key]
	state.turnState = turnState
	m.states[key] = state
	m.mu.Unlock()
}

func (m *continuationManager) invalidate(sessionID, branchID string) {
	if sessionID == "" || branchID == "" {
		return
	}
	m.mu.Lock()
	delete(m.states, continuationKey{session: sessionID, branch: branchID})
	m.mu.Unlock()
}

type continuationFingerprintValue struct {
	model           string
	instructionHash string
	inputLen        int
	inputHashes     []string
	fullBodyHash    string
}

func continuationFingerprint(body []byte) (continuationFingerprintValue, bool) {
	var payload struct {
		Model        string            `json:"model"`
		Instructions string            `json:"instructions"`
		Input        []json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return continuationFingerprintValue{}, false
	}
	if payload.Model == "" {
		return continuationFingerprintValue{}, false
	}
	return continuationFingerprintValue{
		model:           payload.Model,
		instructionHash: hashString(payload.Instructions),
		inputLen:        len(payload.Input),
		inputHashes:     hashJSONItems(payload.Input),
		fullBodyHash:    hashBytes(canonicalJSON(body)),
	}, true
}

func hashJSONItems(items []json.RawMessage) []string {
	out := make([]string, len(items))
	for i := range items {
		out[i] = hashBytes(canonicalJSON(items[i]))
	}
	return out
}

func hasInputPrefix(input, prefix []string) bool {
	if len(input) < len(prefix) {
		return false
	}
	for i := range prefix {
		if input[i] != prefix[i] {
			return false
		}
	}
	return true
}

func withPreviousResponseID(body []byte, previousResponseID string, inputStart int) []byte {
	if previousResponseID == "" {
		return body
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	payload["previous_response_id"] = previousResponseID
	if rawInput, ok := payload["input"].([]any); ok && inputStart > 0 && inputStart <= len(rawInput) {
		input := rawInput[inputStart:]
		for len(input) > 0 && inputItemRole(input[0]) == "assistant" {
			input = input[1:]
		}
		payload["input"] = input
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

func inputItemRole(item any) string {
	m, ok := item.(map[string]any)
	if !ok {
		return ""
	}
	role, _ := m["role"].(string)
	return role
}

func responseIDFromFrame(raw []byte) string {
	var payload struct {
		ResponseID string `json:"response_id"`
		Response   struct {
			ID string `json:"id"`
		} `json:"response"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if payload.Response.ID != "" {
		return payload.Response.ID
	}
	return payload.ResponseID
}

func hashString(value string) string {
	return hashBytes([]byte(value))
}

func hashBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(raw []byte) []byte {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return raw
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return raw
	}
	return encoded
}
