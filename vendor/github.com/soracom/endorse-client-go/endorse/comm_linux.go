// +build linux

package endorse

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const (
	ttyDir       = "/sys/class/tty"
	PORT_UNKNOWN = 0
)

type serialInfo struct {
	_type           int
	line            int
	port            uint
	irq             int
	flags           int
	xmit_fifo_size  int
	custom_divisor  int
	baud_base       int
	close_delay     uint16
	io_type         byte
	reserved_char   [1]byte
	hub6            int
	closing_wait    uint16
	closing_wait2   uint16
	iomem_base      uintptr
	iomem_reg_shift uint16
	port_high       uint
	iomap_base      uint32
}

func listCOMPorts() ([]string, error) {
	fileInfos, err := ioutil.ReadDir(ttyDir)
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)
	for _, fi := range fileInfos {
		name := fi.Name()
		candidatePath := filepath.Join(ttyDir, name)
		stat, err := os.Lstat(candidatePath)
		if err != nil {
			log("lstat() failed on %s", candidatePath)
			continue
		}
		if stat.Mode()&os.ModeSymlink == 0 {
			candidatePath = filepath.Join(ttyDir, name, "device")
		}

		target, err := os.Readlink(candidatePath)
		if err != nil {
			log("readlink() failed on %s", candidatePath)
			continue
		}
		if strings.Contains(target, "virtual") {
			log("target path contains 'virtual': %s", target)
			continue
		}

		if strings.Contains(target, "serial8250") {
			fd, err := syscall.Open(target, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0600)
			if err != nil {
				log("open() failed on: %s", target)
				continue
			}
			defer syscall.Close(fd)

			var si serialInfo
			ret, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(syscall.TIOCGSERIAL), uintptr(unsafe.Pointer(&si)))
			if errno != 0 {
				log("ioctl() failed on %s, errno == %08x (%d)", target, errno, errno)
				continue
			}
			if ret != 0 {
				log("ioctl() failed on %s, ret == %08x (%d)", target, ret, ret)
				continue
			}

			if si._type == PORT_UNKNOWN {
				log("serial_struct.type == PORT_UNKNOWN")
				continue
			}
		}
		result = append(result, filepath.Join("/dev", name))
	}

	return result, nil
}
