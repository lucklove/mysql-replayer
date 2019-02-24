```
 _____ ______   ________  _______   ________  ___       ________      ___    ___ _______   ________     
|\   _ \  _   \|\   __  \|\  ___ \ |\   __  \|\  \     |\   __  \    |\  \  /  /|\  ___ \ |\   __  \    
\ \  \\\__\ \  \ \  \|\  \ \   __/|\ \  \|\  \ \  \    \ \  \|\  \   \ \  \/  / | \   __/|\ \  \|\  \   
 \ \  \\|__| \  \ \   _  _\ \  \_|/_\ \   ____\ \  \    \ \   __  \   \ \    / / \ \  \_|/_\ \   _  _\  
  \ \  \    \ \  \ \  \\  \\ \  \_|\ \ \  \___|\ \  \____\ \  \ \  \   \/  /  /   \ \  \_|\ \ \  \\  \| 
   \ \__\    \ \__\ \__\\ _\\ \_______\ \__\    \ \_______\ \__\ \__\__/  / /      \ \_______\ \__\\ _\ 
    \|__|     \|__|\|__|\|__|\|_______|\|__|     \|_______|\|__|\|__|\___/ /        \|_______|\|__|\|__|
                                                                    \|___|/                             
```
## 作用
对测试环境数据库重放录制的MySQL请求

## 录制方式
该工具读取tcpdump的产物，因此需要使用tcpdump进行流量录制，命令如下：
```sh
tcpdump -s 0 -i eth0 dst port 4000 and tcp -w /run/test.pcap
```
其中`-s 0`表示捕获整个数据包，`-i eth0`表示在eth0网卡上录制数据，`dst port 4000 and tcp`表示捕获目标端口为`4000`的TCP数据包，取决于线上数据库在哪个端口监听，然后`-w /run/test/pcap`表示将录制的数据写到`/run/test.pcap`

如果记不住这么多参数又经常需要录制的话，可以把上述命令写到一个脚本里`record.sh`:
```sh
#! /bin/sh

tcpdump -s 0 -i eth0 dst port 4000 and tcp -w /run/test.pcap
```
这就是我们录制流量的小工具啦！当然你可以对脚本内容做一些小调整让它更高效，比如加上-B参数来避免突发流量时tcpdump处理不过来造成的丢包

## 回放方式
### 预处理
要回放流量，需要先解析tcpdump的录制结果，假定录制产物为`/run/test.pcap`，需要先这样处理一下:
```sh
mysql-replayer prepare -i /run/test.pcap -o /tmp/test
```
这会对pcap文件进行分析，并将分析结果放在/tmp/test目录里（如果`/tmp/test`目录不存在则自动创建，若为非目录则报错），分析之后输出产物为一系列文件，类似这样：
```
1550990789-194.32.77.196-56598-498081.rec
1550990789-194.32.77.196-57616-137694.rec
1550990790-194.32.77.196-57490-118496.rec
1550990789-194.32.77.196-56600-727887.rec
1550990789-194.32.77.196-57618-167221.rec
1550990792-194.32.77.196-57266-62854.rec
1550990789-194.32.77.196-56602-131847.rec
1550990789-194.32.77.196-57620-586574.rec  
1550990792-194.32.77.196-57268-60294.rec
```
文件名字组成为`{TCP握手时间}-{客户端ip}-{客户端端口}-{随机数}.rec`
### 回放
回放命令文档:
```
mysql-replayer bench -i input-dir [-h host] [-P port] [-u user] [-p passwd] [-s speed] [-c concurrent]:
        Bench mysql server with data from input-dir.
  -P string
        port number to use for connection (default "4000")
  -c int
        the bench concurrent, 0 or negative number means dynamic concurrent
  -h string
        connect to host (default "127.0.0.1")
  -i string
        the directory contains bench data
  -p string
        password to use when connecting to server
  -s int
        the bench speed (default 1)
  -u string
        user for login (default "root")
```
假设上一步将pcap文件处理到目录`/tmp/test`，测试数据库用户为`root`，数据库监听本地端口`4000`，设置并发为`200`，速度为录制时实际速度的`2`倍（实测结果这个并不太准），那么实际的命令为:
```sh
mysql-replayer bench -i /tmp/test  -c 200 -s 2 # 用户和端口由于是默认值不用填
```
如果不使用参数`-c`，它将使用默认值`0`，这表示根据录制流量时的实际情况动态调整并发（比如录制的时候为`200`过了一会儿增至`1000`，那么这里也会自动从`200`增至`1000`）

### 结果
录制20s sysbench压测流量（768并发），sysbench实际输出为:
```
[ 10s ] thds: 768 tps: 0.00 qps: 260.12 (r/w/o: 213.19/0.00/46.93) lat (ms,95%): 0.00 err/s: 0.00 reconn/s: 0.00
[ 20s ] thds: 768 tps: 0.00 qps: 243.40 (r/w/o: 225.36/0.00/18.04) lat (ms,95%): 0.00 err/s: 0.00 reconn/s: 0.00
```
以录制的结果为数据源使用mysql-replayer回放结果:
```
[root@vps mysql-replayer]# ./mysql-replayer bench -i test  -c 500
Processing...
Process 6273 querys in 32 seconds, QPS: 196
[root@vps mysql-replayer]# ./mysql-replayer bench -i test -c 500 -s 10
Processing...
Process 6273 querys in 8 seconds, QPS: 784
[root@vps mysql-replayer]# ./mysql-replayer bench -i test -c 2000 -s 10
Processing...
Process 6273 querys in 5 seconds, QPS: 1254
```
实际结果取决于当时的网络环境