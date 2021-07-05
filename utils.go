// Codehere is mostly taken from github.com/tailscale/tailscale
// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package headscale

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	mathrand "math/rand"

	"golang.org/x/crypto/nacl/box"
	"gorm.io/gorm"
	"tailscale.com/types/wgkey"
)

// Error is used to compare errors as per https://dave.cheney.net/2016/04/07/constant-errors
type Error string

func (e Error) Error() string { return string(e) }

func decode(msg []byte, v interface{}, pubKey *wgkey.Key, privKey *wgkey.Private) error {
	return decodeMsg(msg, v, pubKey, privKey)
}

func decodeMsg(msg []byte, v interface{}, pubKey *wgkey.Key, privKey *wgkey.Private) error {
	decrypted, err := decryptMsg(msg, pubKey, privKey)
	if err != nil {
		return err
	}
	// fmt.Println(string(decrypted))
	if err := json.Unmarshal(decrypted, v); err != nil {
		return fmt.Errorf("response: %v", err)
	}
	return nil
}

func decryptMsg(msg []byte, pubKey *wgkey.Key, privKey *wgkey.Private) ([]byte, error) {
	var nonce [24]byte
	if len(msg) < len(nonce)+1 {
		return nil, fmt.Errorf("response missing nonce, len=%d", len(msg))
	}
	copy(nonce[:], msg)
	msg = msg[len(nonce):]

	pub, pri := (*[32]byte)(pubKey), (*[32]byte)(privKey)
	decrypted, ok := box.Open(nil, msg, &nonce, pub, pri)
	if !ok {
		return nil, fmt.Errorf("cannot decrypt response")
	}
	return decrypted, nil
}

func encode(v interface{}, pubKey *wgkey.Key, privKey *wgkey.Private) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return encodeMsg(b, pubKey, privKey)
}

func encodeMsg(b []byte, pubKey *wgkey.Key, privKey *wgkey.Private) ([]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		panic(err)
	}
	pub, pri := (*[32]byte)(pubKey), (*[32]byte)(privKey)
	msg := box.Seal(nonce[:], b, &nonce, pub, pri)
	return msg, nil
}

func (h *Headscale) getAvailableIP() (*net.IP, error) {
	i := 0
	for {
		ip, err := getRandomIP()
		if err != nil {
			return nil, err
		}
		m := Machine{}
		if result := h.db.First(&m, "ip_address = ?", ip.String()); errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ip, nil
		}
		i++
		if i == 100 { // really random number
			break
		}
	}
	return nil, errors.New("Could not find an available IP address in 100.64.0.0/10")
}

func getRandomIP() (*net.IP, error) {
	mathrand.Seed(time.Now().Unix())
	ipo, ipnet, err := net.ParseCIDR("100.64.0.0/10")
	if err == nil {
		ip := ipo.To4()
		// fmt.Println("In Randomize IPAddr: IP ", ip, " IPNET: ", ipnet)
		// fmt.Println("Final address is ", ip)
		// fmt.Println("Broadcast address is ", ipb)
		// fmt.Println("Network address is ", ipn)
		r := mathrand.Uint32()
		ipRaw := make([]byte, 4)
		binary.LittleEndian.PutUint32(ipRaw, r)
		// ipRaw[3] = 254
		// fmt.Println("ipRaw is ", ipRaw)
		for i, v := range ipRaw {
			// fmt.Println("IP Before: ", ip[i], " v is ", v, " Mask is: ", ipnet.Mask[i])
			ip[i] = ip[i] + (v &^ ipnet.Mask[i])
			// fmt.Println("IP After: ", ip[i])
		}
		// fmt.Println("FINAL IP: ", ip.String())
		return &ip, nil
	}

	return nil, err
}
