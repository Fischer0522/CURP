# CURP
This repository is a simple implementation of CURP. I implemented it using a client-server based structure, but currently, this implementation doesn't support crash recovery. You can find more details in this paper: [CURP paper](https://www.usenix.org/system/files/nsdi19-park.pdf).

This repository is just a simple implementation for learning purposes. Do not use it in any production project.

## How to run CURP
This repository is a client-server based implementation. You should launch the server first and then use the client to send requests.
### Server
If you just want to run the server on a single machine, you can follow these steps:
```bash
cd curp_server
go build -o server main.go
./server
```

By executing these commands, you can run three CURP servers on different ports (default: 8020, 8021, 8022) of a single machine.
If you want to run it in a real distributed environment, you can launch the server with the addrs parameter on different machines:
```bash
./server -s=true -addrs="http://192.168.0.204:7010,http://192.168.0.206:7010,http://192.168.0.203:7010" -id=0
```

### Client
You can launch the client in the same way by executing these commands:
```bash
cd curp_client
go build -o client main.go
./client
```
Parameters:
n: The number of clients, default: 1.
local: Launch the client in local mode. If not, the client will listen on a port and wait for requests from application clients. Currently, you can send requests using YCSB. The code can be found at:[](https://github.com/Fischer0522/go-ycsb) default: true

## Future Work
- Fix unit tests.
- Implement crash recovery.
- Create a command-line based client.
