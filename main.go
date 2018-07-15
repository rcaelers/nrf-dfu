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

package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"gopkg.in/cheggaaa/pb.v2"
)

func cmdScan(c *cli.Context) error {
	fmt.Printf("Scanning for BLE devices\n")
	ble := NewBleClient()
	err := ble.Scan(func(adv BleAdvertisement) {
		info := ""
		for _, v := range adv.Services {
			if v == "fe59" {
				info = "[DFU Supported]"
			}
		}
		fmt.Printf("%s : %s %s\n", adv.Addr, adv.Name, info)
	})
	return err
}

func cmdPrepare(c *cli.Context) error {
	addr := c.String("addr")

	if addr == "" {
		return errors.New("no address specified")
	}
	fmt.Printf("Preparing for firmware upgrade of %s\n", addr)

	ble := NewBleClient()
	dfu := NewDfu(ble)

	err := dfu.EnterBootloader(addr)
	if err != nil {
		return errors.Wrap(err, "failed to boot device into DFU mode")
	}

	return nil
}

func dfuProgress(bar *pb.ProgressBar) DfuProgress {
	return func(value int64, maxValue int64, info string) {
		if bar.Total() != maxValue {
			bar.SetTotal(maxValue)
		}
		bar.SetCurrent(value)
	}
}

func cmdDfu(c *cli.Context) error {
	addr := c.String("addr")
	fw := c.String("fw")

	if addr == "" {
		return errors.New("no address specified")
	}
	if fw == "" {
		return errors.New("no firmware filename")
	}

	fmt.Printf("Upgrading firmware of %s with %s\n", addr, fw)

	ble := NewBleClient()
	dfu := NewDfu(ble)

	bar := pb.ProgressBarTemplate(`{{ white "DFU:" }} {{bar . | green}} {{speed . "%s byte/s" | white }}`).Start(100)

	err := dfu.Update(addr, fw, dfuProgress(bar))
	if err != nil {
		return errors.Wrap(err, "can't upgrade firmware")
	}

	bar.Finish()

	return err
}

func main() {
	app := cli.NewApp()
	app.Name = "nrf-dfu"
	app.Usage = "A DFU tool for nRF modules"
	app.Version = "0.0.1"
	app.Action = cli.ShowAppHelp

	flgAddr := cli.StringFlag{Name: "addr, a", Usage: "Address of device to be upgraded"}
	flgFWFilenme := cli.StringFlag{Name: "fw, f", Usage: "Filename of the firmware archive"}

	app.Commands = []cli.Command{
		{
			Name:    "scan",
			Aliases: []string{"s"},
			Usage:   "Scan for upgradable devices",
			Action:  cmdScan,
		},
		{
			Name:    "dfu",
			Aliases: []string{"d"},
			Usage:   "Perform device firmware upgrade",
			Action:  cmdDfu,
			Flags:   []cli.Flag{flgAddr, flgFWFilenme},
		},
		{
			Name:    "boot",
			Aliases: []string{"b"},
			Usage:   "Boot device into DFU mode",
			Action:  cmdPrepare,
			Flags:   []cli.Flag{flgAddr},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
