# Make sure to have all keys with mode 600 at harmony-benchmark directory.
IFS=$'\n'
rm -rf ./tmp
mkdir tmp
for address in $(cat ./leader_addresses.txt)
do
    echo "trying to download from address $address"
    mkdir -p tmp/$address
    scp -r -o "StrictHostKeyChecking no" -i ../keys/california-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/frankfurt-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/ireland-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/ohio-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/oregon-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/singapore-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/tokyo-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
    scp -r -o "StrictHostKeyChecking no" -i ../keys/virginia-key-benchmark.pem ec2-user@$address:/home/tmp_log/* ./tmp/$address/
done