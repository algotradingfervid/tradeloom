// Package mockx looks like internal/kite/mock but is not under it: no g0-path exemption.
package mockx

// PlaceOrder is a G-0 violation (g0-path).
const PlaceOrder = "/orders/regular"
