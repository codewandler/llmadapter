// Package muxclient exposes a stateless unified.Client that routes each request
// through a router.ProviderEndpoint set.
//
// It is the in-process equivalent of the HTTP gateway route path: select a
// compatible route, rewrite to the provider-native model, call the provider
// client, and optionally fall back before streaming starts.
package muxclient
