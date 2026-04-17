// Package dashboard owns the typed dashboard runtime, page contract, and
// transport boundary for go-dashboard.
//
// The canonical presentation contract is Page. Legacy payload-map helpers such
// as Controller.LayoutPayload are temporary migration adapters and should not be
// treated as the long-term rendering or transport boundary.
package dashboard
