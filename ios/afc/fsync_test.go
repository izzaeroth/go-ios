package afc

/*
import (
	"fmt"
	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/golog"
	"path"
	"testing"
)

const test_device_udid = "udid_here"

func TestConnection_Remove(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	err = conn.Remove("/DCIM/fsync.go")
	if err != nil {
		golog.Fatal("remove failed", "error", err)
	}
}

func TestConnection_RemoveAll(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	err = conn.RemoveAll("/DCIM/TestDir")
	if err != nil {
		golog.Fatal("remove failed", "error", err)
	}
}

func TestConnection_Mkdir(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	err = conn.MkDir("/DCIM/TestDir")
	if err != nil {
		golog.Fatal("mkdir failed", "error", err)
	}
}

func TestConnection_stat(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	si, err := conn.Stat("/DCIM/architecture_diagram.png")
	if err != nil {
		golog.Fatal("get Stat failed", "error", err)
	}
	golog.Info("stat result", "stat", si)
}

func TestConnection_listDir(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	flist, err := conn.listDir("/DCIM/")
	if err != nil {
		golog.Fatal("tree view failed", "error", err)
	}
	for _, v := range flist {
		fmt.Printf("path: %+v\n", v)
	}
}

func TestConnection_TreeView(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	err = conn.TreeView("/DCIM/", "", true)
	if err != nil {
		golog.Fatal("tree view failed", "error", err)
	}
}

func TestConnection_pullSingleFile(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	err = conn.PullSingleFile("/DCIM/architecture_diagram.png", "architecture_diagram.png")
	if err != nil {
		golog.Fatal("pull single file failed", "error", err)
	}
}

func TestConnection_Pull(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)

	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}
	srcPath := "/DCIM/"
	dstpath := "TempRecv"
	dstpath = path.Join(dstpath, srcPath)
	err = conn.Pull(srcPath, dstpath)
	if err != nil {
		golog.Fatal("pull failed", "error", err)
	}
}

func TestConnection_Push(t *testing.T) {
	deviceEnrty, _ := ios.GetDevice(test_device_udid)
	conn, err := New(deviceEnrty)
	if err != nil {
		golog.Fatal("connect service failed", "error", err)
	}

	srcPath := "fsync.go"
	dstpath := "/DCIM/"

	err = conn.Push(srcPath, dstpath)
	if err != nil {
		golog.Fatal("push failed", "error", err)
	}
}
*/
