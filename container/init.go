package container

import (
	"io"
	"mydocker/constant"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"

	errors "github.com/pkg/errors"
)

/*
 * 这里的 init 函数是在容器内部执行的，也就是说，代码执行到这里后，容器所在的进程其实就已经创建出来了，
 * 这是本容器执行的第一个进程。
 * 使用 mount 先去挂载 proc 文件系统，以便后面通过 ps 等系统命令去查看当前进程资源的情况。
 */
func RunContainerInitProcess() error {
	// 从 Pipe 中读取命令
	cmdArray := readUserCommand()
	if len(cmdArray) == 0 {
		return errors.New("run container get user command error, cmdArray is nil")
	}

	// 挂载文件系统
	setUpMount()

	path, err := exec.LookPath(cmdArray[0])
	if err != nil {
		log.Errorf("Exec loop path error: %v", err)
		return err
	}
	log.Infof("Find path %s", path)
	if err := syscall.Exec(path, cmdArray[0:], os.Environ()); err != nil {
		log.Errorf("RunContainerInitProcess exec :" + err.Error())
	}
	return nil
}

const fdIndex = 3

func readUserCommand() []string {
	// uintptr(3）就是指 index 为 3 的文件描述符，也就是传递进来的管道的另一端，至于为什么是3，具体解释如下：
	/*
		因为每个进程默认都会有 3 个文件描述符，分别是标准输入、标准输出、标准错误。这 3 个是子进程一创建的时候就会默认带着的，
		前面通过 ExtraFiles 方式带过来的 readPipe 理所当然地就成为了第 4 个。
		在进程中可以通过 index 方式读取对应的文件，比如
		index0：标准输入
		index1：标准输出
		index2：标准错误
		index3：带过来的第一个 FD，也就是 readPipe
		由于可以带多个 FD 过来，所以这里的 3 就不是固定的了。
		比如像这样：cmd.ExtraFiles = []*os.File{a,b,c,readPipe} 这里带了 4 个文件过来，分别的 index 就是 3,4,5,6
		那么我们的 readPipe 就是 index6，读取时就要像这样：pipe := os.NewFile(uintptr(6), "pipe")
	*/
	pipe := os.NewFile(uintptr(fdIndex), "pipe")
	defer pipe.Close()

	msg, err := io.ReadAll(pipe)
	if err != nil {
		log.Errorf("init read pipe error: %v", err)
		return nil
	}
	msgStr := string(msg)
	return strings.Split(msgStr, " ")
}

/*
 * Init 挂载点
 */
func setUpMount() {
	// 获取当前路径
	pwd, err := os.Getwd()
	if err != nil {
		log.Errorf("Get current location error %v", err)
		return
	}
	log.Infof("Current location is %s", pwd)

	// systemd 加入 Linux 之后, mount namespace 就变成 shared by default,
	// 所以你必须显式声明你要这个新的 mount namespace 独立。
	// 即 mount proc 之前先把所有挂载点的传播类型改为 private，避免本 namespace 中的挂载事件外泄。
	// 如果不先做 private mount，会导致挂载事件外泄，后续再执行 mydocker 命令时 /proc 文件系统异常
	// 可以执行 mount -t proc proc /proc 命令重新挂载来解决
	_ = syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")

	err = pivotRoot(pwd)
	if err != nil {
		log.Errorf("pivot root error: %v", err)
	}

	// mount /proc
	defaultMountFlags := syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV
	_ = syscall.Mount("proc", "/proc", "proc", uintptr(defaultMountFlags), "")

	// 由于前面 pivotRoot 切换了 rootfs，因此这里重新 mount 一下 /dev 目录
	// tmpfs 是一种基于内存的文件系统，可以使用 RAM、swap 分区来存储。
	// 不挂载 /dev，会导致容器内部无法访问和使用许多设备，这可能导致系统无法正常工作
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")
}

func pivotRoot(root string) error {
	/*
	 * NOTE: PivotRoot 调用有限制，newRoot 和 oldRoot 不能再同一个文件系统下
	 * 因此，为了使当前 root 的老 root 和新 root 不在同一个文件系统下，这里把 root 重新 mount 了一次
	 * bind mount 是把相同的内容换了一个挂载点的挂载方法
	 */
	if err := syscall.Mount(root, root, "bind", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}

	// 创建 rootfs/.pivot_root 目录用于存储 old_root
	pivotDir := filepath.Join(root, ".pivot_root")
	if err := os.Mkdir(pivotDir, constant.Perm0777); err != nil {
		return err
	}

	// 执行 pivot_root 调用，将系统 rootfs 切换到新的 rootfs
	// PivotRoot 调用会把 old_root 挂载到 pivotDir，也就是rootfs/.pivot_root，挂载点现在依然可以在 mount 命令中看到
	if err := syscall.PivotRoot(root, pivotDir); err != nil {
		return errors.WithMessagef(err, "pivotRoot failed,new_root:%v old_put:%v", root, pivotDir)
	}

	// 修改当前的工作目录到根目录
	if err := syscall.Chdir("/"); err != nil {
		return errors.WithMessage(err, "chdir to / failed")
	}

	// 最后再把 old_root umount 了，即 umount rootfs/.pivot_root
	// 由于当前已经是在 rootfs 下了，就不能再用上面的 rootfs/.pivot_root 这个路径了，现在直接用/.pivot_root这个路径即可
	pivotDir = filepath.Join("/", ".pivot_root")
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return errors.WithMessage(err, "unmount pivot_root dir failed")
	}

	// 删除临时文件夹
	return os.Remove(pivotDir)
}
