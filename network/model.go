package network

import (
	"net"

	"github.com/vishvananda/netlink"
)

type Network struct {
	Name    string     // 网络名
	IPRange *net.IPNet // 地址段
	Driver  string     // 驱动名
}

type Endpoint struct {
	ID          string           `json:"id"`
	Device      netlink.Veth     `json:"dev"`
	IPAddress   net.IP           `json:"ip"`
	MacAddress  net.HardwareAddr `json:"mac"`
	Network     *Network
	PortMapping []string
}

type Driver interface {
	Name() string
	Create(subnet string, name string) (*Network, error)
	Delete(network *Network) error
	Connect(networkName string, endpoint *Endpoint) error // 内部会修改 endpoint.Device，必修传指针
	Disconnect(endpointID string) error
}

type IPAMer interface {
	// 从指定的 subnet 网段中分配 IP 地址
	Allocate(subnet *net.IPNet) (ip net.IP, err error)
	// 从指定的 subnet 网段中释放指定的 IP 地址
	Release(subnet *net.IPNet, ip *net.IP) error
}
