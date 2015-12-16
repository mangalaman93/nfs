package voip

import (
	"log"
)

const (
	OVSBR_OS = "br-int"
)

// TODO: only works for localhost
func ovsosRoute(host_ip, cmac, mac, smac string) error {
	cmd := "sudo ovs-vsctl --data=bare --no-heading --columns=ofport find interface " +
		"external_ids:attached-mac=\\\"" + cmac + "\\\""
	port, err := runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to find client ofport!")
		return err
	}

	cmd = "sudo ovs-ofctl add-flow " + OVSBR_OS + " priority=100,ip,dl_src=" + cmac
	cmd += ",dl_dst=" + smac + ",actions=mod_dl_dst=" + mac + ",resubmit:" + string(port)
	_, err = runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to setup route for", mac, err)
		return err
	}

	return nil
}

func ovsosDeRoute(host_ip, cmac string) {
	cmd := "sudo ovs-ofctl del-flows " + OVSBR_OS + " dl_src=" + cmac
	_, err := runsh(cmd)
	if err != nil {
		log.Println("[WARN] unable to de-setup route for", cmac, err)
	}
}
