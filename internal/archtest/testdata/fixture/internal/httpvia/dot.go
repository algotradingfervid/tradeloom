package httpvia

import . "net/http"

func Dot() { _, _ = DefaultClient.Head("https://api.kite.trade/user/profile") }
