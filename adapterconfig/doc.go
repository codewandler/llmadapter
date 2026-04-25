// Package adapterconfig is the public construction boundary for llmadapter.
//
// It loads and validates JSON/env configuration, detects credentials for auto
// mux clients, builds provider endpoints through providerregistry, loads modeldb
// catalogs and overlays, applies capability/pricing metadata, and constructs
// routers or mux clients.
package adapterconfig
