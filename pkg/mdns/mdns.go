package mdns

import (
	"context"
	"time"

	"github.com/grandcat/zeroconf"
)

// Location represents a discovered device.
type Location struct {
	Hostname string
	Address  string
}

// Discover searches for all devices with the _naos._tcp service type.
func Discover(duration time.Duration) ([]Location, error) {
	// create resolver
	resolver, err := zeroconf.NewResolver(zeroconf.SelectIPTraffic(zeroconf.IPv4))
	if err != nil {
		return nil, err
	}

	// prepare context
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// prepare channels
	done := make(chan struct{})
	entries := make(chan *zeroconf.ServiceEntry, 8)

	// collect addresses
	var addresses []Location
	go func() {
		for entry := range entries {
			addresses = append(addresses, Location{
				Hostname: entry.HostName,
				Address:  entry.AddrIPv4[0].String(),
			})
		}
		close(done)
	}()

	// perform lookup
	err = resolver.Browse(ctx, "_naos._tcp", "local.", entries)
	if err != nil {
		return nil, err
	}

	// wait for done
	<-done

	return addresses, nil
}
