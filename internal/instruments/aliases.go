package instruments

// indexAliases maps an index tradingsymbol to the name its NFO/BFO futures use in the Kite dump,
// so a search for "BANKNIFTY" finds the NIFTY BANK index (FR-WATCH-008 AC2). Checked against a
// real dump in P0-2 Task 15 Step 5.
var indexAliases = map[string]string{
	"NIFTY 50":          "NIFTY",
	"NIFTY BANK":        "BANKNIFTY",
	"NIFTY FIN SERVICE": "FINNIFTY",
	"NIFTY MID SELECT":  "MIDCPNIFTY",
	"NIFTY NEXT 50":     "NIFTYNXT50",
	"SENSEX":            "SENSEX",
	"BANKEX":            "BANKEX",
}
