# mydocker
mydocker 是一个简单的容器运行时实现，当然它没有很多复杂的东西：不支持 CRI、代码耦合度较高、扩展性也不好……

它有这样那样的缺点，但无所谓，它带着我窥见了容器实现的一角（目前只是用到了 Namespace 和 Cgroups，更进一步还是得看看 Linux 的源码），在实现它的过程中我觉得很有趣。

本项目在很大程度上参考了《自己动手写 Docker》一书，用来入门很不错。但毕竟容器和 Go 的生态迭代得太快了，里面还是有很多东西与现在主流的技术不一致，因此做了一定的修改：
1. 将 vendor 模式改成了 module 模式。
2. 将 UnionFS 从 AUFS 替换为 OverlayFS
3. 部分地方在写法上进行了一定的优化

mydocker 支持的命令我贴在下方，用作记录：

```text
USAGE:
   mydocker [global options] command [command options] [arguments...]

COMMANDS:
   init     Init container process run user's process in container. Do not call it outside
   run      Create a container with namespace and cgroups limit
              mydocker run -it/-d [-name containerName] imageName command [arg...]
   commit   commit container to image, e.g. mydocker commit 123456789 myimage
   ps       list all the containers
   logs     print logs of a container
   exec     exec a command in container, e.g. mydocker exec 123456789 /bin/sh
   stop     stop a container
   rm       remove a container, e.g. mydocker rm 1234567890
   network  container network commands
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h  show help
```