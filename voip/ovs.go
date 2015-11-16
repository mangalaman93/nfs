package voip

import (
	"errors"
	"os/exec"
)

const (
	OVS_BRIDGE = "ovsbr"
	INET       = "173.16.1.1"
	NETMASK    = "255.255.255.0"
)

var (
	ErrOvsNotFound = errors.New("openvswitch is not installed")
)

func ovsInit() error {
	undo := false

	out, err := exec.Command("/bin/sh", "-c", "which ovs-vsctl").Output()
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	out, err = exec.Command("/bin/sh", "-c", "which ovs-docker").Output()
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	_, err = exec.Command("/bin/sh", "-c", "sudo ovs-vsctl br-exists "+OVS_BRIDGE).Output()
	if err != nil {
		return nil
	}

	out, err = exec.Command("/bin/sh", "-c", "sudo ovs-vsctl add-br "+OVS_BRIDGE).Output()
	if err != nil {
		return err
	}
	defer func() {
		if undo {
			ovsDestroy()
		}
	}()

	out, err = exec.Command("/bin/sh", "-c", "sudo ifconfig "+OVS_BRIDGE+" "+INET+" netmask "+NETMASK+" up").Output()
	if err != nil {
		undo = true
		return err
	}

	return nil
}

func ovsDestroy() {
	exec.Command("/bin/sh", "-c", "sudo ovs-vsctl del-br "+OVS_BRIDGE).Output()
}

func ovsSetupNetwork(id, mac string) error {
	_, err := exec.Command("/bin/sh", "-c", "sudo ovs-docker add-port "+OVS_BRIDGE+" eth0 "+id+" --macaddress "+mac).Output()
	return err
}

// we only route at client
func ovsRoute() error {
	return nil
}
