package mantleauth

import (
	"context"
	"fmt"

	"github.com/codewandler/llmadapter/transport"
)

type CredentialTransport struct {
	Inner                    transport.ByteStreamTransport
	StaticToken              string
	TokenProvider            TokenProvider
	MissingCredentialMessage string
}

func (t *CredentialTransport) Open(ctx context.Context, req *transport.Request) (transport.ByteStream, error) {
	token := t.StaticToken
	if token == "" {
		if t.TokenProvider == nil {
			if t.MissingCredentialMessage != "" {
				return nil, fmt.Errorf("%s", t.MissingCredentialMessage)
			}
			return nil, fmt.Errorf("bedrock credentials are not configured")
		}
		var err error
		token, err = t.TokenProvider.Token(ctx)
		if err != nil {
			return nil, err
		}
	}
	cloned := *req
	cloned.Header = req.Header.Clone()
	cloned.Header.Set("Authorization", "Bearer "+token)
	inner := t.Inner
	if inner == nil {
		inner = transport.NewDefaultRetryTransport(transport.NewHTTPByteStreamTransport(transport.HTTPTransportConfig{FrameFormat: transport.FrameFormatSSE}))
	}
	return inner.Open(ctx, &cloned)
}
