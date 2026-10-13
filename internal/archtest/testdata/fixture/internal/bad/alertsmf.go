package bad

import kiteconnect "github.com/zerodha/gokiteconnect/v4"

// AlertsAndMF is a G-0 violation for every alert/MF write (each selector is flagged); GetAlerts is read-only.
func AlertsAndMF(c *kiteconnect.Client) {
	c.CreateAlert(kiteconnect.AlertParams{})
	c.ModifyAlert("u", kiteconnect.AlertParams{})
	_ = c.DeleteAlerts("u")
	c.PlaceMFOrder(kiteconnect.OrderParams{})
	c.CancelMFOrder("o")
	c.PlaceMFSIP(kiteconnect.OrderParams{})
	c.ModifyMFSIP("s", kiteconnect.OrderParams{})
	c.CancelMFSIP("s")
	c.InitiateHoldingsAuth(kiteconnect.OrderParams{})
	c.GetAlerts()
}
