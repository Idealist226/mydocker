package main

import (
	"fmt"
	"mydocker/cgroups/subsystems"
	"mydocker/container"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -it [command]`,
	// 每个命令都可以通过 cli.Flag 指定具体参数
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "it", // 简单起见，这里把 -i 和 -t 参数合并成一个
			Usage: "enable tty",
		},
		cli.StringFlag{
			Name:  "mem", // 限制进程内存使用量
			Usage: "memory limit, e.g.: -mem 100m",
		},
		cli.StringFlag{
			Name:  "cpu", // 限制进程 cpu 使用率
			Usage: "cpu quota, e.g.: -cpu 100",
		},
		cli.StringFlag{
			Name:  "cpuset", // 限制进程 cpu 使用核数
			Usage: "cpuset limit, e.g.: -cpuset 2,4",
		},
		cli.StringFlag{
			Name:  "v", // 数据卷挂载
			Usage: "volume, e.g.: -v /data:/data",
		},
	},
	/*
	 * run 命令执行的真正函数
	 * 1. 判断参数是否包含 command
	 * 2. 获取用户指定的 command
	 * 3. 调用 Run function 去准备启动容器
	 */
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container command")
		}
		var cmdArray []string
		for _, arg := range context.Args() {
			cmdArray = append(cmdArray, arg)
		}
		tty := context.Bool("it")
		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("mem"),
			CpuCfsQuota: context.Int("cpu"),
			CpuSet:      context.String("cpuset"),
		}
		volume := context.String("v")
		Run(tty, cmdArray, resConf, volume)
		return nil
	},
}

var initCommand = cli.Command{
	Name:  "init",
	Usage: "Init container process run user's process in container. Do not call it outside",
	Action: func(context *cli.Context) error {
		log.Infof("init come on")
		err := container.RunContainerInitProcess()
		return err
	},
}
