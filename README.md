# Ethr [![Build Status](https://travis-ci.org/Microsoft/ethr.svg?branch=master)](https://travis-ci.org/Microsoft/ethr)

Ethr is a cross platform network performance measurement tool written in golang. The goal of this project is to provide a native tool for comprehensive network performance measurements of bandwidth, connections/s, packets/s, latency, loss & jitter, across multiple protocols such as TCP, UDP, HTTP, HTTPS, and across multiple platforms such as Windows, Linux and other Unix systems.

<p align="center">
  <img alt="Ethr server in action" src="https://user-images.githubusercontent.com/44273634/49815752-506f0000-fd21-11e8-954e-d587e79c5d85.png">
</p>

Ethr takes inspiration from existing open source network performance tools and builds upon those ideas. For Bandwidth measurement, it is similar to iPerf3, for TCP & UDP traffic. iPerf3 has many more options for doing such as throttled testing, richer feature set, while Ethr has support for multiple threads, that allows it to scale to 1024 or even higher number of connections, multiple clients communication to a single server etc. For latency measurements, it is similar to latte on Windows or sockperf on Linux.

Ethr provides more test measurements as compared to other tools, e.g. it provides measurements for bandwidth, connections/s, packets/s, latency, and TCP connection setup latency, all in a single tool. In the future, there are plans to add more features (hoping for others to contribute) as well as more protocol support to make it a comprehensive tool for network performance measurements.

Ethr is natively cross platform, thanks to golang, as compared to compiling via an abstraction layer like cygwin that may limit functionality. It hopes to unify performance measurement by combining the functionality of tools like iPerf3, ntttcp, psping, sockperf, and latte and offering a single tool across multiple platforms and multiple protocols.

# Installation

## Download

https://github.com/Microsoft/ethr/releases/latest

**Linux**
```
wget https://github.com/microsoft/ethr/releases/latest/download/ethr_linux.zip
unzip ethr_linux.zip
```

**Windows Powershell**
```
wget https://github.com/microsoft/ethr/releases/latest/download/ethr_windows.zip -OutFile ethr_windows.zip
Expand-Archive .\ethr_windows.zip -DestinationPath .
```

**OSX**
```
wget https://github.com/microsoft/ethr/releases/latest/download/ethr_osx.zip
unzip ethr_osx.zip
```

## Building from Source

Note: go version 1.11 or higher is required building it from the source.

