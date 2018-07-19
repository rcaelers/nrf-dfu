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
	"time"
)

type AdvertisementHandler func(adv Advertisement)

type Advertisement struct {
	Addr     string
	Name     string
	Services []string
}

type Client interface {
	ConnectName(name string, timeout time.Duration) (Peripheral, error)
	ConnectAddress(address string, timeout time.Duration) (Peripheral, error)
	Scan(duration time.Duration, handler AdvertisementHandler) error
}

type Peripheral interface {
	Addr() string

	Disconnect() error

	FindService(uuid string) Service
	FindCharacteristic(uuid string) Characteristic

	WriteCharacteristic(uuid string, data []byte, noresp bool) error
	Subscribe(uuid string, indication bool, f func([]byte)) error
	Unsubscribe(uuid string, indication bool) error
}

type Service interface {
	Uuid() string
	FindCharacteristic(uuid string) Characteristic
}

type Characteristic interface {
	Uuid() string

	WriteCharacteristic(data []byte, noresp bool) error
	Subscribe(indication bool, f func([]byte)) error
	Unsubscribe(indication bool) error
}
