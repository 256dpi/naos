package nadm

// A Manager provides numerous services to interact with NADK devices.
type Manager struct {
	BrokerURL string
}

// NewManager creates and returns a new Manager.
func NewManager(brokerURL string) *Manager {
	return &Manager{
		BrokerURL: brokerURL,
	}
}
