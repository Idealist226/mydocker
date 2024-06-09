package main

import (
	"encoding/json"
	"mydocker/constant"
	"mydocker/container"
	"os"
	"strconv"
	"syscall"

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