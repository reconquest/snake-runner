// +build !windows

package runner

import (
	"io/ioutil"
	"strings"

	"github.com/reconquest/pkg/log"
)

func IsDocker() bool {
	contents, err := ioutil.ReadFile("/proc/1/cgroup")
	if err != nil {
		log.Errorf(err, "unable to read /proc/1/cgroup to determine "+
			"is it docker container or not")
	}

	/**
	* A docker container has /docker/ in its /cgroup file
	*
	* / # cat /proc/1/cgroup | grep docker
	* 11:pids:/docker/14f3db3a669169c0b801a3ac99...
	* 10:freezer:/docker/14f3db3a669169c0b801a3ac9...
	* 9:cpu,cpuacct:/docker/14f3db3a669169c0b801a3ac...
	* 8:hugetlb:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 7:perf_event:/docker/14f3db3a669169c0b801a3...
	* 6:devices:/docker/14f3db3a669169c0b801a3ac99f...
	* 5:memory:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 4:blkio:/docker/14f3db3a669169c0b801a3ac99f89e914...
	* 3:cpuset:/docker/14f3db3a669169c0b801a3ac99f89e914a...
	* 2:net_cls,net_prio:/docker/14f3db3a669169c0b801a3ac...
	* 1:name=systemd:/docker/14f3db3a669169c0b801a3ac99f89e...
	* 0::/system.slice/docker.service
	***/
	if strings.Contains(string(contents), "/docker/") {
		return true
	}

	return false
}
