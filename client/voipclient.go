package client

import (
	"encoding/gob"
	"net"

	"github.com/Unknwon/goconfig"
	"github.com/mangalaman93/nfs/voip"
)

type VoipClient struct {
	conn *net.UnixConn
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

	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{sockfile, "unix"})
	if err != nil {
		return nil, err
	}

	return &VoipClient{
		conn: conn,
	}, nil
}

func (c *VoipClient) Close() {
	c.conn.Close()
}

func (c *VoipClient) AddServer() (string, error) {
	enc := gob.NewEncoder(c.conn)
	dec := gob.NewDecoder(c.conn)

	enc.Encode(&voip.Command{
		Code: voip.StartServer,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
		},
	})

	var res voip.Response
	dec.Decode(&res)
	if res.Err != nil {
		return "", res.Err
	}

	return res.Result, nil
}

func (c *VoipClient) AddClient(server string) (string, error) {
	enc := gob.NewEncoder(c.conn)
	dec := gob.NewDecoder(c.conn)

	enc.Encode(&voip.Command{
		Code: voip.StartClient,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
			"server": server,
		},
	})

	var res voip.Response
	dec.Decode(&res)
	if res.Err != nil {
		return "", res.Err
	}

	return res.Result, nil
}

func (c *VoipClient) AddSnort() (string, error) {
	enc := gob.NewEncoder(c.conn)
	dec := gob.NewDecoder(c.conn)

	enc.Encode(&voip.Command{
		Code: voip.StartSnort,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
		},
	})

	var res voip.Response
	dec.Decode(&res)
	if res.Err != nil {
		return "", res.Err
	}

	return res.Result, nil
}

func (c *VoipClient) Stop(cont string) {
	enc := gob.NewEncoder(c.conn)
	dec := gob.NewDecoder(c.conn)

	enc.Encode(&voip.Command{
		Code: voip.StopCont,
		KeyVal: map[string]string{
			"host":   "local",
			"shares": "1024",
		},
	})

	var res voip.Response
	dec.Decode(&res)
	if res.Err != nil {
		panic(res.Err)
	}
}

func (c *VoipClient) Route(client, router, server string) error {
	enc := gob.NewEncoder(c.conn)
	dec := gob.NewDecoder(c.conn)

	enc.Encode(&voip.Command{
		Code: voip.StopCont,
		KeyVal: map[string]string{
			"client": client,
			"server": server,
			"router": router,
		},
	})

	var res voip.Response
	dec.Decode(&res)
	if res.Err != nil {
		return res.Err
	}

	return nil
}
