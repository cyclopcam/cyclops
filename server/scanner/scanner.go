package scanner

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"time"

	"github.com/bmharper/cyclops/server/camera"
	"github.com/bmharper/cyclops/server/configdb"
	"github.com/bmharper/cyclops/server/gen"
)

/*
This is a dump from my Linux machine:

up|loopback, , ip+net, 127.0.0.1/8
up|loopback, , ip+net, ::1/128
up|broadcast|multicast, 24:4b:fe:55:b0:7e, ip+net, 192.168.10.15/24                  This is what we're looking for
up|broadcast|multicast, 24:4b:fe:55:b0:7e, ip+net, fe80::3c1:a2a7:6272:7bf2/64
up|broadcast|multicast, 02:42:fd:c2:70:78, ip+net, 172.17.0.1/16
up|broadcast|multicast, 02:42:f2:7f:00:db, ip+net, 172.16.238.1/24
up|broadcast|multicast, 02:42:e4:d9:0e:6c, ip+net, 172.19.0.1/16
up|broadcast|multicast, 02:42:43:b4:63:38, ip+net, 172.21.0.1/16
up|broadcast|multicast, 02:42:6d:f6:09:04, ip+net, 172.18.0.1/16
up|broadcast|multicast, 02:42:43:b4:7a:fd, ip+net, 192.168.49.1/24                   wireguard related?
up|broadcast|multicast, 02:42:e7:62:38:e1, ip+net, 172.22.0.1/16
up|broadcast|multicast, 02:42:d7:2f:4a:b9, ip+net, 172.20.0.1/16

Without better knowledge, I'm going with:
* Find the first adapter with an IPv4 and IPv6 address, where the IPv4 is on 192.168.X.X
*/

// Any option, if left to the zero value, is ignored, and defaults are used instead.
type ScanOptions struct {
	Timeout time.Duration // Timeout on connecting to each host
}

/* ScanForLocalCameras scans the local IPv4 network for cameras
options is optional.
*/
func ScanForLocalCameras(options *ScanOptions) ([]*configdb.Camera, error) {
	ip, err := getLocalIPv4()
	//fmt.Printf("getLocalIPv4: %v, %v\n", ip, err)
	if err != nil {
		return nil, err
	}
	ip4 := ip.To4()
	if ip4 == nil {
		return nil, fmt.Errorf("Local IP address is not an IPv4 address")
	}

	nThreads := 50
	workQueue := make(chan net.IP, 256)
	resultQueue := make(chan *configdb.Camera, 256)
	doneQueue := make(chan bool, nThreads)

	// assume address 1 is DHCP, and not used by a camera
	//fmt.Printf("loading up work\n")
	for i := 2; i < 255; i++ {
		workQueue <- net.IPv4(ip4[0], ip4[1], ip4[2], byte(i))
	}
	//fmt.Printf("starting threads\n")
	for i := 0; i < nThreads; i++ {
		go func() {
			done := false
			for !done {
				select {
				case camIP := <-workQueue:
					//fmt.Printf("Trying %v\n", camIP)
					model, err := tryToContactCamera(camIP, options)
					if err == nil && model != camera.CameraModelUnknown {
						cam := &configdb.Camera{
							Model: string(model),
							Host:  camIP.String(),
						}
						//fmt.Printf("Found %v %v %v\n", camIP, cam.Model, cam.Host)
						resultQueue <- cam
					}
				default:
					done = true
				}
			}
			//fmt.Printf("thread done\n")
			doneQueue <- true
		}()
	}

	for i := 0; i < nThreads; i++ {
		<-doneQueue
	}
	//fmt.Printf("done\n")
	cams := gen.DrainChannelIntoSlice(resultQueue)

	// always present a consistent view to the user
	sort.Slice(cams, func(i, j int) bool {
		return cams[i].Host < cams[j].Host
	})

	return cams, nil
}

func tryToContactCamera(ip net.IP, options *ScanOptions) (camera.CameraModels, error) {
	//fmt.Printf("Contacting %v...", ip)

	// 100ms has been sufficient on my home network with HikVision cameras and ethernet, but it might be too aggressive for some
	timeout := 100 * time.Millisecond
	if options != nil && options.Timeout != 0 {
		timeout = options.Timeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	u := "http://" + ip.String()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return camera.CameraModelUnknown, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return camera.CameraModelUnknown, err
	}
	defer resp.Body.Close()
	bodyB, err := io.ReadAll(resp.Body)
	if err != nil {
		return camera.CameraModelUnknown, err
	}
	body := string(bodyB)
	return camera.IdentifyCameraFromHTTP(resp.Header, body), nil
}

// GetLocalIPv4 tries to figure out our local IPv4 address (eg 192.168.1.5)
// From https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getLocalIPv4() (net.IP, error) {
	ip, err := getOutboundIP()
	if err == nil {
		return ip, nil
	}

	// fall back to scanning local interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		hasIPv4 := false
		hasIPv6 := false
		var first4 net.IP
		for _, addr := range addresses {
			//fmt.Printf("%v, %v, %v, %v\n", iface.Flags.String(), iface.HardwareAddr.String(), addr.Network(), addr.String())
			switch v := addr.(type) {
			case *net.IPAddr:
				//fmt.Printf("IPAddr %v\n", v)
			case *net.IPNet:
				//fmt.Printf("IPNet %v\n", v)
				ip4 := v.IP.To4()
				if ip4 != nil && ip4[0] == 192 && ip4[1] == 168 {
					first4 = ip4
					hasIPv4 = true
				}
				if v.IP.To16() != nil {
					hasIPv6 = true
				}
			}
		}
		if hasIPv4 && hasIPv6 {
			return first4, nil
		}
	}

	return nil, fmt.Errorf("Failed to find local IP address")
}

// From https://stackoverflow.com/questions/23558425/how-do-i-get-the-local-ip-address-in-go
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "1.1.1.1:80")
	if err != nil {
		return net.IP{}, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP, nil
}
