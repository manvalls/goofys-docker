package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/docker/go-plugins-helpers/volume"
	"github.com/jacobsa/fuse"
	goofys "github.com/kahing/goofys/api"
)

const (
	socketAddress = "/run/docker/plugins/goofys.sock"
	catfsFolder   = "/var/lib/driver/catfs/"
	goofysFolder  = "/var/lib/driver/goofys/"
	cacheFolder   = "/var/lib/driver/cache/"
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

	return &volume.PathResponse{}, nil
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
			catfsFolder + r.Name,
			goofysFolder + r.Name,
			cacheFolder + r.Name,
		} {
			os.MkdirAll(path, 0777)
		}

		_, _, err := goofys.Mount(
			context.Background(),
			bucketNamespace+r.Name,
			&goofys.Config{
				MountOptions: map[string]string{"allow_other": ""},
				MountPoint:   goofysFolder + r.Name,
				Cache: []string{
					"-o",
					"allow_other",
					"--free",
					getEnv("CACHE_FREE", "10G"),
					"--",
					goofysFolder + r.Name,
					cacheFolder + r.Name,
					catfsFolder + r.Name,
				},

				DirMode:      os.FileMode(0770),
				FileMode:     os.FileMode(0770),
				Gid:          33,
				Uid:          33,
				StatCacheTTL: 1 * time.Minute,
				TypeCacheTTL: 1 * time.Minute,

				StorageClass: "STANDARD",
				Region:       os.Getenv("AWS_REGION"),
				Endpoint:     os.Getenv("AWS_ENDPOINT"),
				Foreground:   true,
			},
		)

		if err != nil {
			return &volume.MountResponse{}, err
		}

	}

	return &volume.MountResponse{
		Mountpoint: catfsFolder + r.Name,
	}, nil
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
			catfsFolder + r.Name,
			goofysFolder + r.Name,
		} {
			fuse.Unmount(path)
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
			Name: r.Name,
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
				Name: strings.TrimPrefix(*v.Name, bucketNamespace),
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
