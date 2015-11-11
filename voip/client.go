package voip

import (
	"encoding/gob"
	"net"
)

func AddServer() error {
	return nil
}

func DelServer() error {
	return nil
}

func AddClient() error {
	return nil
}

func DelClient() error {
	return nil
}

func AddSnort() error {
	return nil
}

func DelSnort() error {
	return nil
}

func Route() error {
	return nil
}

func sendCommand(cmd *Command) (*Response, error) {
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{sock_file, "unix"})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	enc := gob.NewEncoder(conn)
	dec := gob.NewDecoder(conn)

	err = enc.Encode(cmd)
	if err != nil {
		return nil, err
	}

	var response Response
	err = dec.Decode(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}
