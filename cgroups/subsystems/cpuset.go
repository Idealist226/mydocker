package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"mydocker/constant"

	"github.com/pkg/errors"
)

type CpusetSubSystem struct {
}

// Name 返回 cgroup 名字
func (s *CpusetSubSystem) Name() string {
	return "cpuset"
}

// Set 设置 cgroupPath 对应的 cgroup 的 CPU 限制
func (s *CpusetSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.CpuSet == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(subsysCgroupPath, "cpuset.cpus"),
		[]byte(res.CpuSet),
		constant.Perm0644)
	if err != nil {
		return fmt.Errorf("set cgroup cpuset fail %v", err)
	}

	return nil
}

// Apply 将进程 PID 添加到 cgroupPath 对应的 cgroup 中
func (s *CpusetSubSystem) Apply(cgroupPath string, pid int, res *ResourceConfig) error {
	if res.CpuSet == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}

	err = os.WriteFile(path.Join(subsysCgroupPath, "tasks"),
		[]byte(strconv.Itoa(pid)),
		constant.Perm0644)
	if err != nil {
		return fmt.Errorf("set cgroup proc fail %v", err)
	}

	return nil
}

// Remove 移除 cgroupPath 对应的 cgroup
func (s *CpusetSubSystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}

	return os.RemoveAll(subsysCgroupPath)
}
