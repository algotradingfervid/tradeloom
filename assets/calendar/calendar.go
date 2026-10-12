// Package calendardata embeds the shipped market calendar (FR-DATA-001).
package calendardata

import _ "embed"

// Shipped is the current year's NSE/BSE trading-holiday list.
//
//go:embed nse-bse-2026.csv
var Shipped []byte

// ShippedName is the file name recorded with the calendar version.
const ShippedName = "nse-bse-2026.csv"
