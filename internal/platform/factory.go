package platform

// New returns the Platform implementation for the current OS.
func New() Platform {
	return newForOS()
}
