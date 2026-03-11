// Package bridge provides the Veriqid Bridge API (Phase 2).
// This HTTP API wraps the CGO crypto library for browser/extension communication.
//
// Planned endpoints:
//   POST /api/identity/create    - Generate msk, register on-chain
//   POST /api/identity/register  - Generate membership proof + spk
//   POST /api/identity/auth      - Generate auth proof
//   GET  /api/identity/challenge  - Generate random challenge
package bridge

// TODO: Phase 2 implementation
