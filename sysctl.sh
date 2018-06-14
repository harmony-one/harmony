# System kernel setup (Mac)
# Reference: http://www.macfreek.nl/memory/Kernel_Configuration
sudo sysctl -w kern.ipc.somaxconn=10240      # Limit of number of new connections
#sudo sysctl -w kern.ipc.maxsockets=10240       # Initial number of sockets in memory
sudo sysctl -w net.inet.tcp.msl=1000       # TIME_WAIT
#sudo sysctl -w net.inet.tcp.rfc1323=1          # Enable TCP window scaling
sudo sysctl -w kern.ipc.maxsockbuf=4194304   # Maximum TCP Window size
sudo sysctl -w net.inet.tcp.sendspace=131072     # Default send buffer
sudo sysctl -w net.inet.tcp.recvspace=358400     # Default receive buffer

# The commented suffix is for linux
# Reference: https://github.com/Zilliqa/Zilliqa/blob/master/tests/Node/test_node_simple.sh
#sudo sysctl net.core.somaxconn=1024
#sudo sysctl net.core.netdev_max_backlog=65536;
#sudo sysctl net.ipv4.tcp_tw_reuse=1;
#sudo sysctl -w net.ipv4.tcp_rmem='65536 873800 1534217728';
#sudo sysctl -w net.ipv4.tcp_wmem='65536 873800 1534217728';
#sudo sysctl -w net.ipv4.tcp_mem='65536 873800 1534217728';