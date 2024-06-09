package container

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"mydocker/constant"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func RecordContainerInfo(containerPid int, cmdArray []string, containerName, containerId string) error {
	// 如果未指定容器名，则使用随机生成的 containerID
	if containerName == "" {
		containerName = containerId
	}
	cmd := strings.Join(cmdArray, "")
	containerInfo := &Info{
		Id:          containerId,
		Pid:         strconv.Itoa(containerPid),
		Command:     cmd,
		CreatedTime: time.Now().Format("2006-01-02 15:04:05"),
		Status:      RUNNING,
		Name:        containerName,
	}

	// 将容器信息序列化为 json 字符串
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		return errors.WithMessage(err, "container info marshal failed")
	}
	jsonStr := string(jsonBytes)

	// 拼接出存储容器信息文件的路径，如果目录不存在则级联创建
	dirPath := fmt.Sprintf(InfoLocFormat, containerId)
	if err = os.MkdirAll(dirPath, constant.Perm0622); err != nil {
		return errors.WithMessagef(err, "mkdir %s failed", dirPath)
	}
	// 将容器信息写入文件
	fileName := path.Join(dirPath, ConfigName)
	file, err := os.Create(fileName)
	if err != nil {
		return errors.WithMessagef(err, "create file %s failed", fileName)
	}
	defer file.Close()
	if _, err = file.WriteString(jsonStr); err != nil {
		return errors.WithMessagef(err, "write container info to file %s failed", fileName)
	}

	return nil
}

func DeleteContainerInfo(containerId string) {
	dirPath := fmt.Sprintf(InfoLocFormat, containerId)
	if err := os.RemoveAll(dirPath); err != nil {
		log.Errorf("Remove dir %s error %v", dirPath, err)
	}
}

func GenerateContainerID() string {
	return randStringBytes(IDLength)
}

func randStringBytes(n int) string {
	letterBytes := "1234567890"
	randSource := rand.NewSource(time.Now().UnixNano())
	r := rand.New(randSource)
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[r.Intn(len(letterBytes))]
	}
	return string(b)
}

func GetLogFileName(containerId string) string {
	return fmt.Sprintf(LogFile, containerId)
}

func GetConfigFilePath(containerId string) string {
	dirPath := fmt.Sprintf(InfoLocFormat, containerId)
	configFilePath := path.Join(dirPath, ConfigName)
	return configFilePath
}
