package rawpath

// Spellings that reach a write endpoint once the server decodes and cleans the path (M1). Violations
// (g0-path) ×8: repeated slash, dot segment, percent-encoded letter, relative path, parent segment,
// encoded slashes, double encoding, and a relative MF path.
var Bypass = []string{
	"//orders/regular",
	"/./orders/regular",
	"/%6Frders/regular",
	"orders/regular",
	"/quote/../gtt/triggers",
	"%2Falerts%2Fx",
	"/%256Frders/regular",
	"mf/orders",
}
