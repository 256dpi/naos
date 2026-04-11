package mdns

import (
	"context"
	"time"

	"github.com/grandcat/zeroconf"
)

// Descriptor represents a discovered device.
type Descriptor struct {
	Hostname string
	Address  string
}

// Discover searches for devices with the _naos._tcp service type and calls the
// provided callback for each discovered device until the stop channel is closed.
func Discover(stop chan struct{}, cb func(Descriptor)) error {
	for {
		// collect devices
		list, err := Collect(5 * time.Second)
		if err != nil {
			return err
		}

		// yield devices
		for _, d := range list {
			go cb(d)
		}

		// check stop
		select {
		case <-stop:
			return nil
		default:
		}
	}
}

// Collect searches for devices with the _naos._tcp service type for the
// specified duration and returns the results.
func Collect(duration time.Duration) ([]Descriptor, error) {
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
	var addresses []Descriptor
	go func() {
		for entry := range entries {
			if len(entry.AddrIPv4) == 0 {
				continue
			}
			addresses = append(addresses, Descriptor{
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
