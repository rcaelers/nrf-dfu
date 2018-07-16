// Copyright (C) 2018 Rob Caelers <rob.caelers@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ble

import (
	"context"
	"fmt"
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
	"time"
)

type GoBleInitFunc func() (ble.Device, error)

type bleClient struct {
	b         *bleClient
	client    ble.Client
	profile   *ble.Profile
	connected bool
}

var currentDevice *ble.Device

func NewGoBleClient(init GoBleInitFunc) (*bleClient, error) {
	if currentDevice == nil {
		device, err := init()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new BLE device")
		}
		ble.SetDefaultDevice(device)

		currentDevice = &device
	}

	return &bleClient{
		connected: false,
	}, nil
}

func (b *bleClient) Connect(address string, timeout time.Duration) (err error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))

	b.client, err = ble.Dial(ctx, ble.NewAddr(address))
	if err != nil {
		return errors.Wrap(err, "failed to connect to device")
	}

	b.profile, err = b.client.DiscoverProfile(true)
	if err != nil {
		return errors.Wrap(err, "failed to discover device profiles")
	}

	b.connected = true
	return
}

func (b *bleClient) Disconnect() (err error) {
	if !b.connected {
		return errors.Wrap(err, "not connected")
	}
	b.client.CancelConnection()
	b.connected = false
	return
}

func (b *bleClient) WriteCharacteristic(uuid string, data []byte, noresp bool) (err error) {
	if !b.connected {
		return errors.Wrap(err, "not connected")
	}

	bleUuid, _ := ble.Parse(uuid)
	if c := b.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = b.client.WriteCharacteristic(c.(*ble.Characteristic), data, noresp)
		if err != nil {
			return errors.Wrap(err, "failed to write to BLE characteric")
		}
	}

	return
}

func (b *bleClient) Subscribe(uuid string, indication bool, f func([]byte)) (err error) {
	if !b.connected {
		return errors.Wrap(err, "not connected")
	}

	bleUuid, _ := ble.Parse(uuid)
	if c := b.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = b.client.Subscribe(c.(*ble.Characteristic), indication, f)
		if err != nil {
			return errors.Wrap(err, "failed to subscribe to BLE characteric value changes")
		}
	}

	return
}

func (b *bleClient) Unsubscribe(uuid string, indication bool) (err error) {
	if !b.connected {
		return errors.Wrap(err, "not connected")
	}

	bleUuid, _ := ble.Parse(uuid)
	if c := b.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = b.client.Unsubscribe(c.(*ble.Characteristic), indication)
		if err != nil {
			return errors.Wrap(err, "failed to unsubscribe to BLE characteris value changes")
		}
	}

	return
}

func (b *bleClient) handleAdvertisement(handler AdvertisementHandler) ble.AdvHandler {
	return func(a ble.Advertisement) {
		services := []string{}
		for _, s := range a.Services() {
			services = append(services, s.String())
		}

		handler(Advertisement{Name: a.LocalName(), Addr: a.Addr().String(), Services: services})
	}
}

func (b *bleClient) Scan(duration time.Duration, handler AdvertisementHandler) (err error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), duration))

	err = ble.Scan(ctx, false, b.handleAdvertisement(handler), nil)

	switch errors.Cause(err) {
	case context.DeadlineExceeded:
		return nil
	case context.Canceled:
		fmt.Printf("Canceled..\n")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to start BLE scan")
	}

	return err
}
