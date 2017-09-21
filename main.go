package main

import (
	"strconv"
	"sync"

	"fmt"
	"log"
	"os"
    "os/user"
	"io/ioutil"
	"encoding/json"
	"strings"

	"syscall"
	"time"

	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"

	goofys "github.com/kahing/goofys/api"

	"golang.org/x/net/context"
	"github.com/jacobsa/fuse"
)

type s3Volume struct {
	Bucket string
	Prefix string
	BucketName string

	connections int

	Config *(goofys.Config)
}

type s3Driver struct {
	sync.Mutex

	root        string
	statePath   string
	volumes     map[string]*s3Volume
}
const (
	socketAddress = "/run/docker/plugins/goofys.sock"
)

func newS3Driver(root string) (*s3Driver, error) {
	d := &s3Driver{
		root:        filepath.Join(root, "volumes"),
		statePath:   filepath.Join(root, "state", "goofys-state.json"),
		volumes:     map[string]*s3Volume{},
	}

	data, err := ioutil.ReadFile(d.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			logrus.WithField("statePath", d.statePath).Debug("no state found")
		} else {
			return nil, err
		}
	} else {
		if err := json.Unmarshal(data, &d.volumes); err != nil {
			return nil, err
		}
	}

	return d, nil
}

func (d *s3Driver) saveState() {
	data, err := json.Marshal(d.volumes)
	if err != nil {
		logrus.WithField("statePath", d.statePath).Error(err)
		return
	}

	if err := ioutil.WriteFile(d.statePath, data, 0644); err != nil {
		logrus.WithField("savestate", d.statePath).Error(err)
	}
}

func (d *s3Driver) Create(r *volume.CreateRequest) error {
	logrus.WithField("method", "create").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()
	v := &s3Volume{}

	var cacheArgs []string

	goofysConfig := &goofys.Config{
		MountOptions: map[string]string{"_netdev": "","allow_other":""},

		DirMode: os.FileMode(0755),
		FileMode: os.FileMode(0644),
		Gid: uint32(50),
		Uid: uint32(1000),
		StatCacheTTL: 1 * time.Minute,
		TypeCacheTTL: 1 * time.Minute,

		StorageClass: "STANDARD",
		Region: "us-east-1",
		Foreground: true,
	}

	for key, val := range r.Options {
		switch key {
		case "bucket":
			v.Bucket = val
			bkt := strings.Split(val,":")
			v.BucketName = bkt[0]
			if len(bkt) > 1 {
				v.Prefix = bkt[1]
			}
		case "bucket-name":
			v.BucketName = val
		case "prefix":
			v.Prefix = val

		case "dir-mode":
			if i, err := strconv.ParseUint(val,0,32); err == nil {
				goofysConfig.DirMode = os.FileMode(i)
			}
		case "file-mode":
			if i, err := strconv.ParseUint(val,0,32); err == nil {
				goofysConfig.FileMode = os.FileMode(i)
			}
		case "gid":
			if i, err := strconv.ParseUint(val,0,32); err == nil {
				goofysConfig.Gid = uint32(i)
			}
		case "uid":
			if i, err := strconv.ParseUint(val,0,32); err == nil {
				goofysConfig.Uid = uint32(i)
			}
		case "endpoint":
			goofysConfig.Endpoint = val
		case "region":
			goofysConfig.Region = val
		case "region-set":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.RegionSet = b
			}
		case "storage-class":
			goofysConfig.StorageClass = val
		case "profile":
			goofysConfig.Profile = val
		case "use-content-type":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.UseContentType = b
			}
		case "sse":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.UseSSE = b
			}
		case "sse-kms":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.UseKMS = b
			}
		case "kms-key-id":
			goofysConfig.KMSKeyID = val
		case "acl":
			goofysConfig.ACL = val

		case "cheap":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.Cheap = b
			}
		case "explicit-dir":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.ExplicitDir = b
			}
		case "state-cache-ttl":
			if t, err := time.ParseDuration(val); err == nil {
				goofysConfig.StatCacheTTL = t
			}
		case "type-cache-ttl":
			if t, err := time.ParseDuration(val); err == nil {
				goofysConfig.TypeCacheTTL = t
			}

		case "debug-fuse":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.DebugFuse = b
			}
		case "debug-s3":
			if b, err := strconv.ParseBool(val); err == nil {
				goofysConfig.DebugS3 = b
			}
		}
	}

	if v.BucketName == "" {
		return logError("BucketName was not found, bucket: %s, prefix: %s", v.Bucket, v.Prefix)
	}

	if strings.IndexByte(v.Bucket,':') == -1 && v.Prefix != "" {
		v.Bucket = v.BucketName + ":" + v.Prefix
	}

	goofysConfig.MountPoint = filepath.Join(d.root, r.Name)

	cacheDir :=filepath.Join("mnt", "cache", v.BucketName, v.Prefix)
	err := os.MkdirAll(cacheDir, 0700)
	if err !=nil {
		logrus.WithField("create", cacheDir).Error(err)
		return err
	}

	// cacheArgs = append([]string{"--test"}, cacheArgs...)
	cacheArgs = append(cacheArgs, "-ononempty")

	cacheArgs = append(cacheArgs, "--")
	cacheArgs = append(cacheArgs, goofysConfig.MountPoint)
	cacheArgs = append(cacheArgs, cacheDir)
	cacheArgs = append(cacheArgs, goofysConfig.MountPoint)
	goofysConfig.Cache = cacheArgs

	v.Config = goofysConfig
	d.volumes[r.Name] = v

	d.saveState()

	return nil
}

