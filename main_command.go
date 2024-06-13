package main

import (
	"fmt"
	"os"

	"mydocker/cgroups/subsystems"
	"mydocker/container"
	"mydocker/network"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var runCommand = cli.Command{
	Name: "run",
	Usage: `Create a container with namespace and cgroups limit
			mydocker run -it/-d [-name containerName] imageName command [arg...]`,
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
		cli.StringSliceFlag{ // 针对多个环境变量的情况
			Name:  "e", // 环境变量
			Usage: "set environment variables, e.g. -e name=mydocker",
		},
		cli.StringFlag{
			Name:  "net",
			Usage: "container network, e.g. -net testbr",
		},
		cli.StringSliceFlag{
			Name:  "p",
			Usage: "port mapping, e.g. -p 8080:80 -p 30336:3306",
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

		// get image name
		imageName := cmdArray[0]
		cmdArray = cmdArray[1:]

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
		envSlice := context.StringSlice("e")
		network := context.String("net")
		portMapping := context.StringSlice("p")
		Run(tty, cmdArray, envSlice, portMapping, resConf, volume, containerName, imageName, network)
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
	Usage: "commit container to image, e.g. mydocker commit 123456789 myimage",
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 2 {
			return fmt.Errorf("missing container name and image name")
		}
		containerId := context.Args().Get(0)
		imageName := context.Args().Get(1)
		return CommitContainer(containerId, imageName)
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
	Usage: "exec a command in container, e.g. mydocker exec 123456789 /bin/sh",
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

var removeCommand = cli.Command{
	Name:  "rm",
	Usage: "remove a container, e.g. mydocker rm 1234567890",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "f", // 强制删除
			Usage: "force delete running container",
		}},
	Action: func(context *cli.Context) error {
		if len(context.Args()) < 1 {
			return fmt.Errorf("missing container id")
		}
		containerId := context.Args().Get(0)
		force := context.Bool("f")
		RemoveContainer(containerId, force)
		return nil
	},
}

var networkCommand = cli.Command{
	Name:  "network",
	Usage: "container network commands",
	Subcommands: []cli.Command{
		{
			Name:  "create",
			Usage: "create a container network",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "driver",
					Usage: "network driver",
				},
				cli.StringFlag{
					Name:  "subnet",
					Usage: "subnet cidr",
				},
			},
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("missing network name")
				}
				driver := context.String("driver")
				subnet := context.String("subnet")
				networkName := context.Args().Get(0)
				err := network.CreateNetwork(driver, subnet, networkName)
				if err != nil {
					return fmt.Errorf("create network error: %+v", err)
				}
				return nil
			},
		},
		{
			Name:  "list",
			Usage: "list container network",
			Action: func(context *cli.Context) error {
				network.ListNetwork()
				return nil
			},
		},
		{
			Name:  "remove",
			Usage: "remove container network",
			Action: func(context *cli.Context) error {
				if len(context.Args()) < 1 {
					return fmt.Errorf("missing network name")
				}
				err := network.DeleteNetwork(context.Args()[0])
				if err != nil {
					return fmt.Errorf("remove network error: %+v", err)
				}
				return nil
			},
		},
	},
}
