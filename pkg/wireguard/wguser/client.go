package wguser

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Client can only be used by a single thread at a time.
// There is a giant lock that ensures this.
type Client struct {
	lock            sync.Mutex
	conn            net.Conn
	encoder         *gob.Encoder
	decoder         *gob.Decoder
	requestBuffer   bytes.Buffer
	responseBuffer  bytes.Buffer
	maxReadDuration time.Duration
	clientSecret    string
}

func NewClient(secret string) *Client {
	return &Client{
		maxReadDuration: 10 * time.Second,
		clientSecret:    secret,
	}
}

func (c *Client) Connect() error {
	c.lock.Lock()

	c.close()
	if err := c.connect(); err != nil {
		c.lock.Unlock()
		return err
	}

	c.lock.Unlock()

	if err := c.Authenticate(); err != nil {
		c.close()
		return fmt.Errorf("Error authenticating: %w", err)
	}
	return nil
}

func (c *Client) connect() error {
	// max delay = 2 ^ 7 * 10ms = 1.28s
	// max total delay ~= 2.5s
	var conn net.Conn
	var err error
	for attempt := 0; attempt < 8; attempt++ {
		//proto := "tcp"
		//addr := c.host + ":666"
		//conn, err := net.Dial(proto, addr)

		proto := "unix"
		addr := UnixSocketName
		conn, err = net.Dial(proto, addr)
		if err == nil {
			break
		}
		time.Sleep((1 << attempt) * 10 * time.Millisecond)
	}
	if err != nil {
		return err
	}
	c.conn = conn
	c.encoder = gob.NewEncoder(&c.requestBuffer)
	c.decoder = gob.NewDecoder(&c.responseBuffer)
	return nil
}

func (c *Client) IsConnected() bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.conn != nil
}

func (c *Client) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.close()
}

func (c *Client) close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.encoder = nil
		c.decoder = nil
	}
}

func (c *Client) do(requestType MsgType, request any, expectResponseType MsgType, response any) error {
	if c.conn == nil {
		return ErrNotConnected
	}

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
	if _, err := io.Copy(c.conn, &c.requestBuffer); err != nil {
		return fmt.Errorf("Error writing request: %w", err)
	}

	// read response
	c.responseBuffer.Reset()
	rbuf := [4096]byte{}
	for {
		c.conn.SetReadDeadline(time.Now().Add(c.maxReadDuration))
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

// Rehydrate an error that got turned into a string over the wire
func makeError(e string) error {
	switch e {
	case ErrNotConnected.Error():
		return ErrNotConnected
	case ErrWireguardDeviceNotExist.Error():
		return ErrWireguardDeviceNotExist
	}

	return errors.New(e)
}

func (c *Client) readResponse(responseType MsgType, expectResponseType MsgType, response any) error {
	if responseType == MsgTypeError {
		r := MsgError{}
		if err := c.decoder.Decode(&r); err != nil {
			return fmt.Errorf("Error decoding MsgErr: %v", err)
		}
		return makeError(r.Error)
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
	c.lock.Lock()
	defer c.lock.Unlock()

	resp := MsgGetPeersResponse{}
	if err := c.do(MsgTypeGetPeers, nil, MsgTypeGetPeersResponse, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) GetDevice() (*MsgGetDeviceResponse, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	resp := MsgGetDeviceResponse{}
	if err := c.do(MsgTypeGetDevice, nil, MsgTypeGetDeviceResponse, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) IsDeviceAlive() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeIsDeviceAlive, nil, MsgTypeNone, nil)
}

func (c *Client) Authenticate() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	msg := MsgAuthenticate{
		Secret: c.clientSecret,
	}
	return c.do(MsgTypeAuthenticate, &msg, MsgTypeNone, nil)
}

func (c *Client) BringDeviceUp() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeBringDeviceUp, nil, MsgTypeNone, nil)
}

func (c *Client) TakeDeviceDown() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeTakeDeviceDown, nil, MsgTypeNone, nil)
}

func (c *Client) CreatePeers(msg *MsgCreatePeersInMemory) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeCreatePeersInMemory, msg, MsgTypeNone, nil)
}

func (c *Client) RemovePeer(msg *MsgRemovePeerInMemory) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeRemovePeerInMemory, msg, MsgTypeNone, nil)
}

func (c *Client) CreateDeviceInConfigFile(msg *MsgCreateDeviceInConfigFile) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeCreateDeviceInConfigFile, msg, MsgTypeNone, nil)
}

func (c *Client) SetProxyPeerInConfigFile(msg *MsgSetProxyPeerInConfigFile) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	return c.do(MsgTypeSetProxyPeerInConfigFile, msg, MsgTypeNone, nil)
}
