import boto3
s3 = boto3.client('s3')
bucket_name = 'first-try'
s3.create_bucket(Bucket=bucket_name,ACL='public-read-write')
response = s3.list_buckets()
buckets = [bucket['Name'] for bucket in response['Buckets']]
print("Bucket List: %s" % buckets)
filename='myfirst.txt'
s3.upload_file(filename, bucket_name, filename)