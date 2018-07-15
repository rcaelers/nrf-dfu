# nRF51/52 Device Firmware Update tool

Command line tool to update firmware of nRF51/52 devices with Nordic's Secure DFU bootloader.

Requires Go 1.11+

Tested on macOS with a SparkFun nRF52832 Breakout board.

### TODO

- [X] Support unbonded buttonless bootloader
- [ ] Support bonded buttonless bootloader
- [ ] Automatically boot device into DFU mode and perform upgrade
- [ ] Make scan duration configurable
- [X] Report progress
- [ ] Remove duplicates when scanning
- [ ] Test on Linux
- [ ] Remove sleep-hack
- [ ] ...
