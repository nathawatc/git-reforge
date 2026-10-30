package ui

import "os"

var (
	Red    = "\033[1;31m"
	Green  = "\033[1;32m"
	Yellow = "\033[1;33m"
	Blue   = "\033[0;34m"
	Cyan   = "\033[0;36m"
	Bold   = "\033[1m"
	NC     = "\033[0m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		DisableColors()
	}
}

func DisableColors() {
	Red = ""
	Green = ""
	Yellow = ""
	Blue = ""
	Cyan = ""
	Bold = ""
	NC = ""
}
