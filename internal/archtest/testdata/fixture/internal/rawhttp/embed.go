package rawhttp

import kc "github.com/zerodha/gokiteconnect/v4"

// wrapped embeds the banned interface. Violations (g0-kitehttp): the embedded HTTPClient and the
// promoted Do.
type wrapped struct{ kc.HTTPClient }

func ViaEmbedded(w wrapped) error {
	_, err := w.Do("POST", "x")
	return err
}
