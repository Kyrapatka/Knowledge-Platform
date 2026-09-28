// Package interview manages questions, profiles and an account's concept graph.
// Handlers parse HTTP requests and call Service. Service owns profile readiness,
// validation and import reconciliation; Store owns SQL, ownership-scoped loading
// and transaction boundaries. Folder import joins its existing transaction so
// materials and their profiles are committed together.
//
// The graph subpackage selects roots and follow-ups from concepts and routing
// metadata. Mock plans distribute roots across available topics; deep mode
// explores a branch within configured depth and eligibility limits. These pure
// selectors do not own persistence or SRS updates: training coordinates those.
package interview
