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
	"gopkg.in/cheggaaa/pb.v2"
)

type dfuCommand struct {
	*baseCommand

	timeout          time.Duration
	address          string
	firmwareFilename string
}

func newDfuCommand() *dfuCommand {
	c := &dfuCommand{}

	c.baseCommand = newBaseCommand(&cobra.Command{
		Use:   "dfu",
		Short: "Perform device firmware upgrade",
		Args:  cobra.NoArgs,
		Long: `This command can be used to perform a firmware upgrade of an nRF51 or nRF52
device. If the device supports the Buttonless DFU service, this service will
be used to first reboot the device into DFU mode.`,
		Example: `nrf-dfu dfu --address 4b668b2e16e41429fca7af1b0dc50644 --firmware FW.zip
nrf-dfu dfu --address 4b668b2e16e41429fca7af1b0dc50644 --firmware FW.zip --timeout=20s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runDfu()
		},
	})

	c.cmd.Flags().DurationVarP(&c.timeout, "timeout", "t", 30*time.Second, "Timeout for connecting to device")
	c.cmd.Flags().StringVarP(&c.firmwareFilename, "firmware", "f", "", "Filename of the firmware archive")
	c.cmd.Flags().StringVarP(&c.address, "address", "a", "", "Address of device to be upgraded")
	return c
}

func (c *dfuCommand) runDfu() error {
	if c.address == "" {
		return errors.New("No address specified. Use --addr to specify device address.")
	}
	if c.firmwareFilename == "" {
		return errors.New("No firmware filename specified. Use --firmware to specify firmware archive filename.")
	}

	jww.INFO.Printf("Upgrading firmware of device '%s' with '%s'\n", c.address, c.firmwareFilename)

	bleClient, err := ble.NewClient()
	if err != nil {
		return errors.Wrap(err, "failed to create new BLE client")
	}

	dfu := dfu.NewDfu(bleClient, c.timeout)
	dfu.SetDeviceAddress(c.address)

	var bar *pb.ProgressBar = nil

	err = dfu.Update(c.firmwareFilename, func(value int64, maxValue int64, info string) {
		if bar == nil {
			bar = pb.ProgressBarTemplate(`{{ white "DFU:" }} {{bar . | green}} {{speed . "%s byte/s" | white }}`).Start(100)
		}
		if bar.Total() != maxValue {
			bar.SetTotal(maxValue)
		}
		bar.SetCurrent(value)
	})

	if err != nil {
		return errors.Wrap(err, "failed to upgrade device firmware")
	}

	bar.Finish()

	return err
}
