package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	"mydocker/constant"
	"mydocker/container"
	"mydocker/utils"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

var (
	defaultNetworkPath = "/var/lib/mydocker/network/network/"
	drivers            = map[string]Driver{}
)

func init() {
	// 加载网络驱动
	var bridgeDriver = BridgeNetworkDriver{}
	drivers[bridgeDriver.Name()] = &bridgeDriver

	// 文件不存在则创建
	exist, err := utils.PathExists(defaultNetworkPath)
	if err != nil {
		log.Errorf("Fail to judge whether dir %s exists. %v", defaultNetworkPath, err)
		return
	}
	if !exist {
		if err = os.MkdirAll(defaultNetworkPath, constant.Perm0644); err != nil {
			log.Errorf("create %s failed,detail:%v", defaultNetworkPath, err)
			return
		}
	}
}

func (net *Network) dump(dumpPath string) error {
	// 检查保存的目录是否存在，不存在则创建
	exist, err := utils.PathExists(dumpPath)
	if err != nil {
		log.Errorf("Fail to judge whether dir %s exists. %v", dumpPath, err)
		return err
	}
	if !exist {
		if err = os.MkdirAll(dumpPath, constant.Perm0644); err != nil {
			return errors.Wrapf(err, "create network dump path %s failed", dumpPath)
		}
	}

	// 保存的文件名是网络的名字
	netPath := path.Join(dumpPath, net.Name)
	// 打开保存的文件用于写入,后面打开的模式参数分别是存在内容则清空、只写入、不存在则创建
	netFile, err := os.OpenFile(netPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, constant.Perm0644)
	if err != nil {
		return errors.Wrapf(err, "open file %s failed", dumpPath)
	}
	defer netFile.Close()

	netJson, err := json.Marshal(net)
	if err != nil {
		return errors.Wrapf(err, "Marshal %v failed", net)
	}

	_, err = netFile.Write(netJson)
	return errors.Wrapf(err, "write %s failed", netJson)
}

func (net *Network) remove(dumpPath string) error {
	// 检查网络对应的配置文件状态，如果文件己经不存在就直接返回
	fullPath := path.Join(dumpPath, net.Name)
	if _, err := os.Stat(fullPath); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	// 否则删除这个网络对应的配置文件
	return os.Remove(fullPath)
}

func (net *Network) load(dumpPath string) error {
	// 打开配置文件
	netConfigFile, err := os.Open(dumpPath)
	if err != nil {
		return err
	}
	defer netConfigFile.Close()
	// 从配置文件中读取网络 配置 json 符串
	netJson := make([]byte, 2000)
	n, err := netConfigFile.Read(netJson)
	if err != nil {
		return err
	}

	err = json.Unmarshal(netJson[:n], net)
	return errors.Wrapf(err, "unmarshal %s failed", netJson[:n])
}

/* 读取 defaultNetworkPath 目录下的 Network 信息存放到内存中，便于使用 */
func loadNetwork() (map[string]*Network, error) {
	networks := map[string]*Network{}

	// 检查网络配置目录中的所有文件，并执行第二个参数中的函数指针去处理目录下的每一个文件
	loadErr := filepath.Walk(defaultNetworkPath, func(netPath string, info os.FileInfo, err error) error {
		// 如果是目录则跳过
		if info.IsDir() {
			return nil
		}
		// 加载文件名为网络名
		_, netName := path.Split(netPath)
		net := &Network{
			Name: netName,
		}
		// 从文件中加载网络配置
		if err = net.load(netPath); err != nil {
			log.Errorf("load network %s failed, detail is %v", netName, err)
		}
		// 将网络的配置信息加入到 networks 字典中
		networks[netName] = net
		return nil
	})
	return networks, loadErr
}

/* 根据不同 driver 创建 Network */
func CreateNetwork(driver, subnet, name string) error {
	// 将网段的字符串转换成 net.IPNet 对象
	_, cidr, _ := net.ParseCIDR(subnet)
	// 通过 IPAM 分配网关 IP，获取到网段中第一个 IP 作为网关的 IP
	ip, err := ipAllocator.Allocate(cidr)
	if err != nil {
		return err
	}
	cidr.IP = ip
	// 调用指定的网络驱动创建网络，这里的 drivers 字典是各个网络驱动的实例字典
	// 通过调用网络驱动的 Create 方法创建网络，后面会以 Bridge 驱动为例介绍它的实现
	net, err := drivers[driver].Create(cidr.String(), name)
	if err != nil {
		return err
	}
	// 保存网络信息，将网络的信息保存在文件系统中，以便查询和在网络上连接网络端点
	return net.dump(defaultNetworkPath)
}

/* 打印出当前全部 Network 信息 */
func ListNetwork() {
	networks, err := loadNetwork()
	if err != nil {
		log.Errorf("load network from file failed,detail: %v", err)
		return
	}
	// 通过tabwriter库把信息打印到屏幕上
	w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
	fmt.Fprint(w, "NAME\tIpRange\tDriver\n")
	for _, net := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			net.Name,
			net.IPRange.String(),
			net.Driver,
		)
	}
	if err = w.Flush(); err != nil {
		log.Errorf("Flush error %v", err)
		return
	}
}

