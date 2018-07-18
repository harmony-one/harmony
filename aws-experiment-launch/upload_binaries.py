import boto3
import os
GOHOME ='/Users/alok/Documents/goworkspace/'
s3 = boto3.client('s3')
bucket_name = 'unique-bucket-bin'
#response = s3.create_bucket(Bucket=bucket_name,ACL='public-read-write', CreateBucketConfiguration={'LocationConstraint': 'us-west-2'})
response = s3.list_buckets()
buckets = [bucket['Name'] for bucket in response['Buckets']]
print("Bucket List: %s" % buckets)
dirname = GOHOME + 'src/harmony-benchmark/bin/'
for myfile in os.listdir(dirname):
    with open('distribution_config.txt','r') as f:
        f = open(os.path.join(dirname,myfile))
        response = s3.put_object(ACL='public-read-write',Body=f.read(),Bucket=bucket_name,Key=myfile)
        print(response)
