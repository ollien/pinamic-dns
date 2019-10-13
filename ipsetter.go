package pinamicdns

import "net"

// IPSetter associates the given ip with the given domain and subdomain name.
// An example of such an association would be the setting of a DNS entry.
type IPSetter interface {
	// SetIP associates the given ip with the given domain and subdomain name.
	// If a record already exists for the given subdomain name, only one record that does not have the same IP address will be updated.
	// If all records have the same IP address, no updating will be performed.
	SetIP(domain, name string, ip net.IP) error
}
