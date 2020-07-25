#!/bin/sh
/usr/bin/mc config host rm myminio
/usr/bin/mc config host add --quiet --api s3v4 myminio http://minio:9000 minio minio123
/usr/bin/mc mb --quiet myminio/testbucket/
/usr/bin/mc policy set public myminio/testbucket

while :
do
	sleep 1
done