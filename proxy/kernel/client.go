package kernel

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
)

type Client struct {
	conn           net.Conn
	encoder        *gob.Encoder
	decoder        *gob.Decoder
	requestBuffer  bytes.Buffer
	responseBuffer bytes.Buffer
}

func (c *Client) Connect() error {
	conn, err := net.Dial("unix", UnixSocketName)
	if err != nil {
		return err
	}
	c.conn = conn
	c.encoder = gob.NewEncoder(&c.requestBuffer)
	c.decoder = gob.NewDecoder(&c.responseBuffer)
	return nil
}

func (c *Client) do(requestType MsgType, request any, expectResponseType MsgType, response any) error {
	headerPlaceholder := [8]byte{}
	c.requestBuffer.Reset()
	c.requestBuffer.Write(headerPlaceholder[:])

	if request != nil {
		if err := c.encoder.Encode(request); err != nil {
			return fmt.Errorf("Error encoding request: %w", err)
		}
	}
	if c.requestBuffer.Len() > MaxMsgSize {
		return fmt.Errorf("Request too large (%v bytes)", c.requestBuffer.Len())
	}
	header := c.requestBuffer.Bytes()
	binary.LittleEndian.PutUint32(header[0:4], uint32(c.requestBuffer.Len()))
	binary.LittleEndian.PutUint32(header[4:8], uint32(requestType))
	_, err := io.Copy(c.conn, &c.requestBuffer)
	if err != nil {
		return fmt.Errorf("Error writing request: %w", err)
	}

	// read response
	c.responseBuffer.Reset()
	rbuf := [4096]byte{}
	for {
		n, err := c.conn.Read(rbuf[:])
		if err != nil {
			return fmt.Errorf("Error reading response: %w", err)
		}
		c.responseBuffer.Write(rbuf[:n])
		if c.responseBuffer.Len() >= 8 {
			// We'll run this chunk over and over until we have the entire response
			raw := c.responseBuffer.Bytes()
			msgLen := int(binary.LittleEndian.Uint32(raw[0:4]))
			responseType := MsgType(binary.LittleEndian.Uint32(raw[4:8]))

			if c.responseBuffer.Len() > msgLen {
				return fmt.Errorf("Server sent more bytes (%v) than expected (%v)", c.responseBuffer.Len(), msgLen)
			} else if c.responseBuffer.Len() == msgLen {
				// We have the entire response

				// Dump the 8 header bytes, so that GOB can decode the payload
				dump := [8]byte{}
				c.responseBuffer.Read(dump[:])

				return c.readResponse(responseType, expectResponseType, response)
			}
		}
	}
}

func (c *Client) readResponse(responseType MsgType, expectResponseType MsgType, response any) error {
	if responseType == MsgTypeError {
		r := MsgError{}
		if err := c.decoder.Decode(&r); err != nil {
			return fmt.Errorf("Error decoding MsgErr: %v", err)
		}
		return errors.New(r.Error)
	} else if responseType != expectResponseType {
		return fmt.Errorf("Response type (%v) was not expected (%v)", responseType, expectResponseType)
	}

	// The server will send MsgTypeNone to indicate success
	if responseType != MsgTypeNone {
		if err := c.decoder.Decode(response); err != nil {
			return fmt.Errorf("Error decoding response: %w", err)
		}
	}
	return nil
}

func (c *Client) GetPeers() (*MsgGetPeersResponse, error) {
	resp := MsgGetPeersResponse{}
	if err := c.do(MsgTypeGetPeers, nil, MsgTypeGetPeersResponse, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetDevice() (*MsgGetDeviceResponse, error) {
	resp := MsgGetDeviceResponse{}
	if err := c.do(MsgTypeGetDevice, nil, MsgTypeGetDeviceResponse, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) IsDeviceAlive() error {
	return c.do(MsgTypeIsDeviceAlive, nil, MsgTypeNone, nil)
}

func (c *Client) CreateDevice() error {
	return c.do(MsgTypeCreateDevice, nil, MsgTypeNone, nil)
}

func (c *Client) CreatePeers(msg *MsgCreatePeers) error {
	return c.do(MsgTypeCreatePeers, msg, MsgTypeNone, nil)
}
