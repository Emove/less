package transport

// TransportServer is an abstraction for net on different platform
type TransportServer interface {
	Serv(network, addr string) error
	Stop()
}
