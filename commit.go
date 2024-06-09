package main

import (
	"os/exec"

	"mydocker/utils"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

var ErrImageAlreadyExists = errors.New("Image Already Exists")

func CommitContainer(containerId, imageName string) error {
	mntPath := utils.GetMerged(containerId)
	imageTar := utils.GetImage(imageName)
	exists, err := utils.PathExists(imageTar)
	if err != nil {
		return errors.WithMessagef(err, "check is image [%s/%s] exist failed", imageName, imageTar)
	}
	if exists {
		return ErrImageAlreadyExists
	}
	log.Infof("commitContainer imageTar: %s", imageTar)
	if _, err := exec.Command("tar", "-czf", imageTar, "-C", mntPath, ".").CombinedOutput(); err != nil {
		log.Errorf("Tar folder %s error %v", mntPath, err)
	}
	return nil
}
