go build -o server main.go
RDMA
./server -s=true -addrs="http://192.168.0.204:7010,http://192.168.0.206:7010,http://192.168.0.203:7010" -id=0

./server -s=true -addrs="http://10.26.42.218:7010,http://10.26.42.215:7010,http://10.26.42.217:7010" -id=0