package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"mydocker/constant"

	"github.com/pkg/errors"
)

type MemorySubSystem struct {
}

// Name 返回 cgroup 名字
func (s *MemorySubSystem) Name() string {
	return "memory"
}

// Set 设置 cgroupPath 对应的 cgroup 的内存资源限制
func (s *MemorySubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.MemoryLimit == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}

	// 设置这个 cgroup 的内存限制，即将限制写入到 cgroup 对应目录的 memory.limit_in_bytes 文件中
	err = os.WriteFile(path.Join(subsysCgroupPath, "memory.limit_in_bytes"),
		[]byte(res.MemoryLimit),
		constant.Perm0644)
	if err != nil {
		return fmt.Errorf("set cgroup memory fail %v", err)
	}

	return nil
}

// Apply 将 pid 加入到 cgroupPath 对应的 cgroup 中
func (s *MemorySubSystem) Apply(cgroupPath string, pid int, res *ResourceConfig) error {
	if res.MemoryLimit == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return errors.Wrapf(err, "get cgroup %s", cgroupPath)
	}

	// 将进程 PID 加入到 cgroupPath 对应的 cgroup 中
	err = os.WriteFile(path.Join(subsysCgroupPath, "tasks"),
		[]byte(strconv.Itoa(pid)),
		constant.Perm0644)
	if err != nil {
		return fmt.Errorf("set cgroup proc fail %v", err)
	}

	return nil
}

// Remove 删除 cgroupPath 对应的 cgroup
func (s *MemorySubSystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	return os.RemoveAll(subsysCgroupPath)
}
