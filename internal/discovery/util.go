package discovery

import (
	"os"

	"lanpanel/internal/oui"
)

func osHostname() (string, error) { return os.Hostname() }

func ouiLookup(mac string) string { return oui.Lookup(mac) }