/* 删除指定 Network */
func DeleteNetwork(networkName string) error {
	// 从文件中加载网络信息
	networks, err := loadNetwork()
	if err != nil {
		return errors.WithMessage(err, "load network from file failed")
	}
	// 获取网络信息
	net, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no such network: %s", networkName)
	}
	// 调用 IPAM 的示例 ipAlloctor 释放网络网关的 IP
	if err = ipAllocator.Release(net.IPRange, &net.IPRange.IP); err != nil {
		return errors.Wrap(err, "remove Network gateway ip failed")
	}
	// 调用网络驱动的删除方法
	if err = drivers[net.Driver].Delete(net); err != nil {
		return errors.Wrap(err, "remove Network DriverError failed")
	}
	// 最后从网络的配直目录中删除该网络对应的配置文件
	return net.remove(defaultNetworkPath)
}

/* 连接容器到之前创建的网络 mydocker run -net testnet -p 8080:80 xxxx */
func Connect(networkName string, info *container.Info) (net.IP, error) {
	networks, err := loadNetwork()
	if err != nil {
		return nil, errors.WithMessage(err, "load network from file failed")
	}
	// 从 networks 字典中取到容器链接的网络的信息，networks 字典中保存了当前已经创建的网络
	network, ok := networks[networkName]
	if !ok {
		return nil, fmt.Errorf("no Such Network: %s", networkName)
	}

	// 分配容器 IP 地址
	ip, err := ipAllocator.Allocate(network.IPRange)
	if err != nil {
		return ip, errors.Wrapf(err, "allocate ip")
	}
	// 创建网络端点
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", info.Id, networkName),
		IPAddress:   ip,
		Network:     network,
		PortMapping: info.PortMapping,
	}
	// 调用网络驱动挂载和配置网络端点
	if err = drivers[network.Driver].Connect(networkName, ep); err != nil {
		return ip, err
	}
	// 到容器的 namespace 配置容器网络设备 IP 地址
	if err = configEndpointIpAndRoute(ep, info); err != nil {
		return ip, err
	}
	// 配置端口映射信息，例如 mydocker run -p 8080:80
	return ip, addPortMapping(ep)
}

/* 将容器中指定网络移除 */
func Disconnect(networkName string, info *container.Info) error {
	networks, err := loadNetwork()
	if err != nil {
		return errors.WithMessage(err, "load network from file failed")
	}
	// 从networks字典中取到容器连接的网络的信息，networks字典中保存了当前己经创建的网络
	network, ok := networks[networkName]
	if !ok {
		return fmt.Errorf("no Such Network: %s", networkName)
	}
	// veth 从 bridge 解绑并删除 veth-pair 设备对
	drivers[network.Driver].Disconnect(fmt.Sprintf("%s-%s", info.Id, networkName))

	// 调用 ipAlloctor 释放容器的 IP
	ip := net.ParseIP(info.IP)
	if ip == nil {
		return fmt.Errorf("invalid IP address: %s", info.IP)
	}
	if err = ipAllocator.Release(network.IPRange, &ip); err != nil {
		return errors.Wrap(err, "remove Network gateway ip failed")
	}

	// 清理端口映射添加的 iptables 规则
	ep := &Endpoint{
		ID:          fmt.Sprintf("%s-%s", info.Id, networkName),
		IPAddress:   net.ParseIP(info.IP),
		Network:     network,
		PortMapping: info.PortMapping,
	}
	return deletePortMapping(ep)
}

/*
 * 将容器的网络端点加入到容器的网络空间中
 * 并锁定当前程序所执行的线程，使当前线程进入到容器的网络空间
 * 返回值是一个函数指针，执行这个返回函数才会退出容器的网络空间，回归到宿主机的网络空间
 */
func enterContainerNetns(enLink *netlink.Link, info *container.Info) func() {
	// 找到这个容器的 Net Namespace
	// /proc/[pid]/ns/net 打开这个文件的文件描述符就可以来操作 Net Namespace
	// 而 ContainerInfo 中的 PID，即容器在宿主机上映射的进程 ID
	// 它对应的 /proc/[pid]/ns/net 就是容器内部的 Net Namespace
	f, err := os.OpenFile(fmt.Sprintf("/proc/%s/ns/net", info.Pid), os.O_RDONLY, 0)
	if err != nil {
		log.Errorf("error get container net namespace, %v", err)
	}

	nsFD := f.Fd()
	// 锁定当前程序所执行的线程，如果不锁定操作系统线程的话
	// Go 语言的 goroutine 可能会被调度到别的线程上去
	// 就不能保证一直在所需要的网络空间中了
	// 所以先调用 runtime.LockOSThread() 锁定当前程序执行的线程
	runtime.LockOSThread()

	// 修改网络端点 Veth 的另一端，将其移动到容器的 Net Namespace 中
	if err = netlink.LinkSetNsFd(*enLink, int(nsFD)); err != nil {
		log.Errorf("error set link netns , %v", err)
	}

	// 获取当前的网络 namespace
	origins, err := netns.Get()
	if err != nil {
		log.Errorf("error get current netns, %v", err)
	}

	// 调用 netns.Set 方法，将当前进程加入容器的 Net Namespace
	if err = netns.Set(netns.NsHandle(nsFD)); err != nil {
		log.Errorf("error set netns, %v", err)
	}
	// 返回之前 Net Namespace 的函数
	// 在容器的网络空间中执行完容器配置之后调用此函数就可以将程序恢复到原生的 Net Namespace
	return func() {
		// 恢复到上面获取到的之前的 Net Namespace
		netns.Set(origins)
		origins.Close()
		// 取消对当前程序的线程锁定
		runtime.UnlockOSThread()
		f.Close()
	}
}

