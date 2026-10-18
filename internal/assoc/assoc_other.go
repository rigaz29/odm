//go:build !windows

package assoc

import "errors"

const supported = false

var errUnsupported = errors.New("file associations are only registered on Windows")

type hive struct{}

var std hive

func (hive) set(string) error { return errUnsupported }
func (hive) unset() error     { return nil }
func (hive) exe() string      { return "" }

func openSettings() error { return errUnsupported }
