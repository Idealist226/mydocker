package main

import (
	"fmt"
	"mydocker/cgroups/subsystems"
	"mydocker/container"
	"os"

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
		cli.BoolFlag{
			Name:  "d",
			Usage: "detach container",
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
		cli.StringFlag{
			Name:  "name",
			Usage: "container name",
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
		detach := context.Bool("d")
		if tty && detach {
			return fmt.Errorf("it and d flag can not both provided")
		}
		// 如果不指定后台运行，则默认前台运行
		if !detach {
			tty = true
		}

		resConf := &subsystems.ResourceConfig{
			MemoryLimit: context.String("mem"),
			CpuCfsQuota: context.Int("cpu"),
			CpuSet:      context.String("cpuset"),
		}
		volume := context.String("v")
		containerName := context.String("name")
		Run(tty, cmdArray, resConf, volume, containerName)
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

var commitCommand = cli.Command{
	Name:  "commit",
	Usage: "commit container to image",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing image name")
		}
		imageName := context.Args().Get(0)
		CommitContainer(imageName)
		return nil
	},
}

var listCommand = cli.Command{
	Name:  "ps",
	Usage: "list all the containers",
	Action: func(context *cli.Context) error {
		ListContainers()
		return nil
	},
}

var logCommand = cli.Command{
	Name:  "logs",
	Usage: "print logs of a container",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container name")
		}
		containerName := context.Args().Get(0)
		LogContainer(containerName)
		return nil
	},
}

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "exec a command in container",
	Action: func(context *cli.Context) error {
		// 如果环境变量存在，说明 C 代码已经运行过了，即 setns 系统调用已经执行了，
		// 这里就直接返回，避免重复执行
		if os.Getenv(EnvExecPid) != "" {
			log.Infof("pid callback pid %v", os.Getgid())
			return nil
		}
		// 格式：mydocker exec 容器名字 命令，因此至少会有两个参数
		if len(context.Args()) < 2 {
			return fmt.Errorf("missing container name or command")
		}
		containerName := context.Args().Get(0)
		// 将除了容器名之外的参数作为命令部分
		var cmdArray []string
		cmdArray = append(cmdArray, context.Args().Tail()...)
		ExecContainer(containerName, cmdArray)
		return nil
	},
}

var stopCommand = cli.Command{
	Name:  "stop",
	Usage: "stop a container",
	Action: func(context *cli.Context) error {
		// 期望输入是：mydocker stop 容器Id，如果没有指定参数直接打印错误
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container name")
		}
		containerName := context.Args().Get(0)
		StopContainer(containerName)
		return nil
	},
}
