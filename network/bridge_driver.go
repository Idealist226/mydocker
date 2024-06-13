package network

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

type BridgeNetworkDriver struct {
}

func (d *BridgeNetworkDriver) Name() string {
	return "bridge"
}

/* 创建 Bridge 网络 */
func (d *BridgeNetworkDriver) Create(subnet string, name string) (*Network, error) {
	ip, ipRange, _ := net.ParseCIDR(subnet)
	ipRange.IP = ip
	n := &Network{
		Name:    name,
		IPRange: ipRange,
		Driver:  d.Name(),
	}
	err := d.initBridge(n)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create bridge network")
	}
	return n, err
}

/* 删除 Bridge 网络 */
func (d *BridgeNetworkDriver) Delete(name string) error {
	// 根据名字找到对应的 Bridge 设备
	br, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	// 删除网络对应的 Linux Bridge 设备
	return netlink.LinkDel(br)
}

/* 连接一个网络和网络端点 */
func (d *BridgeNetworkDriver) Connect(networkName string, ep *Endpoint) error {
	bridgeName := networkName
	// 通过接口名获取到 Linux Bridge 接口的对象和接口属性
	br, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return err
	}
	// 创建 Veth 接口的配置
	la := netlink.NewLinkAttrs()
	// 由于 Linux 接口名的限制,取 endpointID 的前 5 位
	la.Name = ep.ID[:5]
	// 通过设置 Veth 接口 master 属性，设置这个 Veth 的一端挂载到网络对应的 Linux Bridge
	la.MasterIndex = br.Attrs().Index
	// 创建 Veth 对象，通过 PeerNarne 配置 Veth 另外一端的接口名
	// 配置 Veth 另外一端的名字 cif-{endpoint ID 的前 5 位｝
	ep.Device = netlink.Veth{
		LinkAttrs: la,
		PeerName:  "cif-" + la.Name,
	}
	// 调用 netlink 的 LinkAdd 方法创建出这个 Veth 接口
	// 因为上面指定了 link 的 MasterIndex是 网络对应的 Linux Bridge
	// 所以 Veth 的一端就已经挂载到了网络对应的 Linux Bridge 上
	if err = netlink.LinkAdd(&ep.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	// 调用 netlink 的 LinkSetUp 方法，设置 Veth 启动
	// 相当于 ip link set xxx up 命令
	if err = netlink.LinkSetUp(&ep.Device); err != nil {
		return fmt.Errorf("error Add Endpoint Device: %v", err)
	}
	return nil
}

/* 断开一个网络和网络端点 */
func (d *BridgeNetworkDriver) Disconnect(endpointID string) error {
	// 根据名字找到对应的 Veth 设备
	vethNme := endpointID[:5] // 由于 Linux 接口名的限制,取 endpointID 的前 5 位
	veth, err := netlink.LinkByName(vethNme)
	if err != nil {
		return err
	}
	// 从网桥解绑
	err = netlink.LinkSetNoMaster(veth)
	if err != nil {
		return errors.WithMessagef(err, "find veth [%s] failed", vethNme)
	}
	// 删除 veth-pair
	// 一端为 xxx,另一端为 cif-xxx
	err = netlink.LinkDel(veth)
	if err != nil {
		return errors.WithMessagef(err, "delete veth [%s] failed", vethNme)
	}
	veth2Name := "cif-" + vethNme
	veth2, err := netlink.LinkByName(veth2Name)
	if err != nil {
		return errors.WithMessagef(err, "find veth [%s] failed", veth2Name)
	}
	err = netlink.LinkDel(veth2)
	if err != nil {
		return errors.WithMessagef(err, "delete veth [%s] failed", veth2Name)
	}
	return nil
}

/* 初始化 Bridge 网络 */
func (d *BridgeNetworkDriver) initBridge(n *Network) error {
	bridgeName := n.Name
	// 1) 创建 Bridge 虚拟设备
	if err := createBridgeInterface(bridgeName); err != nil {
		return errors.Wrapf(err, "Failed to create bridge %s", bridgeName)
	}

	// 2) 设置 Bridge 设备地址和路由
	gatewayIP := *n.IPRange
	if err := setInterfaceIP(bridgeName, gatewayIP.String()); err != nil {
		return errors.Wrapf(err, "Error set bridge ip: %s on bridge: %s", gatewayIP.String(), bridgeName)
	}

	// 3) 启动 Bridge 设备
	if err := setInterfaceUP(bridgeName); err != nil {
		return errors.Wrapf(err, "Error set bridge up: %s", bridgeName)
	}

	// 4) 设置 iptables SNAT 规则
	if err := setupIPTables(bridgeName, n.IPRange); err != nil {
		return errors.Wrapf(err, "Error setting iptables for %s", bridgeName)
	}

	return nil
}

