package unified

type CacheControlType string

const (
	CacheControlEphemeral CacheControlType = "ephemeral"
)

type CacheControl struct {
	Type CacheControlType `json:"type"`
	TTL  string           `json:"ttl,omitempty"`
}

func EphemeralCache(ttl string) *CacheControl {
	return &CacheControl{Type: CacheControlEphemeral, TTL: ttl}
}
