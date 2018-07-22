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
	"github.com/go-ble/ble"
	"github.com/pkg/errors"
	"strings"
	"time"
)

type GoBleInitFunc func() (ble.Device, error)

type bleClient struct {
	device *ble.Device
}

type blePeripheral struct {
	address string
	client  ble.Client
	profile *ble.Profile
}

type bleService struct {
	client  ble.Client
	service *ble.Service
}

type bleCharacteristic struct {
	client         ble.Client
	characteristic *ble.Characteristic
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

	return &bleClient{device: currentDevice}, nil
}

func (b *bleClient) ConnectName(name string, timeout time.Duration) (Peripheral, error) {

	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))

	client, err := ble.Connect(ctx, func(a ble.Advertisement) bool {
		return strings.ToLower(a.LocalName()) == strings.ToLower(name)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to BLE peripheral")
	}

	addr := client.Addr()

	profile, err := client.DiscoverProfile(true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover profiles")
	}

	return &blePeripheral{
		address: addr.String(),
		client:  client,
		profile: profile,
	}, nil
}

func (b *bleClient) ConnectAddress(address string, timeout time.Duration) (Peripheral, error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), timeout))

	client, err := ble.Dial(ctx, ble.NewAddr(address))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to BLE peripheral")
	}

	profile, err := client.DiscoverProfile(true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover profiles")
	}

	return &blePeripheral{
		address: address,
		client:  client,
		profile: profile,
	}, nil
}

func (b *bleClient) Scan(duration time.Duration, handler AdvertisementHandler) (err error) {
	ctx := ble.WithSigHandler(context.WithTimeout(context.Background(), duration))

	err = ble.Scan(ctx, false, b.handleAdvertisement(handler), nil)

	return err
}

func (p *blePeripheral) Disconnect() (err error) {
	p.client.CancelConnection()
	return
}

func (p *blePeripheral) Addr() string {
	return p.address
}

func (p *blePeripheral) FindService(uuid string) Service {
	bleUuid, _ := ble.Parse(uuid)
	if s := p.profile.FindService(ble.NewService(bleUuid)); s != nil {
		return &bleService{
			client:  p.client,
			service: s,
		}
	}
	return nil
}

func (p *blePeripheral) FindCharacteristic(uuid string) Characteristic {
	bleUuid, _ := ble.Parse(uuid)
	if c := p.profile.FindCharacteristic(ble.NewCharacteristic(bleUuid)); c != nil {
		return &bleCharacteristic{
			client:         p.client,
			characteristic: c,
		}
	}
	return nil

}

func writeCharacteristicTypeToBool(writeType WriteCharacteristicType) bool {
	switch writeType {
	case NoResponse:
		return true
	case WithResponse:
		return false
	}
	return false
}

func (p *blePeripheral) WriteCharacteristic(uuid string, data []byte, writeType WriteCharacteristicType) (err error) {
	bleUuid, _ := ble.Parse(uuid)
	if c := p.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = p.client.WriteCharacteristic(c.(*ble.Characteristic), data, writeCharacteristicTypeToBool(writeType))
		if err != nil {
			return errors.Wrap(err, "failed to write to BLE characteristic")
		}
	}

	return
}

func subscriptionTypeToBool(subType SubscriptionType) bool {
	switch subType {
	case SubscriptionTypeNotification:
		return false
	case SubscriptionTypeIndication:
		return true
	}
	return false
}

func (p *blePeripheral) Subscribe(uuid string, subType SubscriptionType, f func([]byte)) (err error) {
	bleUuid, _ := ble.Parse(uuid)
	if c := p.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = p.client.Subscribe(c.(*ble.Characteristic), subscriptionTypeToBool(subType), f)
		if err != nil {
			return errors.Wrap(err, "failed to subscribe to BLE characteristic value changes")
		}
	}

	return
}

func (p *blePeripheral) Unsubscribe(uuid string, subType SubscriptionType) (err error) {
	bleUuid, _ := ble.Parse(uuid)
	if c := p.profile.Find(ble.NewCharacteristic(bleUuid)); c != nil {
		err = p.client.Unsubscribe(c.(*ble.Characteristic), subscriptionTypeToBool(subType))
		if err != nil {
			return errors.Wrap(err, "failed to unsubscribe to BLE characteristic value changes")
		}
	}

	return
}

func (s *bleService) Uuid() string {
	return s.service.UUID.String()
}

func (s *bleService) FindCharacteristic(uuid string) Characteristic {
	bleUuid, _ := ble.Parse(uuid)
	refChar := ble.NewCharacteristic(bleUuid)
	for _, c := range s.service.Characteristics {
		if c.UUID.Equal(refChar.UUID) {
			return &bleCharacteristic{
				client:         s.client,
				characteristic: c,
			}
		}
	}

	return nil
}

func (c *bleCharacteristic) Uuid() string {
	return c.characteristic.UUID.String()
}

func (c *bleCharacteristic) WriteCharacteristic(data []byte, writeType WriteCharacteristicType) (err error) {
	err = c.client.WriteCharacteristic(c.characteristic, data, writeCharacteristicTypeToBool(writeType))
	if err != nil {
		return errors.Wrap(err, "failed to write to BLE characteristic")
	}
	return
}

func (c *bleCharacteristic) Subscribe(subType SubscriptionType, f func([]byte)) (err error) {
	err = c.client.Subscribe(c.characteristic, subscriptionTypeToBool(subType), f)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to BLE characteristic value changes")
	}

	return
}

func (c *bleCharacteristic) Unsubscribe(subType SubscriptionType) (err error) {
	err = c.client.Unsubscribe(c.characteristic, subscriptionTypeToBool(subType))
	if err != nil {
		return errors.Wrap(err, "failed to unsubscribe from BLE characteristic value changes")
	}

	return
}

func (c *bleClient) handleAdvertisement(handler AdvertisementHandler) ble.AdvHandler {
	return func(a ble.Advertisement) {
		services := []string{}
		for _, s := range a.Services() {
			services = append(services, s.String())
		}

		handler(Advertisement{Name: a.LocalName(), Addr: a.Addr().String(), Services: services})
	}
}
