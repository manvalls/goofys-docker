package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/docker/go-plugins-helpers/volume"
)

const (
	socketAddress = "/run/docker/plugins/goofys.sock"
	catfsFolder   = "/mnt/catfs/volumes/"
	goofysFolder  = "/mnt/goofys/volumes/"
	cacheFolder   = "/mnt/cache/volumes/"
)

var (
	bucketNamespace = getEnv("AWS_NAMESPACE", "goofys_")
)

var (
	errNotFound   = errors.New("Volume not found")
	errVolumeUsed = errors.New("Volume in use")
)

type s3Driver struct {
	*sync.Mutex
	*s3.S3
	connections map[string]uint
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func (d *s3Driver) Create(r *volume.CreateRequest) error {
	d.Lock()
	defer d.Unlock()

	_, err := d.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketNamespace + r.Name),
	})

	if aerr, ok := err.(awserr.Error); ok && aerr.Code() == s3.ErrCodeBucketAlreadyOwnedByYou {
		return nil
	}

	return err
}

func (d *s3Driver) Remove(r *volume.RemoveRequest) error {
	d.Lock()
	defer d.Unlock()

	_, err := d.DeleteBucket(&s3.DeleteBucketInput{
		Bucket: aws.String(bucketNamespace + r.Name),
	})

	return err
}

func (d *s3Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error) {
	d.Lock()
	defer d.Unlock()

	_, err := d.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(bucketNamespace + r.Name),
	})

	if err != nil {
		return &volume.PathResponse{}, err
	}

	return &volume.PathResponse{
		Mountpoint: catfsFolder + bucketNamespace + r.Name,
	}, nil
}

func (d *s3Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	d.Lock()
	defer d.Unlock()

	c, ok := d.connections[r.Name]

	if ok {
		c++
	} else {
		c = 1
	}

	d.connections[r.Name] = c

	if c == 1 {

		for _, path := range []string{
			catfsFolder + bucketNamespace + r.Name,
			goofysFolder + bucketNamespace + r.Name,
			cacheFolder + bucketNamespace + r.Name,
		} {
			os.MkdirAll(path, 0777)
		}

		goofys := exec.Command(
			"goofys",
			"-o",
			"allow_other",
			"--dir-mode",
			"0777",
			"--file-mode",
			"0777",
			"--uid",
			"33",
			"--gid",
			"33",
			"--endpoint",
			os.Getenv("AWS_ENDPOINT"),
			"--region",
			os.Getenv("AWS_REGION"),
			bucketNamespace+r.Name,
			goofysFolder+bucketNamespace+r.Name,
		)

		err := goofys.Start()

		if err != nil {
			return &volume.MountResponse{}, err
		}

		goofys.Stdout = os.Stdout
		goofys.Stderr = os.Stderr

		err = exec.Command(
			"catfs",
			"-o",
			"allow_other",
			"--free",
			getEnv("CACHE_FREE", "10G"),
			"--",
			goofysFolder+bucketNamespace+r.Name,
			cacheFolder+bucketNamespace+r.Name,
			catfsFolder+bucketNamespace+r.Name,
		).Start()

		if err != nil {
			return &volume.MountResponse{}, err
		}

	}

	return &volume.MountResponse{}, nil
}

func (d *s3Driver) Unmount(r *volume.UnmountRequest) error {
	d.Lock()
	defer d.Unlock()

	c, ok := d.connections[r.Name]

	if ok {
		if c == 1 {
			delete(d.connections, r.Name)
			c = 0
		} else {
			c--
			d.connections[r.Name] = c
		}
	} else {
		c = 0
	}

	if c == 0 {

		for _, path := range []string{
			catfsFolder + bucketNamespace + r.Name,
			goofysFolder + bucketNamespace + r.Name,
		} {
			fuseUnmount := exec.Command("fusermount", "-u", path)
			fuseUnmount.Start()
			fuseUnmount.Wait()
			os.RemoveAll(path)
		}

	}

	return nil
}

func (d *s3Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	d.Lock()
	defer d.Unlock()

	_, err := d.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(bucketNamespace + r.Name),
	})

	if err != nil {
		return &volume.GetResponse{}, err
	}

	return &volume.GetResponse{
		Volume: &volume.Volume{
			Name:       r.Name,
			Mountpoint: catfsFolder + bucketNamespace + r.Name,
		},
	}, nil
}

func (d *s3Driver) List() (*volume.ListResponse, error) {
	d.Lock()
	defer d.Unlock()

	result, err := d.S3.ListBuckets(&s3.ListBucketsInput{})

	if err != nil {
		return &volume.ListResponse{}, err
	}

	response := &volume.ListResponse{
		Volumes: make([]*volume.Volume, 0),
	}

	for _, v := range result.Buckets {
		if strings.HasPrefix(*v.Name, bucketNamespace) {
			response.Volumes = append(response.Volumes, &volume.Volume{
				Name:       strings.TrimPrefix(*v.Name, bucketNamespace),
				Mountpoint: catfsFolder + *v.Name,
			})
		}
	}

	return response, nil
}

func (d *s3Driver) Capabilities() *volume.CapabilitiesResponse {
	return &volume.CapabilitiesResponse{
		Capabilities: volume.Capability{
			Scope: "global",
		},
	}
}

func main() {

	for _, path := range []string{
		catfsFolder,
		goofysFolder,
	} {
		os.RemoveAll(path)
	}

	for _, path := range []string{
		catfsFolder,
		goofysFolder,
		cacheFolder,
	} {
		os.MkdirAll(path, 0777)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:   aws.String(os.Getenv("AWS_REGION")),
		Endpoint: aws.String(os.Getenv("AWS_ENDPOINT")),
		Credentials: credentials.NewStaticCredentials(
			os.Getenv("AWS_ACCESS_KEY_ID"),
			os.Getenv("AWS_SECRET_ACCESS_KEY"),
			os.Getenv("AWS_SESSION_TOKEN"),
		),
	}))

	d := &s3Driver{
		connections: make(map[string]uint),
		S3:          s3.New(sess),
		Mutex:       &sync.Mutex{},
	}

	h := volume.NewHandler(d)

	if err := h.ServeUnix(socketAddress, 0); err != nil {
		panic(err)
	}

}
