module fixture

go 1.25

require (
	github.com/zerodha/gokiteconnect/v4 v4.0.0
	golang.org/x/sys v0.0.0
)

replace github.com/zerodha/gokiteconnect/v4 => ./fakekite

replace golang.org/x/sys => ./fakesys
