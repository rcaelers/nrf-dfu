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
	"fmt"
	"os"

	"github.com/spf13/cobra"
	jww "github.com/spf13/jwalterweatherman"
)

type Command interface {
	init(cli *Cli)
	getCommand() *cobra.Command
}

type globalOptions struct {
	Quiet bool
	Debug bool
}

type baseCommand struct {
	cmd *cobra.Command
	cli *Cli
}

func (c *baseCommand) init(cli *Cli) {
	c.cli = cli
}

func (c *baseCommand) getCommand() *cobra.Command {
	return c.cmd
}

func (c *baseCommand) AddCommand(command Command) {
	childCmd := command.getCommand()
	c.cmd.AddCommand(childCmd)
}

func newBaseCommand(cmd *cobra.Command) *baseCommand {
	return &baseCommand{cmd: cmd}
}

type Cli struct {
	*baseCommand
	globalOptions
}

func NewCli() *Cli {

	c := &Cli{}

	c.baseCommand = newBaseCommand(&cobra.Command{
		Use:     "nrf-dfu",
		Short:   "A DFU tool for nRF modules",
		Long:    `nrf-dfu is a tool to upload firmware to an nRF51 or nRF52 device.`,
		Version: "0.1",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			c.InitLogging()
		},
	})

	c.cmd.SilenceUsage = true
	c.cmd.SilenceErrors = true

	c.cmd.PersistentFlags().BoolVarP(&c.Quiet, "quiet", "q", false, "suppress all output")
	c.cmd.PersistentFlags().BoolVarP(&c.Debug, "debug", "D", false, "produce debug output")

	c.AddCommand(newScanCommand())
	c.AddCommand(newBootCommand())
	c.AddCommand(newDfuCommand())

	return c
}

func (c *Cli) AddCommand(command Command) {
	command.init(c)
	c.baseCommand.AddCommand(command)
}

func (c *Cli) InitLogging() {
	if c.Debug {
		jww.SetStdoutThreshold(jww.LevelDebug)
	} else if c.Quiet {
		jww.SetStdoutThreshold(jww.LevelFatal)
	} else {
		jww.SetStdoutThreshold(jww.LevelInfo)
	}
}

func (c *Cli) Execute() {
	if err := c.cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
