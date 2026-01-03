package main

import (
	xlog "xlog"
)

func main() {
	_, cleanup := xlog.MustInitFromEnv()
	defer cleanup()

	xlog.S().Infow("hello logging",
		"module", "example",
		"feature", "xlog",
	)
	xlog.L().Debug("debug message (mungkin tersembunyi jika level info)")
}


