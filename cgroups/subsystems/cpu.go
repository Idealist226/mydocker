package subsystems

import (
	"fmt"
	"os"
	"path"
	"strconv"

	"mydocker/constant"
)

type CpuSubSystem struct {
}

const (
	PeriodDefault = 100000
	Percent       = 100
)

// Name 返回 cgroup 名字
func (s *CpuSubSystem) Name() string {
	return "cpu"
}

// Set 设置 cgroupPath 对应的 cgroup 的 CPU 限制
func (s *CpuSubSystem) Set(cgroupPath string, res *ResourceConfig) error {
	if res.CpuCfsQuota == 0 && res.CpuShare == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, true)
	if err != nil {
		return err
	}

	// cpu.shares 控制的是 CPU 使用比例，不是绝对值
	if res.CpuShare != "" {
		err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.shares"),
			[]byte(res.CpuShare),
			constant.Perm0644)
		if err != nil {
			return fmt.Errorf("set cgroup cpu share fail %v", err)
		}
	}

	if res.CpuCfsQuota != 0 {
		// cpu.cfs_period_us 控制的是 CPU 分配周期的时间，单位是微秒
		// 这个值通常设置为 100000，即 100ms，这意味着内核会每 100 毫秒重新调整一次 CPU 时间的分配
		err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.cfs_period_us"),
			[]byte(strconv.Itoa(PeriodDefault)),
			constant.Perm0644)
		if err != nil {
			return fmt.Errorf("set cgroup cpu.cfs_period_us fail %v", err)

		}

		// cpu.cfs_quota_us 定义了在 cpu.cfs_period_us 定义的周期内，一组进程可以运行的 CPU 时间
		// 如果这个值小于周期时间，它将限制进程组的 CPU 使用率
		// 此处的 cpu.cfs_quota_us 根据用户传递的参数 res.CpuCfsQuota 来控制，比如参数为 20，就是希望限制为 20% CPU
		// 所以把 cpu.cfs_quota_us 设置为 cpu.cfs_period_us 的 20% 就行
		// 这里只是简单的计算了下，并没有处理一些特殊情况，比如负数什么的
		err = os.WriteFile(path.Join(subsysCgroupPath, "cpu.cfs_quota_us"),
			[]byte(strconv.Itoa(PeriodDefault/Percent*res.CpuCfsQuota)),
			constant.Perm0644)
		if err != nil {
			return fmt.Errorf("set cgroup cpu.cfs_quota_us fail %v", err)
		}
	}

	return nil
}

// Apply 将 pid 加入到 cgroupPath 对应的 cgroup 中
func (s *CpuSubSystem) Apply(cgroupPath string, pid int, res *ResourceConfig) error {
	if res.CpuCfsQuota == 0 && res.CpuShare == "" {
		return nil
	}

	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return fmt.Errorf("get cgroup %s error: %v", cgroupPath, err)
	}

	err = os.WriteFile(path.Join(subsysCgroupPath, "tasks"),
		[]byte(strconv.Itoa(pid)),
		constant.Perm0644)
	if err != nil {
		return fmt.Errorf("set cgroup proc fail %v", err)
	}
	return nil
}

// Remove 删除 cgroupPath 对应的 cgroup
func (s *CpuSubSystem) Remove(cgroupPath string) error {
	subsysCgroupPath, err := getCgroupPath(s.Name(), cgroupPath, false)
	if err != nil {
		return err
	}
	return os.RemoveAll(subsysCgroupPath)
}
