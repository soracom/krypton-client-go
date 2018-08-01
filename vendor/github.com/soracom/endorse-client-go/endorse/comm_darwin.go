// +build darwin

package endorse

import (
	"os/exec"
	"regexp"
)

var regexpIregIOCalloutDevice = regexp.MustCompile(`"IOCalloutDevice" = "(.*)"`)

func listCOMPorts() ([]string, error) {
	out, err := exec.Command("ioreg", "-l").Output()
	if err != nil {
		return nil, err
	}

	sm := regexpIregIOCalloutDevice.FindAllStringSubmatch(string(out), -1)

	result := make([]string, 0, len(sm))
	for _, m := range sm {
		if len(m) < 2 {
			continue
		}
		result = append(result, m[1])
	}
	return result, nil
}
