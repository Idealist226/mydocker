package main

import (
	"encoding/json"
	"os"
	"strconv"
	"syscall"

	"mydocker/constant"
	"mydocker/container"
	"mydocker/network"

	log "github.com/sirupsen/logrus"
)

func StopContainer(containerId string) {
	// 1. 根据容器Id查询容器信息
	containerInfo, err := container.GetInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	pidInt, err := strconv.Atoi(containerInfo.Pid)
	if err != nil {
		log.Errorf("Conver pid from string to int error %v", err)
		return
	}
	// 2. 发送 SIGTERM 信号
	if err = syscall.Kill(pidInt, syscall.SIGTERM); err != nil {
		log.Errorf("Stop container %s error %v", containerId, err)
		return
	}
	// 3. 修改容器信息，将容器置为 STOP 状态，并清空 PID
	containerInfo.Status = container.STOP
	containerInfo.Pid = ""
	newContentBytes, err := json.Marshal(containerInfo)
	if err != nil {
		log.Errorf("Json marshal %s error %v", containerId, err)
		return
	}
	// 4. 重新协会存储容器信息的文件
	configFilePath := container.GetConfigFilePath(containerId)
	if err := os.WriteFile(configFilePath, newContentBytes, constant.Perm0622); err != nil {
		log.Errorf("Write file %s error:%v", configFilePath, err)
	}
}

func RemoveContainer(containerId string, force bool) {
	containerInfo, err := container.GetInfoByContainerId(containerId)
	if err != nil {
		log.Errorf("Get container %s info error %v", containerId, err)
		return
	}
	switch containerInfo.Status {
	case container.STOP:
		// 如果容器已经停止，直接删除容器信息
		// 先删除配置目录，再删除 rootfs 目录
		if err = container.DeleteContainerInfo(containerId); err != nil {
			log.Errorf("Remove container [%s]'s config failed, detail: %v", containerId, err)
			return
		}
		container.DeleteWorkSpace(containerId, containerInfo.Volume)
		if containerInfo.NetworkName != "" { // 清理网络资源
			if err = network.Disconnect(containerInfo.NetworkName, containerInfo); err != nil {
				log.Errorf("Remove container [%s]'s config failed, detail: %v", containerId, err)
				return
			}
		}
	case container.RUNNING:
		// 如果容器正在运行，且强制删除为 true，则停止容器后删除容器信息
		if !force {
			log.Errorf("Couldn't remove running container [%s], Stop the container before attempting removal or"+
				" force remove", containerId)
			return
		}
		log.Infof("force delete running container [%s]", containerId)
		StopContainer(containerId)
		RemoveContainer(containerId, force)
	default:
		log.Errorf("Couldn't remove container,invalid status %s", containerInfo.Status)
		return
	}

}
