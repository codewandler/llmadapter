package unified

type CacheControlType string

const (
	CacheControlEphemeral CacheControlType = "ephemeral"
)

type CachePolicy string

const (
	CachePolicyAuto CachePolicy = "auto"
	CachePolicyOn   CachePolicy = "on"
	CachePolicyOff  CachePolicy = "off"
)

type CacheControl struct {
	Type CacheControlType `json:"type"`
	TTL  string           `json:"ttl,omitempty"`
}

func EphemeralCache(ttl string) *CacheControl {
	return &CacheControl{Type: CacheControlEphemeral, TTL: ttl}
}