/* 配置容器网络端点的地址和路由 */
func configEndpointIpAndRoute(ep *Endpoint, info *container.Info) error {
	// 根据名字找到对应 Veth 设备
	peerLink, err := netlink.LinkByName(ep.Device.PeerName)
	if err != nil {
		return errors.WithMessagef(err, "found veth [%s] failed", ep.Device.PeerName)
	}
	// 将容器的网络端点加入到容器的网络空间中
	// 并使这个函数下面的操作都在这个网络空间中进行
	// 执行完函数后，恢复为默认的网络空间
	defer enterContainerNetns(&peerLink, info)()

	// 获取到容器的IP地址及网段，用于配置容器内部接口地址
	// 比如容器IP是192.168.1.2，而网络的网段是192.168.1.0/24
	// 那么这里产出的 IP 字符串就是192.168.1.2/24，用于容器内 Veth 端点配置
	interfaceIP := *ep.Network.IPRange
	interfaceIP.IP = ep.IPAddress
	// 设置容器内 Veth 端点的 IP
	if err = setInterfaceIP(ep.Device.PeerName, interfaceIP.String()); err != nil {
		return fmt.Errorf("%v,%s", ep.Network, err)
	}
	// 启动容器内的 Veth 端点
	if err = setInterfaceUP(ep.Device.PeerName); err != nil {
		return err
	}
	// Net Namespace 中默认本地地址 127.0.0.1 的 “lo” 网卡是关闭状态的
	// 启动它以保证容器访问自己的请求
	if err = setInterfaceUP("lo"); err != nil {
		return err
	}
	// 设置容器内的外部请求都通过容器内的Veth端点访问
	// 0.0.0.0/0的网段，表示所有的IP地址段
	_, cidr, _ := net.ParseCIDR("0.0.0.0/0")
	// 构建要添加的路由数据，包括网络设备、网关 IP 及目的网段
	// 相当于route add -net 0.0.0.0/0 gw {Bridge网桥地址} dev {容器内的Veth端点设备}
	defaultRoute := &netlink.Route{
		LinkIndex: peerLink.Attrs().Index,
		Gw:        ep.Network.IPRange.IP,
		Dst:       cidr,
	}
	// 调用netlink的RouteAdd,添加路由到容器的网络空间
	// RouteAdd 函数相当于route add 命令
	if err = netlink.RouteAdd(defaultRoute); err != nil {
		return err
	}

	return nil
}

func addPortMapping(ep *Endpoint) error {
	return configPortMapping(ep, false)
}

func deletePortMapping(ep *Endpoint) error {
	return configPortMapping(ep, true)
}

// configPortMapping 配置端口映射
func configPortMapping(ep *Endpoint, isDelete bool) error {
	action := "-A"
	if isDelete {
		action = "-D"
	}
	var err error
	// 遍历容器端口映射列表
	for _, pm := range ep.PortMapping {
		// 分割成宿主机的端口和容器的端口
		portMapping := strings.Split(pm, ":")
		if len(portMapping) != 2 {
			log.Errorf("port mapping format error, %v", pm)
			continue
		}
		// 由于 iptables 没有Go语言版本的实现，所以采用exec.Command的方式直接调用命令配置
		// 在 iptables 的 PREROUTING 中添加 DNAT 规则
		// 将宿主机的端口请求转发到容器的地址和端口上
		// iptables -t nat -A PREROUTING ! -i testbridge -p tcp -m tcp --dport 8080 -j DNAT --to-destination 10.0.0.4:80
		iptablesCmd := fmt.Sprintf("-t nat %s PREROUTING ! -i %s -p tcp -m tcp --dport %s -j DNAT --to-destination %s:%s",
			action, ep.Network.Name, portMapping[0], ep.IPAddress.String(), portMapping[1])
		cmd := exec.Command("iptables", strings.Split(iptablesCmd, " ")...)
		log.Infoln("配置端口映射 DNAT cmd:", cmd.String())
		// 执行iptables命令,添加端口映射转发规则
		output, err := cmd.Output()
		if err != nil {
			log.Errorf("iptables Output, %v", output)
			continue
		}
	}
	return err
}