func (d *BridgeNetworkDriver) deleteBridge(n *Network) error {
	bridgeName := n.Name

	// get the link
	l, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("getting link with name %s failed: %v", bridgeName, err)
	}

	// delete the link
	if err = netlink.LinkDel(l); err != nil {
		return fmt.Errorf("failed to remove bridge interface %s delete: %v", bridgeName, err)
	}

	return nil
}

/* 创建 Bridge 设备 */
func createBridgeInterface(bridgeName string) error {
	// 先检查是否已经存了这个同名的 Bridge 设备
	_, err := net.InterfaceByName(bridgeName)
	// 如果已经存在或者报错则返回创建错误
	// errNoSuchInterface 这个错误未导出也没提供判断方法，只能判断字符串了。。
	if err == nil || !strings.Contains(err.Error(), "no such network interface") {
		return err
	}

	// 初始化一个 netlink 的 Link 基础对象，Link 的名字即 Bridge 虚拟设备的名字
	la := netlink.NewLinkAttrs()
	la.Name = bridgeName
	// 使用刚才创建的 link 的属性创建 netlink 的 Bridge 对象
	br := &netlink.Bridge{LinkAttrs: la}
	// 调用 net link LinkAdd 方法，创建 Bridge 虚拟网络设备
	// netlink.LinkAdd 方法是用来创建虚拟网络设备的，相当于 ip link add xxxx
	if err = netlink.LinkAdd(br); err != nil {
		return errors.Wrapf(err, "Error add bridge device: %s", bridgeName)
	}
	return nil
}

/* 设置 Bridge 设备地址和路由 */
func setInterfaceIP(name string, rawIP string) error {
	retries := 2
	var iface netlink.Link
	var err error
	for i := 0; i < retries; i++ {
		// 通过 LinkByName 方法找到需要设置的网络接口
		iface, err = netlink.LinkByName(name)
		if err == nil {
			break
		}
		log.Debugf("error retrieving new bridge netlink link [ %s ]... retrying", name)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return errors.Wrapf(err, "abandoning retrieving the new bridge link from netlink, Run [ ip link ] to troubleshoot")
	}
	// 由于 netlink.ParseIPNet 是对 net.ParseCIDR 的一个封装，因此可以将 net.PareCIDR 中返回的 IP 进行整合
	// 返回值中的 ipNet 既包含了网段的信息，192 168.0.0/24 ，也包含了原始的 IP 192.168.0.1
	ipNet, err := netlink.ParseIPNet(rawIP)
	if err != nil {
		return err
	}
	// 通过 netlink.AddrAdd 给网络接口配置地址，相当于 ip addr add xxx 命令
	// 同时如果配置了地址所在网段的信息，例如 192.168.0.0/24
	// 还会配置路由表 192.168.0.0/24 转发到这 testbridge 的网络接口上
	addr := &netlink.Addr{IPNet: ipNet}
	return netlink.AddrAdd(iface, addr)
}

/* 启动 Bridge 设备 */
func setInterfaceUP(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return errors.Wrapf(err, "error retrieving a link named [ %s ]:", iface.Attrs().Name)
	}
	// 通过 netlink.LinkSetUp 方法启动网络接口，相当于 ip link set xxx up
	if err = netlink.LinkSetUp(iface); err != nil {
		return errors.Wrapf(err, "nabling interface for %s", name)
	}
	return nil
}

/* 设置 iptables SNAT 规则 */
func setupIPTables(bridgeName string, subnet *net.IPNet) error {
	// 拼接命令
	iptablesCmd := fmt.Sprintf("-t nat -A POSTROUTING -s %s ! -o %s -j MASQUERADE", subnet.String(), bridgeName)
	cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
	log.Infof("配置 SNAT cmd: %v", cmd.String())
	// 执行该命令
	output, err := cmd.Output()
	if err != nil {
		log.Errorf("iptables Output, %v", output)
	}
	return err
}
