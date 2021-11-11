package services

type ConnectionState int

const (
	Disconnected ConnectionState = iota
	Connecting
	Connected
)

type processorState int

const (
	Stopped processorState = iota
	Paused
	Running
)
