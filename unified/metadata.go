package unified

type ContinuationMode string

const (
	ContinuationReplay             ContinuationMode = "replay"
	ContinuationPreviousResponseID ContinuationMode = "previous_response_id"
	ContinuationProviderSession    ContinuationMode = "provider_session"
)

type TransportKind string

const (
	TransportHTTP      TransportKind = "http"
	TransportHTTPSSE   TransportKind = "http_sse"
	TransportWebSocket TransportKind = "websocket"
)

type InteractionMode string

const (
	InteractionAuto    InteractionMode = "auto"
	InteractionOneShot InteractionMode = "one_shot"
	InteractionSession InteractionMode = "session"
)

func ValidInteractionMode(mode InteractionMode) bool {
	switch mode {
	case InteractionAuto, InteractionOneShot, InteractionSession:
		return true
	default:
		return false
	}
}
