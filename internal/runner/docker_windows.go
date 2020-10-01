// +build windows

package runner

// https://hub.docker.com/_/microsoft-windows-base-os-images
// it is actually possible to have windows in docker,
// but for now we always say no
func IsDocker() bool {
	return false
}
