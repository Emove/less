package channel

const (
	Client = 1
	Server = 2
)

// Sider is used to get current side, client or server
type Sider interface {
	Side() int
}
