cd ~/projects/src/harmony-benchmark/
# Compile
sudo go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
cd bin
# Take ip address
IP=`head -n 1 commander_info.txt`
# Run commander
sudo ./bin/commander -ip $IP -port 9000 -config_file distribution_config.txt
