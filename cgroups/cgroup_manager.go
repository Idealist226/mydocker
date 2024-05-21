package cgroups

import (
	"mydocker/cgroups/subsystems"

	log "github.com/sirupsen/logrus"
)

type CgroupManager struct {
	// cgroup 在 hierarchy 中的路径，相当于创建的 cgroup 目录相对于 root cgroup 目录的路径
	Path string
	// 资源配置
	Resource *subsystems.ResourceConfig
}

func NewCgroupManager(path string) *CgroupManager {
	return &CgroupManager{
		Path: path,
	}
}

// Set 设置 cgroup 资源限制
func (c *CgroupManager) Set(res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Set(c.Path, res)
		if err != nil {
			log.Errorf("set subsystem:%s err:%s", subSysIns.Name(), err)
		}
	}
	return nil
}

// Apply 将进程 PID 加入到 cgroup 中
func (c *CgroupManager) Apply(pid int, res *subsystems.ResourceConfig) error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Apply(c.Path, pid, res)
		if err != nil {
			log.Errorf("apply subsystem:%s err:%s", subSysIns.Name(), err)
		}
	}
	return nil
}

// Destroy 释放 cgroup
func (c *CgroupManager) Destroy() error {
	for _, subSysIns := range subsystems.SubsystemsIns {
		err := subSysIns.Remove(c.Path)
		if err != nil {
			log.Errorf("remove subsystem:%s err:%s", subSysIns.Name(), err)
		}
	}
	return nil
}
