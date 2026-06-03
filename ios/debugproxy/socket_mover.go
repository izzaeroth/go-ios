package debugproxy

import (
	"fmt"
	"os"

	"github.com/danielpaulus/go-ios/ios/golog"
	"github.com/google/uuid"
)

var realSocketSuffix = fmt.Sprintf(".%s.real_socket", uuid.New().String())

func MoveSock(socket string) (string, error) {
	newLocation := socket + realSocketSuffix
	if fileExists(newLocation) {
		return "", fmt.Errorf("there is already a file named: %s please remove it or restore original usbmuxd before starting the proxy", newLocation)
	}
	golog.Info("moving socket", "from", socket, "to", newLocation)
	err := os.Rename(socket, newLocation)
	return newLocation, err
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func MoveBack(socket string) error {
	newLocation := socket + realSocketSuffix
	golog.Info("checking if socket exists", "socket", newLocation)
	if !fileExists(newLocation) {
		golog.Info("socket does not exist, doing nothing", "socket", newLocation)
		return nil
	}
	golog.Info("found socket, deleting fake socket", "socket", newLocation, "fakeSocket", socket)

	golog.Info("deleting fake socket", "socket", socket)
	err := os.Remove(socket)
	if err != nil {
		golog.Warn("failed deleting socket", "socket", socket, "error", err)
	}
	golog.Info("moving back socket", "from", newLocation, "to", socket)
	err = os.Rename(newLocation, socket)
	return err
}
