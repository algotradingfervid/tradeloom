package execuser

import "plugin" // g0-plugin: loading code at run time is banned everywhere (Ruling 83)

var _ = plugin.Open
