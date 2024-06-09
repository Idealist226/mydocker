package container

import (
	"mydocker/constant"
	"mydocker/utils"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// NewWorkSpace Create an Overlay2 filesystem as container root workspace
/*
 * 1) 创建 lower 层
 * 2) 创建 upper、worker 层
 * 3) 创建 merged 目录并挂载 overlayFS
 * 4) 如果有指定 volume 则挂载 volume
 */
func NewWorkSpace(containerId, imageName, volume string) {
	createLower(containerId, imageName)
	createDirs(containerId)
	mountOverlayFS(containerId)

	if volume != "" {
		mntPath := utils.GetMerged(containerId)
		hostPath, containerPath, err := volumeExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume parameter input is not correct, detail:%v", err)
			return
		}
		mountVolume(mntPath, hostPath, containerPath)
	}
}

// DeleteWorkSpace Delete the UFS filesystem while container exit
/*
 * 和创建相反
 * 1) 有 volume 则卸载 volume
 * 2) 卸载并移除 merged 目录
 * 3) 卸载并移除 upper、worker 层
 */
func DeleteWorkSpace(containerId, volume string) {
	// 如果制定了 volume 则需要 umount volume
	// NOTE: 一定要要先 umount volume ，然后再删除目录，否则由于 bind mount 存在，删除临时目录会导致 volume 目录中的数据丢失。
	if volume != "" {
		_, containerPath, err := volumeExtract(volume)
		if err != nil {
			log.Errorf("extract volume failed, maybe volume parameter input is not correct, detail:%v", err)
			return
		}
		mntPath := utils.GetMerged(containerId)
		umountVolume(mntPath, containerPath)
	}
	umountOverlayFS(containerId)
	deleteDirs(containerId)
}

// createLower 根据 containerID、imageName 准备 lower 层目录
func createLower(containerId, imageName string) {
	// 根据 containerId 拼接出 lower目录
	// 根据 imageName 找到镜像 tar，并解压到 lower 目录中
	lowerPath := utils.GetLower(containerId)
	imagePath := utils.GetImage(imageName)
	log.Infof("lower:%s image.tar:%s", lowerPath, imagePath)
	// 检查目录是否已经存在
	exist, err := utils.PathExists(lowerPath)
	if err != nil {
		log.Errorf("Fail to judge whether dir %s exists. %v", lowerPath, err)
	}
	// 不存在则创建目录并将 image.tar 解压到 lower 文件夹中
	if !exist {
		if err = os.MkdirAll(lowerPath, constant.Perm0777); err != nil {
			log.Errorf("Mkdir dir %s error. %v", lowerPath, err)
		}
		if _, err = exec.Command("tar", "-xvf", imagePath, "-C", lowerPath).CombinedOutput(); err != nil {
			log.Errorf("Untar dir %s error %v", lowerPath, err)
		}
	}
}

// createDirs 创建overlayfs需要的的merged、upper、worker目录
func createDirs(containerId string) {
	dirs := []string{
		utils.GetMerged(containerId),
		utils.GetUpper(containerId),
		utils.GetWorker(containerId),
	}

	for _, dir := range dirs {
		if err := os.Mkdir(dir, constant.Perm0777); err != nil {
			log.Errorf("Mkdir dir %s error. %v", dir, err)
		}
	}
}

// mountOverlayFS 挂载 overlayfs
func mountOverlayFS(containerId string) {
	// 拼接参数
	// e.g. lowerdir=/root/busybox,upperdir=/root/upper,workdir=/root/work
	dirs := utils.GetOverlayFSDirs(utils.GetLower(containerId), utils.GetUpper(containerId), utils.GetWorker(containerId))
	mergedPath := utils.GetMerged(containerId)
	//完整命令：mount -t overlay overlay -o lowerdir={lowerdir},upperdir={upperdir},workdir={workdir} {mergeddir}
	cmd := exec.Command("mount", "-t", "overlay", "overlay", "-o", dirs, mergedPath)
	log.Infof("mount overlayfs: [%s]", cmd.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Mount overlayfs error: %v", err)
	}
}

// umountOverlayFS 卸载 overlayfs
func umountOverlayFS(containerId string) {
	mntPath := utils.GetMerged(containerId)
	cmd := exec.Command("umount", mntPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	log.Infof("umountOverlayFS,cmd:%v", cmd.String())
	if err := cmd.Run(); err != nil {
		log.Errorf("Umount overlayfs error: %v", err)
	}
}

// deleteDirs 删除 overlayfs 的 merged、upper、work 目录
func deleteDirs(containerId string) {
	dirs := []string{
		utils.GetMerged(containerId),
		utils.GetUpper(containerId),
		utils.GetWorker(containerId),
		utils.GetLower(containerId),
		utils.GetRoot(containerId), // root 目录也要删除
	}

	for _, dir := range dirs {
		if err := os.RemoveAll(dir); err != nil {
			log.Errorf("Remove dir %s error. %v", dir, err)
		}
	}
}
