package subsystems

// ResourceConfig 用于传递资源限制配置的结构体
// 包含内存限制、CPU 时间片权重、CPU 核心数等
type ResourceConfig struct {
	MemoryLimit string
	CpuShare    string
	CpuCfsQuota int
	CpuSet      string
}

// Subsystem 接口定义了 cgroup 中的各个子系统应该具备的方法
// 这里将 cgroup 抽象成了 path，原因是 cgroup 在 hierarchy 的路径，便是虚拟文件系统中的虚拟路径
type Subsystem interface {
	// Name 方法返回 Subsystem 的名字，比如 cpu、memory 等
	Name() string
	// Set 方法用于设置某个 cgroup 在这个 Subsystem 中的资源限制
	Set(path string, res *ResourceConfig) error
	// Apply 方法用于将进程 PID 加入到某个 cgroup 中
	Apply(path string, pid int, res *ResourceConfig) error
	// Remove 方法用于移除某个 cgroup
	Remove(path string) error
}

// SubsystemsIns 是一个 Subsystem 的切片，包含了所有的 Subsystem 实例
var SubsystemsIns = []Subsystem{
	&CpusetSubSystem{},
	&MemorySubSystem{},
	&CpuSubSystem{},
}
