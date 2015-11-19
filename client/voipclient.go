package client

import (
	"encoding/gob"
	"fmt"
	"net"
	"strconv"

	"github.com/Unknwon/goconfig"
	"github.com/mangalaman93/nfs/voip"
)

type VoipClient struct {
	sockfile string
}

func NewVoipClient(cfile string) (*VoipClient, error) {
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

func (v *VoipClient) Close() {
}

func (v *VoipClient) AddServer(host string, shares int) (string, error) {
	return v.doRequest(&voip.Request{
		Code: voip.ReqStartServer,
		KeyVal: map[string]string{
			"host":   host,
			"shares": strconv.Itoa(shares),
		},
	})
}

func (v *VoipClient) AddClient(host string, shares int, server string) (string, error) {
	return v.doRequest(&voip.Request{
		Code: voip.ReqStartClient,
		KeyVal: map[string]string{
			"host":   host,
			"shares": strconv.Itoa(shares),
			"server": server,
		},
	})
}

func (v *VoipClient) AddSnort(host string, shares int) (string, error) {
	return v.doRequest(&voip.Request{
		Code: voip.ReqStartSnort,
		KeyVal: map[string]string{
			"host":   host,
			"shares": strconv.Itoa(shares),
		},
	})
}

func (v *VoipClient) Stop(cont string) {
	_, err := v.doRequest(&voip.Request{
		Code: voip.ReqStopCont,
		KeyVal: map[string]string{
			"cont": cont,
		},
	})

	if err != nil {
		panic(err)
	}
}

func (v *VoipClient) Route(client, router, server string) error {
	_, err := v.doRequest(&voip.Request{
		Code: voip.ReqRouteCont,
		KeyVal: map[string]string{
			"client": client,
			"server": server,
			"router": router,
		},
	})

	return err
}

func (v *VoipClient) doRequest(req *voip.Request) (string, error) {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{v.sockfile, "unix"})
	if err != nil {
		return "", err
	}
	defer conn.Close()

	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)
	enc.Encode(req)
	var resp voip.Response
	dec.Decode(&resp)
	if resp.Err != "" {
		return "", fmt.Errorf("%s", resp.Err)
	}

	return resp.Result, nil
}
