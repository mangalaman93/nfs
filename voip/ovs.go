package voip

import (
	"errors"
	"fmt"
	"log"
	"os/exec"
)

const (
	OVS_BRIDGE  = "ovsbr"
	INET_PREFIX = "173.16.1."
	INET        = INET_PREFIX + "1"
	NETMASK     = "255.255.255.0"
)

var (
	ErrOvsNotFound = errors.New("openvswitch is not installed")
)

var (
	cur_ip = 1
)

func rund(cmd string) ([]byte, error) {
	log.Println("[_INFO] running command", cmd)
	return exec.Command("/bin/sh", "-c", cmd).Output()
}

func ovsInit() error {
	out, err := rund("which ovs-vsctl")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	out, err = rund("which ovs-docker")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	out, err = rund("which ovs-ofctl")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	_, err = rund("sudo ovs-vsctl br-exists " + OVS_BRIDGE)
	if err == nil {
		log.Println("[_WARN] ovs bridge alredy exists, skipping")
		return nil
	}

	undo := false
	out, err = rund("sudo ovs-vsctl add-br " + OVS_BRIDGE)
	if err != nil {
		return err
	}
	defer func() {
		if undo {
			ovsDestroy()
		}
	}()

	out, err = rund("sudo ifconfig " + OVS_BRIDGE + " " + INET + " netmask " + NETMASK + " up")
	if err != nil {
		undo = true
		return err
	}
	log.Println("[_INFO] created ovs bridge", OVS_BRIDGE)

	return nil
}

func ovsDestroy() {
	rund("sudo ovs-vsctl del-br " + OVS_BRIDGE)
	log.Println("[_INFO] deleted ovs bridge")
}

func ovsSetupNetwork(id string) (string, string, error) {
	mac, err := GetMacAddress()
	if err != nil {
		return "", "", err
	}

	cur_ip += 1
	ip := INET_PREFIX + fmt.Sprint(cur_ip)

	_, err = rund("sudo ovs-docker add-port " + OVS_BRIDGE + " eth0 " + id + " --ipaddress=" + ip + "/24 --macaddress=" + mac)
	if err != nil {
		cur_ip -= 1
	}

	return ip, mac, err
}

func ovsUSetupNetwork(id string) {
	_, err := rund("sudo ovs-docker del-port " + OVS_BRIDGE + " eth0 " + id)
	if err != nil {
		log.Println("[_INFO] unable to remove interface from container", id, err)
	} else {
		log.Println("[_INFO] removed interface from container", id)
	}
}

// we only route at client
// TODO: only works for one host (local)
// TODO: resubmitting to port 1 always!
func ovsRoute(cmac, mac, smac string) error {
	cmd := "sudo ovs-ofctl add-flow " + OVS_BRIDGE + " priority=100,ip,dl_src=" + cmac
	cmd += ",dl_dst=" + smac + ",actions=mod_dl_dst=" + mac + ",resubmit:1"
	_, err := rund(cmd)
	if err != nil {
		log.Println("[_WARN] unable to de-setup route for ", mac, err)
	}

	return err
}

func ovsDeRoute(cmac string) {
	cmd := "sudo ovs-ofctl del-flows " + OVS_BRIDGE + " dl_src=" + cmac
	_, err := rund(cmd)
	if err != nil {
		log.Println("[_WARN] unable to desetup route for", cmac, err)
	}
}
