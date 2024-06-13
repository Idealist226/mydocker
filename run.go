package main

import (
	"os"
	"strconv"
	"strings"

	"mydocker/cgroups"
	"mydocker/cgroups/subsystems"
	"mydocker/container"
	"mydocker/network"

	log "github.com/sirupsen/logrus"
)

func Run(tty bool, cmdArray, envSlice, portMapping []string, res *subsystems.ResourceConfig,
	volume, containerName, imageName, net string) {
	// 生成容器 ID
	containerId := container.GenerateContainerID()

	parent, writePipe := container.NewParentProcess(tty, volume, containerId, imageName, envSlice)
	if parent == nil {
		log.Errorf("New parent process error")
		return
	}
	if err := parent.Start(); err != nil {
		log.Errorf("Run parent.Start() error: %v", err)
		return
	}

	// 创建 cgroup manager，并通过调用 set 和 apply 设置资源限制并使限制在容器上生效
	cgroupManager := cgroups.NewCgroupManager("mydocker-cgroup")
	defer cgroupManager.Destroy()
	_ = cgroupManager.Set(res)
	_ = cgroupManager.Apply(parent.Process.Pid, res)

	var containerIP string
	// 如果制定了网络信息则进行配置
	if net != "" {
		// config container network
		containerInfo := &container.Info{
			Pid:         strconv.Itoa(parent.Process.Pid),
			Id:          containerId,
			Name:        containerName,
			PortMapping: portMapping,
		}
		ip, err := network.Connect(net, containerInfo)
		if err != nil {
			log.Errorf("Error Connect Network %v", err)
			return
		}
		containerIP = ip.String()
	}

	// record container info
	containerInfo, err := container.RecordContainerInfo(parent.Process.Pid, cmdArray, portMapping,
		containerName, containerId, volume, net, containerIP)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	// 在子进程创建后通过管道来发送参数
	sendInitCommand(cmdArray, writePipe)
	// 如果是 tty，那么父进程等待，就是前台运行；否则就是跳过，实现后台运行
	if tty {
		_ = parent.Wait()
		container.DeleteWorkSpace(containerId, volume)
		container.DeleteContainerInfo(containerId)
		if net != "" {
			network.Disconnect(net, containerInfo)
		}
	}
}

func sendInitCommand(cmdArray []string, writePipe *os.File) {
	command := strings.Join(cmdArray, " ")
	log.Infof("command all is %s", command)
	_, _ = writePipe.WriteString(command)
	_ = writePipe.Close()
}
