package app

import "golang.org/x/sys/unix"

// Names uses x/sys objects that share a name with unix.Exec: the field Attr.Exec and the method Proc.Exec.
func Names() error {
	var a unix.Attr
	_ = a.Exec
	_ = unix.Proc{}.Exec()
	return unix.Exec("", nil, nil)
}
