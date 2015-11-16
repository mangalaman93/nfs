package client

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/Unknwon/goconfig"
	"github.com/mangalaman93/nfs/voip"
)

type VoipClient struct {
	sockfile string
}

func NewVoipClient(cfile string) (*VoipClient, error) {
	// read configuration file
	config, err := goconfig.LoadConfigFile(cfile)
	if err != nil {
		return nil, err
	}

	sockfile, err := config.GetValue("VOIP", "unix_sock")
	if err != nil {
		return nil, err
	}

	return &VoipClient{
		sockfile: sockfile,
	}, nil
}

func (c *VoipClient) Close() {
}

func (c *VoipClient) AddServer() (string, error) {
	return c.runc(&voip.Command{
		Code: voip.CmdStartServer,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
		},
	})
}

func (c *VoipClient) AddClient(server string) (string, error) {
	return c.runc(&voip.Command{
		Code: voip.CmdStartClient,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
			"server": server,
		},
	})
}

func (c *VoipClient) AddSnort() (string, error) {
	return c.runc(&voip.Command{
		Code: voip.CmdStartSnort,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
		},
	})
}

func (c *VoipClient) Stop(cont string) {
	_, err := c.runc(&voip.Command{
		Code: voip.CmdStopCont,
		KeyVal: map[string]string{
			"cont": cont,
		},
	})

	if err != nil {
		panic(err)
	}
}

func (c *VoipClient) Route(client, router, server string) error {
	_, err := c.runc(&voip.Command{
		Code: voip.CmdRouteCont,
		KeyVal: map[string]string{
			"client": client,
			"server": server,
			"router": router,
		},
	})

	return err
}

func (c *VoipClient) runc(cmd *voip.Command) (string, error) {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{c.sockfile, "unix"})
	if err != nil {
		return "", err
	}
	defer conn.Close()
	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	enc.Encode(cmd)
	var res voip.Response
	dec.Decode(&res)
	if res.Err != "" {
		return "", fmt.Errorf("%s", res.Err)
	}

	return res.Result, nil
}
