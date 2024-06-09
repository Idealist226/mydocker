package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"mydocker/container"

	log "github.com/sirupsen/logrus"
)

// nsenter 里的 C 代码里已经出现 mydocker_pid 和 mydocker_cmd 这两个 Key
// 主要是为了控制是否执行 C 代码里面的 setns.
const (
	EnvExecPid = "mydocker_pid"
	EnvExecCmd = "mydocker_cmd"
)

func ExecContainer(containerId string, cmdArray []string) {
	// 根据传进来的容器 ID 获取对应的 PID
	pid, err := getPidByContainerId(containerId)
	if err != nil {
		log.Errorf("Exec container getContainerPidByName %s error %v", containerId, err)
		return
	}

	cmd := exec.Command("/proc/self/exe", "exec")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 把命令拼接成字符串，便于传递
	cmdStr := strings.Join(cmdArray, " ")
	log.Infof("container pid: %s command: %s", pid, cmdStr)
	_ = os.Setenv(EnvExecPid, pid)
	_ = os.Setenv(EnvExecCmd, cmdStr)

	if err = cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerId, err)
	}
}
func getPidByContainerId(containerId string) (string, error) {
	// 查找记录容器信息的文件路径
	configFilePath := container.GetConfigFilePath(containerId)
	// 读取内容并解析
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return "", err
	}
	var containerInfo container.Info
	if err = json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return "", err
	}
	return containerInfo.Pid, nil
}