We use go-module to manage Ethr dependencies. for more information please check [how to use go-modules!](https://github.com/golang/go/wiki/Modules#how-to-use-modules)

```
git clone https://github.com/Microsoft/ethr.git
cd ethr
go build
```

If ethr is cloned inside of the `$GOPATH/src` tree, please make sure you invoke the `go` command with `GO111MODULE=on`!

## Docker

Build image using command: 
```
docker build -t microsoft/ethr .
```

Make binary:

**Linux**
```
docker run -e GOOS=linux -v $(pwd):/out microsoft/ethr make build-docker
```

**Windows**

```
docker run -e BINARY_NAME=ethr.exe -e GOOS=windows -v $(pwd):/out microsoft/ethr make build-docker
```

**OS X**
```
docker run -e BINARY_NAME=ethr -e GOOS=darwin -v $(pwd):/out microsoft/ethr make build-docker
```

## Using go get

```
go get github.com/microsoft/ethr
```

## Using ArchLinux AUR

Assuming you are using [`yay`](https://aur.archlinux.org/packages/yay/) (https://github.com/Jguer/yay):

```
yay -S ethr
```
# Publishing Nuget package
Follow the topic Building from Source to build ethr.exe

Modify ethr.nuspec to add new release version
```
vim ethr.nuspec
```
Create a nuget package(like Ethr.0.2.1.nupkg)
```
nuget.exe pack ethr.nuspec
```
Upload the package to nuget.org.

# Usage

## Simple Usage
Help:
```
ethr -h
```

Server:
```
ethr -s
```

Server with Text UI:
```
ethr -s -ui
```

Client:
```
ethr -c <server ip>
```

Examples:
```
// Start server
ethr -s

// Start client for default (bandwidth) test measurement using 1 thread
ethr -c localhost

// Start bandwidth test using 8 threads
ethr -c localhost -n 8

// Start connections/s test using 64 threads to server 10.1.0.11
ethr -c 10.1.0.11 -t c -n 64

// Run Ethr server on port 9999
./ethr -s -port 9999

// Measure TCP connection setup latency to ethr server on port 9999
// Assuming Ethr server is running on server with IP address: 10.1.1.100
./ethr -c 10.1.1.100 -p tcp -t pi -d 0 -4 -port 9999

// Measure TCP connection setup latency to www.github.com at port 443
./ethr -x www.github.com:443 -p tcp -t pi -d 0 -4

// Measure TCP connection setup latency to www.github.com at port 443
// Note: Here port 443 is driven automatically from https
./ethr -x https://www.github.com -p tcp -t pi -d 0 -4

// Measure ICMP ping latency to www.github.com
sudo ./ethr -x www.github.com -p icmp -t pi -d 0 -4

// Run measurement similar to mtr on Linux
sudo ./ethr -x www.github.com -p icmp -t mtr -d 0 -4

// Measure packets/s over UDP by sending small 1-byte packets
./ethr -c 172.28.192.1 -p udp -t p -d 0
```

## Known Issues & Requirements
### Windows
For ICMP related tests, Ping, TraceRoute, MyTraceRoute, Windows requires ICMP to be allowed via Firewall. This can be done using PowerShell by following commands. However, use this only if security policy of your setup allows that.
```
// Allow ICMP packets via Firewall for IPv4
New-NetFirewallRule -DisplayName "ICMP_Allow_Any" -Direction Inbound -Protocol ICMPv4 -IcmpType Any -Action Allow  -Profile Any -RemotePort Any

// Allow ICMP packets via Firewall for IPv6
New-NetFirewallRule -DisplayName "ICMPV6_Allow_Any" -Direction Inbound -Protocol ICMPv6 -IcmpType Any -Action Allow  -Profile Any -RemotePort Any
```
In addition, for TCP based TraceRoute and MyTraceRoute, Administrator mode is required, otherwise Ethr won't be able to receive ICMP TTL exceeded messages.
### Linux
For ICMP Ping, ICMP/TCP TraceRoute and MyTraceRoute, privileged mode is required via sudo.

## Complete Command Line
### Common Parameters
```
	-h 
		Help
	-no 
		Disable logging to file. Logging to file is enabled by default.
	-o <filename>
		Name of log file. By default, following file names are used:
		Server mode: 'ethrs.log'
		Client mode: 'ethrc.log'
	-debug 
		Enable debug information in logging output.
	-4 
		Use only IP v4 version
	-6 
		Use only IP v6 version
```
### Server Mode Parameters
```
In this mode, Ethr runs as a server, allowing multiple clients to run
performance tests against it.
	-s 
		Run in server mode.
	-ip <string>
		Bind to specified local IP address for TCP & UDP tests.
		This must be a valid IPv4 or IPv6 address.
		Default: <empty> - Any IP
	-port <number>
		Use specified port number for TCP & UDP tests.
		Default: 8888
	-ui 
		Show output in text UI.
```
### Client Mode Parameters
```
In this mode, Ethr client can only talk to an Ethr server.
	-c <server>
		Run in client mode and connect to <server>.
		Server is specified using name, FQDN or IP address.
	-b <rate>
		Transmit only Bits per second (format: <num>[K | M | G])
		Only valid for Bandwidth tests. Default: 0 - Unlimited
		Examples: 100 (100bits/s), 1M (1Mbits/s).
	-cport <number>
		Use specified local port number in client for TCP & UDP tests.
		Default: 0 - Ephemeral Port
	-d <duration>
		Duration for the test (format: <num>[ms | s | m | h]
		0: Run forever
		Default: 10s
	-g <gap>
		Time interval between successive measurements (format: <num>[ms | s | m | h]
		Only valid for latency, ping and traceRoute tests.
		0: No gap
		Default: 1s
	-i <iterations>
		Number of round trip iterations for each latency measurement.
		Only valid for latency testing.
		Default: 1000
	-ip <string>
		Bind to specified local IP address for TCP & UDP tests.
		This must be a valid IPv4 or IPv6 address.
		Default: <empty> - Any IP
	-l <length>
		Length of buffer to use (format: <num>[KB | MB | GB])
		Only valid for Bandwidth tests. Max 1GB.
		Default: 16KB
	-n <number>
		Number of Parallel Sessions (and Threads).
		0: Equal to number of CPUs
		Default: 1
	-p <protocol>
		Protocol ("tcp", "udp", "http", "https", or "icmp")
		Default: tcp
	-port <number>
		Use specified port number for TCP & UDP tests.
		Default: 8888
	-r 
		For Bandwidth tests, send data from server to client.
	-t <test>
		Test to run ("b", "c", "p", "l", "cl" or "tr")
		b: Bandwidth
		c: Connections/s
		p: Packets/s
		l: Latency, Loss & Jitter
		pi: Ping Loss & Latency
		tr: TraceRoute
		mtr: MyTraceRoute with Loss & Latency
		Default: b - Bandwidth measurement.
	-tos 
		Specifies 8-bit value to use in IPv4 TOS field or IPv6 Traffic Class field.
	-w <number>
		Use specified number of iterations for warmup.
		Default: 1
	-T <string>
		Use the given title in log files for logging results.
		Default: <empty>		
```
### External Mode Parameters
```
In this mode, Ethr talks to a non-Ethr server. This mode supports only a
few types of measurements, such as Ping, Connections/s and TraceRoute.
	-x <destination>
		Run in external client mode and connect to <destination>.
		<destination> is specified in URL or Host:Port format.
		For URL, if port is not specified, it is assumed to be 80 for http and 443 for https.
		Example: For TCP - www.microsoft.com:443 or 10.1.0.4:22 or https://www.github.com
		         For ICMP - www.microsoft.com or 10.1.0.4
	-cport <number>
		Use specified local port number in client for TCP & UDP tests.
		Default: 0 - Ephemeral Port
	-d <duration>
		Duration for the test (format: <num>[ms | s | m | h]
		0: Run forever
		Default: 10s
	-g <gap>
		Time interval between successive measurements (format: <num>[ms | s | m | h]
		Only valid for latency, ping and traceRoute tests.
		0: No gap
		Default: 1s
	-ip <string>
		Bind to specified local IP address for TCP & UDP tests.
		This must be a valid IPv4 or IPv6 address.
		Default: <empty> - Any IP
	-n <number>
		Number of Parallel Sessions (and Threads).
		0: Equal to number of CPUs
		Default: 1
	-p <protocol>
		Protocol ("tcp", or "icmp")
		Default: tcp
	-t <test>
		Test to run ("c", "cl", or "tr")
		c: Connections/s
		pi: Ping Loss & Latency
		tr: TraceRoute
		mtr: MyTraceRoute with Loss & Latency
		Default: pi - Ping Loss & Latency.
	-tos 
		Specifies 8-bit value to use in IPv4 TOS field or IPv6 Traffic Class field.
	-w <number>
		Use specified number of iterations for warmup.
		Default: 1
	-T <string>
		Use the given title in log files for logging results.
		Default: <empty>		
```

# Status

Protocol  | Bandwidth | Connections/s | Packets/s | Latency | Ping | TraceRoute | MyTraceRoute
------------- | ------------- | ------------- | ------------- | ------------- | ------------- | ------------- | -------------
TCP  | Yes | Yes | NA | Yes | Yes | Yes | Yes
UDP  | Yes | NA | Yes | No | NA | No | No
ICMP | No | NA | NA | NA | Yes | Yes | Yes

# Platform Support

**Windows**

Tested: Windows 10, Windows 7 SP1

Untested: Other Windows versions

**Linux**

Tested: Ubuntu Linux 18.04.1 LTS, OpenSuse Leap 15

Untested: Other Linux versions

**OSX**

Tested: OSX is tested by contributors

**Other**

No other platforms are tested at this time

# Todo List

Todo list work items are shown below. Contributions are most welcome for these work items or any other features and bugfixes.

* Test Ethr on other Windows versions, other Linux versions, FreeBSD and other OS
* Support for UDP latency, TraceRoute and MyTraceRoute

# Contributing

This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.
