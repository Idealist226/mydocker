package main

import (
	"fmt"
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
	pid, err := container.GetPidByContainerId(containerId)
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
	cmd.Env = append(os.Environ(), getEnvsByPid(pid)...)

	if err = cmd.Run(); err != nil {
		log.Errorf("Exec container %s error %v", containerId, err)
	}
}

// getEnvsByPid 读取指定 PID 进程的环境变量
func getEnvsByPid(pid string) []string {
	path := fmt.Sprintf("/proc/%s/environ", pid)
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("Read file %s error %v", path, err)
		return nil
	}
	// env split by \u0000
	envs := strings.Split(string(contentBytes), "\u0000")
	return envs
}
