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
)

func RecordContainerInfo(containerPid int, cmdArray, portMapping []string,
	containerName, containerId, volume, networkName string) (*Info, error) {
	// 如果未指定容器名，则使用随机生成的 containerID
	if containerName == "" {
		containerName = containerId
	}
	cmd := strings.Join(cmdArray, "")
	containerInfo := &Info{
		Pid:         strconv.Itoa(containerPid),
		Id:          containerId,
		Name:        containerName,
		Command:     cmd,
		CreatedTime: time.Now().Format("2006-01-02 15:04:05"),
		Status:      RUNNING,
		Volume:      volume,
		NetworkName: networkName,
		PortMapping: portMapping,
	}

	// 将容器信息序列化为 json 字符串
	jsonBytes, err := json.Marshal(containerInfo)
	if err != nil {
		return containerInfo, errors.WithMessage(err, "container info marshal failed")
	}
	jsonStr := string(jsonBytes)

	// 拼接出存储容器信息文件的路径，如果目录不存在则级联创建
	dirPath := GetConfigDirPath(containerId)
	if err = os.MkdirAll(dirPath, constant.Perm0622); err != nil {
		return containerInfo, errors.WithMessagef(err, "mkdir %s failed", dirPath)
	}
	// 将容器信息写入文件
	fileName := path.Join(dirPath, ConfigName)
	file, err := os.Create(fileName)
	if err != nil {
		return containerInfo, errors.WithMessagef(err, "create file %s failed", fileName)
	}
	defer file.Close()
	if _, err = file.WriteString(jsonStr); err != nil {
		return containerInfo, errors.WithMessagef(err, "write container info to file %s failed", fileName)
	}

	return containerInfo, nil
}

func DeleteContainerInfo(containerId string) error {
	dirPath := GetConfigDirPath(containerId)
	if err := os.RemoveAll(dirPath); err != nil {
		return errors.WithMessagef(err, "remove dir %s failed", dirPath)
	}
	return nil
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

func GetConfigDirPath(containerId string) string {
	return fmt.Sprintf(InfoLocFormat, containerId)
}

func GetConfigFilePath(containerId string) string {
	dirPath := GetConfigDirPath(containerId)
	configFilePath := path.Join(dirPath, ConfigName)
	return configFilePath
}

func GetInfoByContainerId(containerId string) (*Info, error) {
	configFilePath := GetConfigFilePath(containerId)
	contentBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, errors.Wrapf(err, "read file %s", configFilePath)
	}
	var containerInfo Info
	if err = json.Unmarshal(contentBytes, &containerInfo); err != nil {
		return nil, err
	}
	return &containerInfo, nil
}

func GetPidByContainerId(containerId string) (string, error) {
	containerInfo, err := GetInfoByContainerId(containerId)
	return containerInfo.Pid, err
}
