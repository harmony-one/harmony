cd ~/projects/src/harmony-benchmark/
# Compile
sudo go build -o bin/commander aws-experiment-launch/experiment/commander/main.go
cd bin
sudo cp /tmp/distribution_config.txt .
sudo cp /tmp/commander_info.txt .
# Take ip address
IP=`head -n 1 commander_info.txt`
# Run commander
sudo ./commander -ip $IP -port 9000 -config_file distribution_config.txt
