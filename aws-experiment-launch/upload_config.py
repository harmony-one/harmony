import boto3
import os
#from boto3.session import Session
s3 = boto3.client('s3')
bucket_name = 'unique-bucket-bin'
#response = s3.create_bucket(Bucket=bucket_name,ACL='public-read-write', CreateBucketConfiguration={'LocationConstraint': 'us-west-2'})
myfile = 'distribution_config.txt'
with open('distribution_config.txt','r') as f:
    response  = s3.put_object(ACL='public-read-write',Body=f.read(),Bucket=bucket_name,Key=myfile)
    print(response)
