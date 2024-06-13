package container

import (
	"os"
	"os/exec"
	"path"
	"syscall"

	"mydocker/constant"
	"mydocker/utils"

	log "github.com/sirupsen/logrus"
)

const (
	RUNNING       = "running"
	STOP          = "stopped"
	Exit          = "exited"
	InfoLoc       = "/var/lib/mydocker/containers/"
	InfoLocFormat = InfoLoc + "%s/"
	ConfigName    = "config.json"
	IDLength      = 10
	LogFile       = "%s-json.log"
)

type Info struct {
	Pid         string   `json:"pid"`         // 容器的init进程在宿主机上的 PID
	Id          string   `json:"id"`          // 容器Id
	Name        string   `json:"name"`        // 容器名
	Command     string   `json:"command"`     // 容器内init运行命令
	CreatedTime string   `json:"createTime"`  // 创建时间
	Status      string   `json:"status"`      // 容器的状态
	Volume      string   `json:"volume"`      // 容器的数据卷
	NetworkName string   `json:"networkName"` // 容器所在的网络
	PortMapping []string `json:"portMapping"` // 端口映射
	IP          string   `json:"ip"`
}

/*
 * 这里是父进程，也就是当前进程执行的内容
 * 1. 这里的 /proc/self/exe 调用中，/proc/self/ 指向当前正在执行的进程的环境，exec 是自己调用自己，使用这种方式对创造出来的进程进行初始化
 * 2. 后面的 args 是参数，其中 init 是传递给本进程的第一个参数，在本例中，其实就是会去调用 initCommand 去初始化进程的一些环境和资源
 * 3. Cloneflags 参数是用来设置进程的 Namespace 类型的，这里设置了五个 Namespace，分别是 UTS、PID、Mount、IPC、Network
 * 4. 如果 tty 为 true，那么就会将当前进程的标准输入、输出、错误输出都映射到新创建出来的进程中
 * 5. 返回创建好的 cmd
 */
func NewParentProcess(tty bool, volume, containerId, imageName string, envSlice []string) (*exec.Cmd, *os.File) {
	// 创建匿名管道用于传递参数，将 readPipe 作为子进程的 ExtraFiles，子进程从 readPipe 中读取参数
	// 父进程中则通过 writePipe 将参数写入管道
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		log.Errorf("New pipe error: %v", err)
		return nil, nil
	}
	cmd := exec.Command("/proc/self/exe", "init")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS |
			syscall.CLONE_NEWNET | syscall.CLONE_NEWIPC,
	}
	if tty {
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		// 对于后台运行容器，将 stdout、stderr 重定向到日志文件中，便于后续查看
		dirPath := GetConfigDirPath(containerId)
		if err := os.MkdirAll(dirPath, constant.Perm0622); err != nil {
			log.Errorf("NewParentProcess Mkdir dir %s error %v", dirPath, err)
			return nil, nil
		}
		stdLogFilePath := path.Join(dirPath, GetLogFileName(containerId))
		stdLogFile, err := os.Create(stdLogFilePath)
		if err != nil {
			log.Errorf("NewParentProcess Create log file %s error %v", stdLogFilePath, err)
			return nil, nil

		}
		cmd.Stdout = stdLogFile
		cmd.Stderr = stdLogFile
	}
	cmd.Env = append(os.Environ(), envSlice...)
	cmd.ExtraFiles = []*os.File{readPipe}
	NewWorkSpace(containerId, imageName, volume)
	cmd.Dir = utils.GetMerged(containerId)
	return cmd, writePipe
}
