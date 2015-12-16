package voip

import (
	"crypto/rand"
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

func runsh(cmd string) ([]byte, error) {
	return exec.Command("/bin/sh", "-c", cmd).Output()
}

func ovsdInit() error {
	out, err := runsh("which ovs-vsctl")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}
	out, err = runsh("which ovs-docker")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}
	out, err = runsh("which ovs-ofctl")
	if err != nil || string(out) == "" {
		return ErrOvsNotFound
	}

	undo := true
	_, err = runsh("sudo ovs-vsctl br-exists " + OVS_BRIDGE)
	if err != nil {
		_, err = runsh("sudo ovs-vsctl add-br " + OVS_BRIDGE)
		if err != nil {
			return err
		}
	}
	defer func() {
		if undo {
			ovsdDestroy()
		}
	}()

	_, err = runsh("sudo ifconfig " + OVS_BRIDGE + " " + INET + " netmask " + NETMASK + " up")
	if err != nil {
		return err
	}
	log.Println("[INFO] created ovs bridge", OVS_BRIDGE)

	undo = false
	return nil
}

func ovsdDestroy() {
	runsh("sudo ovs-vsctl del-br " + OVS_BRIDGE)
	log.Println("[INFO] deleted ovs bridge")
}

func ovsdSetupNetwork(id string) (string, string, error) {
	mac, err := getMacAddress()
	if err != nil {
		return "", "", err
	}

	cur_ip += 1
	ip := INET_PREFIX + fmt.Sprint(cur_ip)
	_, err = runsh("sudo ovs-docker add-port " + OVS_BRIDGE + " eth0 " +
		id + " --ipaddress=" + ip + "/24 --macaddress=" + mac)
	if err != nil {
		cur_ip -= 1
	}

	return ip, mac, err
}

func ovsdUSetupNetwork(id string) {
	_, err := runsh("sudo ovs-docker del-port " + OVS_BRIDGE + " eth0 " + id)
	if err != nil {
		log.Println("[INFO] unable to remove interface from container", id, err)
	} else {
		log.Println("[INFO] removed interface from container", id)
	}
}

// we only route at client
// only works for one host (local)
func ovsdRoute(cmac, mac, smac string) error {
	cmd := "sudo ovs-vsctl --data=bare --no-heading --columns=ofport find interface " +
		"external_ids:attached-mac=\\\"" + cmac + "\\\""
	port, err := runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to find client ofport!")
		return err
	}

	cmd = "sudo ovs-ofctl add-flow " + OVS_BRIDGE + " priority=100,ip,dl_src=" + cmac
	cmd += ",dl_dst=" + smac + ",actions=mod_dl_dst=" + mac + ",resubmit:" + string(port)
	_, err = runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to setup route for", mac, err)
		return err
	}

	return nil
}

func ovsdDeRoute(cmac string) {
	cmd := "sudo ovs-ofctl del-flows " + OVS_BRIDGE + " dl_src=" + cmac
	_, err := runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to de-setup route for", cmac, err)
	}
}

// TODO: unique mac?
func getMacAddress() (string, error) {
	buf := make([]byte, 3)
	_, err := rand.Read(buf)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("00:16:3e:%02x:%02x:%02x", buf[0], buf[1], buf[2]), nil
}
