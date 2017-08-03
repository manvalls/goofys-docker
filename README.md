[![license](https://img.shields.io/github/license/monder/goofys-docker.svg?maxAge=2592000&style=flat-square)]()
[![GitHub tag](https://img.shields.io/github/tag/monder/goofys-docker.svg?style=flat-square)]()

goofys-docker is a docker [volume plugin] wrapper for S3

## Overview

The initial idea behind mounting s3 buckets as docker volumes is to provide store for configs and secrets. The volume as per [goofys] does not have features like random-write support, unix permissions, caching.

## Getting started

### Requirements

The docker host should have [FUSE] support with `fusermount` cli utility in `$PATH`

### Building

There are prebuilt plugin(s) availble from docker hub, just run docker plugin install haibinfx/goofys.

If you need to build it yourself there is a build file `.\make.sh` that will build a plugin in your local machine.

For Windows Systems that are not Unix-like systems, run `.\make_in.sh` first, then log into the docker machine to run `.\make.sh`.

### Configuration

docker plugin set haibinfx/goofys http_proxy=http://192.168.100.1:3128 https_proxy=http://192.168.100.1:3128 \
    no_proxy=localhost,127.0.0.1,192.168.99.100,192.168.99.101,192.168.99.102 \
    AWS_ACCESS_KEY_ID=[AABBAKIACDDXIH2C Your ID] AWS_SECRET_ACCESS_KEY=[KDabzNs68Tjasdfasf5as Your Key]
docker plugin enable fx/goofys

The most simple way to configure aws credentials is to use [IAM roles] to access the bucket for the machine, [aws configuration file][AWS auth] or [ENV variables][AWS auth]. The credentials will be used for all buckets mounted by `goofys-docker`.

### Running

```
docker volume create --name=test-plugin --driver=haibinfx/goofys --opt bucket=my-backup-name --opt prefix=path_under_bucket \
--opt debugs3=1 --opt gid=50 --opt uid=1000 --opt dir_mode=0666 --opt file_mode=0666
docker run -it --rm -v test-plugin:/mnt/test:ro busybox sh

#### Options

* `bucket` - Optional S3 bucket name. The default bucket is the volume name.
* `prefix` - Optional S3 prefix path.
* `region` - Optional AWS region (default is "us-east-1" and will be auto detected).
* `debugs3` - Optional S3 debug logs (default is 0).
* `endpoint` - Optional S3 Service endpoint (default is auto detected).
* `profile` - Optional AWS profile.

Create a new volume by issuing a docker volume command:
```
docker volume create --name=test-docker-goofys --driver=haibinfx/goofys region=eu-west-1
```
That will create a volume connected to `test-docker-goofys` bucket. The region of the bucket will be autodetected.

Nothing is mounted yet.

Launch the container with `test-docker-goofys` volume mounted in `/home` inside the container.
```
docker run -it --rm -v test-docker-goofys:/home:ro -it busybox sh
/ # cat /home/test
test file content
/ # ^D
```

Pass the bucket name as an option instead of the default volume name value:
```
docker volume create --name=vol1 --driver=haibinfx/goofys --opt bucket=test-docker-goofys --opt region=eu-west-1
docker run -it --rm -v vol1:/home:ro -it busybox sh
/ # cat /home/test
test file content
/ # ^D
```

It is also possible to mount a subfolder:
```
docker volume create --name=vol1 --driver=haibinfx/goofys --opt prefix=folder region=eu-west-1
docker run -it --rm -v vol1:/home:ro -it busybox sh
/ # cat /home/test
test file content from folder
/ # ^D
```

If multiple folders are mounted for the single bucket on the same machine, only 1 fuse mount will be created. The mount will be shared by docker containers. It will be unmouned when there be no containers to use it.

## License
MIT

[goofys]: https://github.com/kahing/goofys
[volume plugin]: https://docs.docker.com/engine/extend/plugins_volume/
[FUSE]: https://github.com/libfuse/libfuse
[download]: https://github.com/monder/goofys-docker/releases
[AWS auth]: http://docs.aws.amazon.com/sdk-for-go/api/#Configuring_Credentials
[IAM roles]: http://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2.html
