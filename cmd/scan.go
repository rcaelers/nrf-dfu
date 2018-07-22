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
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/rcaelers/nrf-dfu/ble"
	"github.com/spf13/cobra"
)

type scanCommand struct {
	*baseCommand

	duration time.Duration
}

func newScanCommand() *scanCommand {
	c := &scanCommand{}

	c.baseCommand = newBaseCommand(&cobra.Command{
		Use:   "scan",
		Short: "Scan for BLUE devices",
		Example: `nrf-dfu scan
nrf-dfu scan --duration=30s`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runScan()
		},
	})

	c.cmd.Flags().DurationVarP(&c.duration, "duration", "d", 30*time.Second, "Duration of the BLE scan")

	return c
}

func (c *scanCommand) runScan() error {
	fmt.Printf("Scanning for BLE devices...\n")

	bleClient, err := ble.NewClient()
	if err != nil {
		return errors.Wrap(err, "failed to create new BLE client")
	}

	err = bleClient.Scan(c.duration, func(adv ble.Advertisement) {
		info := ""
		for _, v := range adv.Services {
			if v == "fe59" {
				info = "[DFU Supported]"
			}
		}
		fmt.Printf("%s : %s %s\n", adv.Addr, adv.Name, info)
	})

	switch errors.Cause(err) {
	case context.DeadlineExceeded:
		return nil
	case context.Canceled:
		fmt.Printf("Canceled..\n")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to perform BLE scan")
	}
	return nil
}