func (d *s3Driver) Remove(r *volume.RemoveRequest) error {
	logrus.WithField("method", "remove").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return logError("volume %s not found", r.Name)
	}

	if v.connections != 0 {
		return logError("volume %s is currently used by a container", r.Name)
	}
	if err := os.RemoveAll(v.Config.MountPoint); err != nil {
		return logError(err.Error())
	}
	delete(d.volumes, r.Name)
	d.saveState()
	return nil
}

func (d *s3Driver) Path(r *volume.PathRequest) (*volume.PathResponse, error)  {
	logrus.WithField("method", "path").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()


	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.PathResponse{}, logError("volume %s not found", r.Name)
	}

	return &volume.PathResponse{Mountpoint: v.Config.MountPoint}, nil
}

func (d *s3Driver) Mount(r *volume.MountRequest) (*volume.MountResponse, error) {
	logrus.WithField("method", "mount").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.MountResponse{}, logError("volume %s not found", r.Name)
	}

	if v.connections == 0 {
		fi, err := os.Lstat(v.Config.MountPoint)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(v.Config.MountPoint, 0755); err != nil {
				return &volume.MountResponse{}, logError(err.Error())
			}
		} else if err != nil {
			if e, ok := err.(*os.PathError); ok && e.Err == syscall.ENOTCONN {
				// Crashed previously? Unmount
				fuse.Unmount(v.Config.MountPoint)
			} else {
				return &volume.MountResponse{}, logError(err.Error())
			}
		}

		if fi != nil && !fi.IsDir() {
			return &volume.MountResponse{}, logError("%v already exist and it's not a directory", v.Config.MountPoint)
		}

		if err := d.mountVolume(v); err != nil {
			return &volume.MountResponse{}, logError(err.Error())
		}
	}

	v.connections++

	return &volume.MountResponse{Mountpoint: v.Config.MountPoint}, nil
}

func (d *s3Driver) Unmount(r *volume.UnmountRequest) error {
	logrus.WithField("method", "unmount").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()
	v, ok := d.volumes[r.Name]
	if !ok {
		return logError("volume %s not found", r.Name)
	}

	v.connections--

	if v.connections <= 0 {
		if err := d.unmountVolume(v.Config.MountPoint); err != nil {
			return logError(err.Error())
		}
		v.connections = 0
	}

	return nil
}

func (d *s3Driver) Get(r *volume.GetRequest) (*volume.GetResponse, error) {
	logrus.WithField("method", "get").Debugf("%#v", r)

	d.Lock()
	defer d.Unlock()

	v, ok := d.volumes[r.Name]
	if !ok {
		return &volume.GetResponse{}, logError("volume %s not found", r.Name)
	}

	return &volume.GetResponse{Volume: &volume.Volume{Name: r.Name, Mountpoint: v.Config.MountPoint}}, nil
}

func (d *s3Driver) List() (*volume.ListResponse, error) {
	logrus.WithField("method", "list").Debugf("")

	d.Lock()
	defer d.Unlock()

	var vols []*volume.Volume
	for name, v := range d.volumes {
		vols = append(vols, &volume.Volume{Name: name, Mountpoint: v.Config.MountPoint})
	}
	return &volume.ListResponse{Volumes: vols}, nil
}

func (d *s3Driver) Capabilities() *volume.CapabilitiesResponse {
	logrus.WithField("method", "capabilities").Debugf("")

	return &volume.CapabilitiesResponse{Capabilities: volume.Capability{Scope: "local"}}
}

func (d *s3Driver) mountVolume(v *s3Volume) error {
	// Mount the file system.
	var mfs *fuse.MountedFileSystem

	mfs, err := goofys.Mount(
		context.Background(),
		v.Bucket,
		v.Config)

	if err != nil {
		log.Fatalf("Mounting file system: %v", err)
		//kill(os.Getppid(), syscall.SIGUSR2)
		// fatal also terminates itself
	} else {
		//kill(os.Getppid(), syscall.SIGUSR1)
		log.Println("File system has been successfully mounted.")
		// Let the user unmount with Ctrl-C
		// (SIGINT). But if cache is on, catfs will
		// receive the signal and we would detect that exiting

		// Wait for the file system to be unmounted.
		err = mfs.Join(context.Background())
		if err != nil {
			err = fmt.Errorf("MountedFileSystem.Join: %v", err)
			return err
		}

		log.Println("Goofys mountVolume Successfully exited.")
	}
	return nil
}

func (d *s3Driver) unmountVolume(MountPoint string) error {
	return fuse.Unmount(MountPoint)
}

func logError(format string, args ...interface{}) error {
	logrus.Errorf(format, args...)
	return fmt.Errorf(format, args)
}

func main() {
	debug := os.Getenv("DEBUG")
	if ok, _ := strconv.ParseBool(debug); ok {
		logrus.SetLevel(logrus.DebugLevel)
	}

	d, err := newS3Driver("/mnt")
	if err !=nil {
		log.Fatal(err)
	}
	h := volume.NewHandler(d)
	u, _ := user.Lookup("root")
	gid, _ := strconv.Atoi(u.Gid)

	logrus.Infof("listening on %s", socketAddress)
	logrus.Error(h.ServeUnix(socketAddress, gid))
}
