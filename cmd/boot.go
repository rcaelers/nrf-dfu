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

package cmd

import (
	"time"

	"github.com/pkg/errors"
	"github.com/rcaelers/nrf-dfu/ble"
	"github.com/rcaelers/nrf-dfu/dfu"
	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
)

type bootCommand struct {
	*baseCommand

	timeout time.Duration
	address string
}

func newBootCommand() *bootCommand {
	c := &bootCommand{}

	c.baseCommand = newBaseCommand(&cobra.Command{
		Use:   "boot",
		Short: "Reboot device into DFU mode",
		Long: `This command can be used to reboot an nRF51 or nRF52
device into DFU mode. The device supports the Buttonless DFU service.
Note that the dfu command automatically reboots into DFU mode if needed.`,
		Example: `nrf-dfu boot --address 4b668b2e16e41429fca7af1b0dc50644
nrf-dfu boot --address 4b668b2e16e41429fca7af1b0dc50644 --timeout=20s`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runBoot()
		},
	})

	c.cmd.Flags().DurationVarP(&c.timeout, "timeout", "t", 30*time.Second, "Timeout for connecting to device")
	c.cmd.Flags().StringVarP(&c.address, "address", "a", "", "Address of device to be rebooted")

	return c
}

func (c *bootCommand) runBoot() error {
	if c.address == "" {
		return errors.New("No address specified. Use --address to specifiy device address")
	}

	jww.INFO.Printf("Rebooting device '%s' into DFU mode\n", c.address)

	bleClient, err := ble.NewClient()
	if err != nil {
		return errors.Wrap(err, "failed to create new BLE client")
	}

	dfu := dfu.NewDfu(bleClient, c.timeout)

	dfu.SetDeviceAddress(c.address)
	err = dfu.EnterBootloader()
	if err != nil {
		return errors.Wrap(err, "failed to boot device into DFU mode")
	}

	return nil
}
