// Package service may import syscall (it is on the g0-exec import allow-list, Ruling 83): clean.
package service

import "syscall"

// NonBlock is the open flag the real internal/service uses for the calendar import.
const NonBlock = syscall.O_NONBLOCK
