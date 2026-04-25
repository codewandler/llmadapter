// Package router provides deterministic provider-endpoint routing for canonical
// requests.
//
// Routes match source API, public model, requested capabilities, and optional
// dynamic model resolution. Compatible candidates are ranked by route weight,
// provider endpoint priority, source-family preference, and declaration order.
package router
