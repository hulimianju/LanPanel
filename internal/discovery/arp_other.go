//go:build !linux && !darwin

package discovery

func readARP() map[string]string { return map[string]string{} }
func defaultGateway() string     { return "" }
