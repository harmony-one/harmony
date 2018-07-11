# Change the commander address
if [ $# -eq 0 ]; then
    echo "Please provide ip address of the commander"
    exit 1
fi
ADDRESS=$1
mkdir -p ./tmp
scp -r -i "california-key-benchmark.pem" ec2-user@$ADDRESS:~/projects/src/harmony-benchmark/bin/upload ./tmp/
