package network

import (
	"encoding/json"
	"net"
	"os"
	"path"
	"strings"

	"mydocker/constant"
	"mydocker/utils"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const ipamDefaultAllocatorPath = "/var/run/mydocker/network/ipam/subnet.json"

type IPAM struct {
	SubnetAllocatorPath string             // 分配文件存放位置
	Subnets             *map[string]string // 网段和位图算法的数组 map，key 是网段，value 是分配的位图数组
}

// 初始化一个 IPAM 的对象，默认使用 ipamDefaultAllocatorPath 作为分配信息存储位置
var ipAllocator = &IPAM{
	SubnetAllocatorPath: ipamDefaultAllocatorPath,
}

/* Alloc 在网段中分配一个可用的 IP 地址 */
func (ipam *IPAM) Allocate(subnet *net.IPNet) (ip net.IP, err error) {
	// 存放网段中地址分配信息的数据
	ipam.Subnets = &map[string]string{}

	// 从文件中加载已经分配的网段信息
	err = ipam.load()
	if err != nil {
		return nil, errors.Wrap(err, "load subnet allocation info error")
	}
	// net.IPNet.Mask.Size 函数会返回网段的子网掩码的总长度和网段前面的固定位的长度
	// 比如 "127.0.0.0/8" 网段的子网掩码是 "255.0.0.0"
	// 那么 subnet.Mask.Size() 的返回值就是前面 255 所对应的位数和总位数，即 8 和 32
	_, subnet, _ = net.ParseCIDR(subnet.String())
	one, size := subnet.Mask.Size()
	// 如果之前没有分配过这个网段，则初始化网段的分配配置
	if _, exist := (*ipam.Subnets)[subnet.String()]; !exist {
		// 用 "0" 填满这个网段的配置，1 << uint8(size - one) 表示这个网段有多少个可用的 IP
		// size - one 是子网掩码后面的网络位数，2^(size-one) 就是这个网段的可用 IP 数量
		// 而 2^(size-one) 等价 1 << uint8(size - one)
		(*ipam.Subnets)[subnet.String()] = strings.Repeat("0", 1<<uint8(size-one))
	}
	// 遍历这个网段的位图，找到第一个为 0 的位，表示这个 IP 可以分配
	bitmap := (*ipam.Subnets)[subnet.String()]
	for c := range bitmap {
		if bitmap[c] == '0' {
			// 设置这个位为 1，表示这个 IP 已经被分配
			// Go 的字符串，创建之后就不能修改，所以通过转换成 byte 数组，修改后再转换成字符串赋值
			ipalloc := []byte(bitmap)
			ipalloc[c] = '1'
			(*ipam.Subnets)[subnet.String()] = string(ipalloc)
			// 这里的 subnet.IP 只是初始 IP，比如对于网段 192.168.0.0/16，这里就是 192.168.0.0
			ip = subnet.IP
			/*
			 * 还需要通过网段的 IP 与上面的偏移相加计算出分配的 IP 地址
			 * 比如网段是 172.16.0.0/12，则 ip[0] = 172，ip[1] = 16，ip[2] = 0，ip[3] = 0
			 * ip[0] 需要加上 c 的高 8 字符表示的数......ip[3] 需要加上 c 的低 8 字符表示的数
			 */
			/*
				还需要通过网段的 IP 与上面的偏移相加计算出分配的 IP 地址，由于IP地址是uint的一个数组，
				比如网段是 172.16.0.0/12，则 ip[0] = 172，ip[1] = 16，ip[2] = 0，ip[3] = 0
				如果 c 在 bitmap 中的序号是 10, 那么需要在 [172,16,0,0] 上依次加 [uint8(10 >> 24)、uint8(10 >> 16)、
				uint8(10 >> 8)、uint8(10 >> 0)]， 即[0, 0, 0, 10]， 那么获得的 IP 就是 172.16.0.10
			*/
			for t := uint(4); t > 0; t -= 1 {
				ip[4-t] += uint8(c >> ((t - 1) * 8))
			}
			// 由于 IP 是从 1 开始分配的（0 被网关占了），所以最后再加 1，最终得到分配的 IP 172.16.0.11
			ip[3] += 1
			break
		}
	}
	// 最后再调用 dump 将分配结果保存到文件中
	err = ipam.dump()
	if err != nil {
		log.Error("Allocate: dump ipam error", err)
	}
	return
}

func (ipam *IPAM) Release(subnet *net.IPNet, ipaddr *net.IP) error {
	ipam.Subnets = &map[string]string{}
	_, subnet, _ = net.ParseCIDR(subnet.String())

	err := ipam.load()
	if err != nil {
		return errors.Wrap(err, "load subnet allocation info error")
	}
	// 和分配一样的算法，反过来根据 IP 找到位图数组中的对应索引位置
	c := 0
	releaseIP := ipaddr.To4()
	releaseIP[3] -= 1
	for t := uint(4); t > 0; t -= 1 {
		c += int(releaseIP[t-1]-subnet.IP[t-1]) << ((4 - t) * 8)
	}
	// 然后将对应位置 0
	ipalloc := []byte((*ipam.Subnets)[subnet.String()])
	ipalloc[c] = '0'
	(*ipam.Subnets)[subnet.String()] = string(ipalloc)

	// 最后调用 dump 将分配结果保存到文件中
	err = ipam.dump()
	if err != nil {
		log.Error("Allocate: dump ipam error", err)
	}
	return nil
}

/* load 加载网段地址分配信息 */
func (ipam *IPAM) load() error {
	// 检查存储文件状态，如果不存在，则说明之前没有分配，则不需要加载
	exist, err := utils.PathExists(ipam.SubnetAllocatorPath)
	if err != nil {
		return errors.Wrapf(err, "Fail to judge whether dir %s exists.", ipam.SubnetAllocatorPath)
	}
	if !exist {
		return nil
	}

	// 读取文件，加载配置信息
	subnetConfigFile, err := os.Open(ipam.SubnetAllocatorPath)
	if err != nil {
		return errors.Wrapf(err, "Fail to open subnet config file [%s]", ipam.SubnetAllocatorPath)
	}
	defer subnetConfigFile.Close()
	subnetJson := make([]byte, 2000)
	n, err := subnetConfigFile.Read(subnetJson)
	if err != nil {
		return errors.Wrapf(err, "Fail to read subnet config file [%s]", ipam.SubnetAllocatorPath)
	}
	err = json.Unmarshal(subnetJson[:n], ipam.Subnets)
	return errors.Wrap(err, "err dump allocation info")
}

/* dump 存储网段地址分配信息 */
func (ipam *IPAM) dump() error {
	ipamConfigFileDir, _ := path.Split(ipam.SubnetAllocatorPath)
	exist, err := utils.PathExists(ipamConfigFileDir)
	if err != nil {
		return errors.Wrapf(err, "Fail to judge whether dir %s exists.", ipamConfigFileDir)
	}
	if !exist {
		if err = os.MkdirAll(ipamConfigFileDir, constant.Perm0644); err != nil {
			return errors.Wrapf(err, "Mkdir dir %s error.", ipamConfigFileDir)
		}
	}
	// 打开存储文件 O_TRUNC 表示如果存在则消空，O_CREATE 表示如果不存在则创建
	subnetConfigFile, err := os.OpenFile(ipam.SubnetAllocatorPath, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, constant.Perm0644)
	if err != nil {
		return err
	}
	defer subnetConfigFile.Close()
	ipamConfigJson, err := json.Marshal(ipam.Subnets)
	if err != nil {
		return err
	}
	_, err = subnetConfigFile.Write(ipamConfigJson)
	return err
}
