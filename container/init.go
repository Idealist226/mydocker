package container

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

/*
 * 这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了，
 * 这是本容器执行的第一个进程。
 * 使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况。
 */
func RunContainerInitProcess(command string, args []string) error {
	log.Infof("command %s", command)

	// systemd 加入 Linux 之后, mount namespace 就变成 shared by default,
	// 所以你必须显示声明你要这个新的 mount namespace 独立。
	_ = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")
	argv := []string{command}
	if err := syscall.Exec(command, argv, os.Environ()); err != nil {
		log.Errorf(err.Error())
	}
	return nil
}
