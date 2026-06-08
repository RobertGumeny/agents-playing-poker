// Package match orchestrates a single match: it manages agent child processes,
// drives the server-authoritative hand loop over the JSONL wire protocol, and
// writes the session artifacts.
//
// The wire protocol lives in docs/wire-protocol.md; artifact schemas in
// docs/session-artifacts.md.
package match
