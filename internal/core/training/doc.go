// Package training contains learning plans, sessions and per-material progress.
// Long-term progress survives sessions; CRAM progress belongs to its plan and
// never changes the long-term track. Recovery and final review use the pure
// algorithm package; combined sessions coordinate existing plans across folders.
// Interview graph sessions support follow-ups, while practice-only mock
// interviews do not receive SRS review credit.
//
// The service facade retains commands that span session, progress, undo and
// receipt state. ExerciseService and ProgressService use narrower repositories.
// Commands hold the account row lock and commit their effects and receipt in one
// transaction. A retry cannot apply an action twice, stale presentations are
// rejected, and undo restores state while keeping versions monotonic. Analytics
// publish only after commit. HTTP, GORM and SQL never enter the algorithm package.
package training
