package utils

import "fmt"

const (
	ImagePath       = "/var/lib/mydocker/image/"
	RootPath        = "/var/lib/mydocker/overlay2/"
	lowerDirFormat  = RootPath + "%s/lower"
	upperDirFormat  = RootPath + "%s/upper"
	workDirFormat   = RootPath + "%s/work"
	mergedDirFormat = RootPath + "%s/merged"
	overlayFSFormat = "lowerdir=%s,upperdir=%s,workdir=%s"
)

func GetImage(imageName string) string {
	return fmt.Sprintf("%s%s.tar", ImagePath, imageName)
}

func GetRoot(containerId string) string {
	return RootPath + containerId
}

func GetLower(containerId string) string {
	return fmt.Sprintf(lowerDirFormat, containerId)
}

func GetUpper(containerID string) string {
	return fmt.Sprintf(upperDirFormat, containerID)
}

func GetWorker(containerID string) string {
	return fmt.Sprintf(workDirFormat, containerID)
}

func GetMerged(containerID string) string {
	return fmt.Sprintf(mergedDirFormat, containerID)
}

func GetOverlayFSDirs(lower, upper, worker string) string {
	return fmt.Sprintf(overlayFSFormat, lower, upper, worker)
}
