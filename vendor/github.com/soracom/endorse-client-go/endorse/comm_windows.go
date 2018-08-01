// +build windows

package endorse

import (
	"golang.org/x/sys/windows/registry"
)

func listCOMPorts() ([]string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\\DEVICEMAP\\SERIALCOMM`, registry.QUERY_VALUE)
	if err != nil {
		return nil, err
	}
	defer k.Close()

	names, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, err
	}

	ports := make([]string, 0)
	for _, name := range names {
		v, _, err := k.GetStringValue(name)
		if err != nil {
			continue
		}

		ports = append(ports, v)
	}

	return ports, nil
}

/*
var (
	msports, _                      = syscall.LoadLibrary("msports.dll")
	procComDBOpen, _                = syscall.GetProcAddress(msports, "ComDBOpen")
	procComDBClose, _               = syscall.GetProcAddress(msports, "ComDBClose")
	procComDBGetCurrentPortUsage, _ = syscall.GetProcAddress(msports, "ComDBGetCurrentPortUsage")
)

const (
	ERROR_SUCCESS               = 0x00000000
	ERROR_ACCESS_DENIED         = 0x00000005
	HCOMDB_INVALID_HANDLE_VALUE = 0x0
	CDB_REPORT_BITS             = 0x0
	CDB_REPORT_BYTES            = 0x1
)

type comDB struct {
	handle uintptr
}

func openComDB() (*comDB, error) {
	var handle uintptr = 0
	ret, _, errno := syscall.Syscall(procComDBOpen, 1, uintptr(unsafe.Pointer(&handle)), 0, 0)
	if errno != 0 {
		return nil, errors.Errorf("ComDBOpen() failed. errno == %#08x (%d)", errno, errno)
	}

	if ret != ERROR_SUCCESS {
		return nil, errors.Errorf("ComDBOpen() failed. ret == %#08x (%d)", ret, ret)
	}

	return &comDB{
		handle: handle,
	}, nil
}

func (c *comDB) close() error {
	ret, _, errno := syscall.Syscall(procComDBClose, 1, c.handle, 0, 0)
	if errno != 0 {
		return errors.Errorf("ComDBClose() failed. errno == %#08x (%d)", errno, errno)
	}
	if ret != ERROR_SUCCESS {
		return errors.Errorf("ComDBClose() failed. ret == %#08x (%d)", ret, ret)
	}

	return nil
}

func (c *comDB) getCurrentPortUsage() ([]byte, error) {
	var max uint32
	ret, _, errno := syscall.Syscall6(procComDBGetCurrentPortUsage,
		5,
		c.handle,                      // HCOMDB HComDB
		uintptr(0),                    // PBYTE Buffer
		uintptr(0),                    // DWORD BufferSize
		uintptr(0),                    // ULONG ReportType
		uintptr(unsafe.Pointer(&max)), // LPDWORD MaxPortsReported
		uintptr(0),
	)
	if errno != 0 {
		return nil, errors.Errorf("ComDBGetCurrentPortUsage() failed. errno == %#8x (%d)", errno, errno)
	}
	if ret != ERROR_SUCCESS {
		return nil, errors.Errorf("ComDBGetCurrentPortUsage() failed. ret == %#8x (%d)", ret, ret)
	}

	buf := make([]byte, max)
	ret, _, errno = syscall.Syscall6(procComDBGetCurrentPortUsage,
		5,
		c.handle, // HCOMDB HComDB
		uintptr(unsafe.Pointer(&buf[0])), // PBYTE Buffer
		uintptr(len(buf)),                // DWORD BufferSize
		uintptr(CDB_REPORT_BYTES),        // ULONG ReportType
		uintptr(unsafe.Pointer(&max)),    // LPDWRD MaxPortsReported
		uintptr(0),
	)
	if errno != 0 {
		return nil, errors.Errorf("ComDBGetCurrentPortUsage() failed. errno == %#8x (%d)", errno, errno)
	}
	if ret != ERROR_SUCCESS {
		return nil, errors.Errorf("ComDBGetCurrentPortUsage() failed. ret == %#8x (%d)", ret, ret)
	}

	return buf, nil
}

func listCOMPorts() ([]string, error) {
	c, err := openComDB()
	if err != nil {
		return nil, err
	}

	usage, err := c.getCurrentPortUsage()
	if err != nil {
		return nil, err
	}

	result := make([]string, 0)
	for i, v := range usage {
		if v > 0 {
			result = append(result, fmt.Sprintf("COM%d", i))
		}
	}
	return result, nil
}
*/
