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

package dfu

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"io/ioutil"
	"math/rand"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rcaelers/nrf-dfu/ble"
	jww "github.com/spf13/jwalterweatherman"
)

type DfuProgress func(value int64, maxValue int64, info string)

type FirmwareUpdater interface {
	SetDeviceAddress(address string)
	SetDeviceName(name string)
	Update(filename string, progress DfuProgress) error
	EnterBootloader() error
}

type Dfu struct {
	client     ble.Client
	peripheral ble.Peripheral

	packet  ble.Characteristic
	control ble.Characteristic
	boot    ble.Characteristic

	name            string
	address         string
	addressChange   bool
	responseChannel chan []byte
	timeout         time.Duration

	firmwareZipFile *zip.ReadCloser
	initDataFile    *zip.File
	firmwareFile    *zip.File

	progress         DfuProgress
	maxProgressValue int64
	progressValue    int64
}

type dfuOperation byte

const (
	DFU_OP_PROTOCOL_VERSION  dfuOperation = 0x00
	DFU_OP_OBJECT_CREATE     dfuOperation = 0x01
	DFU_OP_RECEIPT_NOTIF_SET dfuOperation = 0x02
	DFU_OP_CRC_GET           dfuOperation = 0x03
	DFU_OP_OBJECT_EXECUTE    dfuOperation = 0x04
	DFU_OP_OBJECT_SELECT     dfuOperation = 0x06
	DFU_OP_MTU_GET           dfuOperation = 0x07
	DFU_OP_OBJECT_WRITE      dfuOperation = 0x08
	DFU_OP_PING              dfuOperation = 0x09
	DFU_OP_HARDWARE_VERSION  dfuOperation = 0x0A
	DFU_OP_FIRMWARE_VERSION  dfuOperation = 0x0B
	DFU_OP_ABORT             dfuOperation = 0x0C
	DFU_OP_RESPONSE          dfuOperation = 0x60
	DFU_OP_INVALID           dfuOperation = 0xFF
)

type dfuResult byte

const (
	DFU_RESULT_INVALID_CODE               dfuResult = 0x00
	DFU_RESULT_SUCCESS                    dfuResult = 0x01
	DFU_RESULT_OPCODE_NOT_SUPPORTED       dfuResult = 0x02
	DFU_RESULT_INVALID_PARAMETER          dfuResult = 0x03
	DFU_RESULT_INSUFFICIENT_RESOURCES     dfuResult = 0x04
	DFU_RESULT_INVALID_OBJECT             dfuResult = 0x05
	DFU_RESULT_UNSUPPORTED_TYPE           dfuResult = 0x07
	DFU_RESULT_DFUOPERATION_NOT_PERMITTED dfuResult = 0x08
	DFU_RESULT_DFUOPERATION_FAILED        dfuResult = 0x0A
)

const (
	dfuServiceUUID            = "fe59"
	dfuControlPointUUID       = "8ec90001-f315-4f60-9fb8-838830daea50"
	dfuPacketUUID             = "8ec90002-f315-4f60-9fb8-838830daea50"
	dfuButtonlessUnbondedUUID = "8ec90003-f315-4f60-9fb8-838830daea50"
	dfuButtonlessBondedUUID   = "8ec90004-f315-4f60-9fb8-838830daea50"
)

type SelectResponse struct {
	MaxSize uint32
	Offset  uint32
	Crc32   uint32
}

type ChecksumResponse struct {
	Offset uint32
	Crc32  uint32
}

func NewDfu(bleClient ble.Client, timeout time.Duration) FirmwareUpdater {
	dfu := new(Dfu)
	dfu.responseChannel = make(chan []byte)
	dfu.client = bleClient
	dfu.timeout = timeout
	return dfu
}

func (dfu *Dfu) sendControl(opcode dfuOperation, request []byte) (response []byte, err error) {
	data := append([]byte{byte(opcode)}, request...)
	err = dfu.control.WriteCharacteristic(data, ble.WithResponse)
	if err != nil {
		return nil, errors.Wrap(err, "failed to write to control characteristic")
	}

	response = <-dfu.responseChannel

	responseCode := dfuOperation(response[0])
	responseOpCode := dfuOperation(response[1])
	resultCode := dfuResult(response[2])

	if responseCode != DFU_OP_RESPONSE {
		return nil, errors.Wrap(err, "Received incorrect response code")
	}
	if responseOpCode != opcode {
		return nil, errors.Wrap(err, "Received response for incorrect operation")
	}
	if resultCode != DFU_RESULT_SUCCESS {
		return nil, errors.Wrap(err, "DFU control operation failed")
	}

	return response[3:], err
}

func (dfu *Dfu) sendBoot(request []byte) (err error) {
	err = dfu.boot.WriteCharacteristic(request, ble.WithResponse)
	if err != nil {
		return errors.Wrap(err, "failed to set advertisment name")
	}

	response := <-dfu.responseChannel
	responseCode := response[0]
	responseOpCode := response[1]
	resultCode := dfuResult(response[2])

	if responseCode != 0x20 {
		return errors.Wrap(err, "Received incorrect response code")
	}
	if responseOpCode != request[0] {
		return errors.Wrap(err, "Received response for incorrect operation")
	}
	if resultCode != DFU_RESULT_SUCCESS {
		return errors.Wrap(err, "DFU control operation failed")
	}

	return nil
}

func (dfu *Dfu) sendBootloaderAdvName(name string) error {
	buf := bytes.NewBuffer([]byte{})
	err := binary.Write(buf, binary.LittleEndian, byte(0x02))
	if err != nil {
		return errors.Wrap(err, "failed to write buffer")
	}
	err = binary.Write(buf, binary.LittleEndian, byte(len(name)))
	if err != nil {
		return errors.Wrap(err, "failed to write buffer")
	}
	err = binary.Write(buf, binary.LittleEndian, []byte(name))
	if err != nil {
		return errors.Wrap(err, "failed to write buffer")
	}

	err = dfu.sendBoot(buf.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send bootloader advertisment name command")
	}
	return nil

}

func (dfu *Dfu) sendEnterBootloader() error {
	err := dfu.sendBoot([]byte{0x01})
	if err != nil {
		return errors.Wrap(err, "failed to send enter bootloader command")
	}
	return nil
}

func (dfu *Dfu) sendData(data []byte) error {
	var err error = nil
	chunkSize := 20

	for i := 0; i < len(data); i += chunkSize {
		end := i + chunkSize

		if end > len(data) {
			end = len(data)
		}

		err = dfu.packet.WriteCharacteristic(data[i:end], ble.NoResponse)
		if err != nil {
			return errors.Wrap(err, "failed to write to packet characteristic")
		}

		dfu.updateProgress(int64(end - i))

		// TODO: Fix BLE library to wait for ack on macOS
		time.Sleep(10 * time.Millisecond)
	}
	return err
}

func (dfu *Dfu) sendSelect(selectCode byte) (SelectResponse, error) {
	var selectResponse SelectResponse

	response, err := dfu.sendControl(DFU_OP_OBJECT_SELECT, []byte{selectCode})
	if err != nil {
		return selectResponse, errors.Wrap(err, "failed to send select command")
	}

	buf := bytes.NewReader(response)
	if err := binary.Read(buf, binary.LittleEndian, &selectResponse); err != nil {
		return selectResponse, errors.Wrap(err, "failed to unpack select response data")
	}

	return selectResponse, err
}

func (dfu *Dfu) sendCreateObject(controlType byte, length uint32) error {
	header := []byte{controlType}
	len_data := make([]byte, 4)
	binary.LittleEndian.PutUint32(len_data, length)
	data := append(header, len_data...)

	_, err := dfu.sendControl(DFU_OP_OBJECT_CREATE, data)
	if err != nil {
		return errors.Wrap(err, "failed to send create object command")
	}
	return err
}

func (dfu *Dfu) sendCrcGet() (ChecksumResponse, error) {
	var checksumResponse ChecksumResponse

	response, err := dfu.sendControl(DFU_OP_CRC_GET, []byte{})
	if err != nil {
		return checksumResponse, errors.Wrap(err, "failed to send crc get command")
	}

	buf := bytes.NewReader(response)
	if err := binary.Read(buf, binary.LittleEndian, &checksumResponse); err != nil {
		return checksumResponse, errors.Wrap(err, "failed to unpack crc get response data")
	}

	return checksumResponse, err
}

func (dfu *Dfu) sendNotify(num uint16) error {
	notify_data := make([]byte, 2)
	binary.LittleEndian.PutUint16(notify_data, num)
	_, err := dfu.sendControl(DFU_OP_RECEIPT_NOTIF_SET, notify_data)
	if err != nil {
		return errors.Wrap(err, "failed to send notify command")
	}
	return err
}

func (dfu *Dfu) sendExecute() error {
	_, err := dfu.sendControl(DFU_OP_OBJECT_EXECUTE, []byte{})
	if err != nil {
		return errors.Wrap(err, "failed to send execute command")
	}
	return err
}

func (dfu *Dfu) updateProgress(increment int64) {
	dfu.progressValue += increment
	if dfu.progress != nil {
		dfu.progress(dfu.progressValue, dfu.maxProgressValue, "")
	}
}

func (dfu *Dfu) readFirmwareArchive(filename string) error {
	firmwareZipFile, err := zip.OpenReader(filename)
	if err != nil {
		return errors.Wrap(err, "Cannot open zip")
	}

	dfu.firmwareZipFile = firmwareZipFile

	for _, f := range firmwareZipFile.File {
		if strings.HasSuffix(f.Name, ".dat") {
			dfu.initDataFile = f
			dfu.maxProgressValue += int64(f.UncompressedSize64)
		}

		if strings.HasSuffix(f.Name, ".bin") {
			dfu.firmwareFile = f
			dfu.maxProgressValue += int64(f.UncompressedSize64)
		}
	}
	return err
}

func (dfu *Dfu) verifyCrc(data []byte, end int) error {
	checksumResponse, err := dfu.sendCrcGet()
	if err != nil {
		return errors.Wrap(err, "failed to compute checksum")
	}

	checksum := crc32.ChecksumIEEE(data[0:end])

	if checksumResponse.Offset != uint32(end) {
		return errors.Wrapf(err, "Size mismatch %d != %d\n", checksumResponse.Offset, end)
	}
	if checksumResponse.Crc32 != checksum {
		return errors.Wrapf(err, "CRC mismatch %d != %d\n", checksumResponse.Crc32, checksum)
	}
	return err
}

func (dfu *Dfu) transfer(objectType byte, file *zip.File) (err error) {
	rc, err := file.Open()
	if err != nil {
		return errors.Wrap(err, "failed to open firmware archive")
	}
	defer rc.Close()

	// TODO: read on demand
	data, err := ioutil.ReadAll(rc)
	if err != nil {
		return errors.Wrap(err, "failed to read firmware archive")
	}

	size := len(data)
	checksum := crc32.ChecksumIEEE(data)

	selectReponse, err := dfu.sendSelect(objectType)
	maxChunkSize := int(selectReponse.MaxSize)

	if selectReponse.Offset == uint32(size) && selectReponse.Crc32 == checksum {
		// Already uploaded
		return
	}

	for i := 0; i < size; i += maxChunkSize {
		end := i + maxChunkSize

		if end > len(data) {
			end = len(data)
		}
		chunkSize := end - i

		err = dfu.sendCreateObject(objectType, uint32(chunkSize))
		if err != nil {
			return errors.Wrap(err, "failed to create object")
		}

		err = dfu.sendData(data[i:end])
		if err != nil {
			return errors.Wrap(err, "failed to write object")
		}

		err = dfu.verifyCrc(data, end)
		if err != nil {
			return errors.Wrap(err, "verification failed")
		}

		err = dfu.sendExecute()
		if err != nil {
			return errors.Wrap(err, "failed to execute")
		}
	}
	return
}

func (dfu *Dfu) connect() (err error) {
	if dfu.address != "" {
		jww.INFO.Printf("Connecting to '%s'\n", dfu.address)
		dfu.peripheral, err = dfu.client.ConnectAddress(dfu.address, dfu.timeout)
	} else {
		jww.INFO.Printf("Connecting to '%s'\n", dfu.name)
		dfu.peripheral, err = dfu.client.ConnectName(dfu.name, dfu.timeout)
	}

	if err != nil {
		return errors.Wrap(err, "failed to connect to device")
	}

	service := dfu.peripheral.FindService(dfuServiceUUID)
	if service == nil {
		return errors.Wrap(err, "DFU Service not found")
	}

	dfu.control = service.FindCharacteristic(dfuControlPointUUID)
	dfu.packet = service.FindCharacteristic(dfuPacketUUID)

	if dfu.control == nil || dfu.packet == nil {
		dfu.addressChange = false
		dfu.boot = service.FindCharacteristic(dfuButtonlessBondedUUID)
		if dfu.boot != nil {
			jww.INFO.Printf("Using bonded buttonless bootloader.")
		} else {
			dfu.boot = service.FindCharacteristic(dfuButtonlessUnbondedUUID)
			dfu.addressChange = true
			if dfu.boot != nil {
				jww.INFO.Printf("Using unbonded buttonless bootloader.")
			}
		}
		if dfu.boot == nil {
			return errors.Wrap(err, "No DFU characteristics found")
		}
	}

	return nil
}

func (dfu *Dfu) disconnect() {
	if dfu.peripheral != nil {
		peripheral := dfu.peripheral

		dfu.peripheral = nil
		dfu.control = nil
		dfu.packet = nil
		dfu.boot = nil

		peripheral.Disconnect()
	}
}

func (dfu *Dfu) generateDeviceName() {
	const letterBytes = "abcdefghijklmnopqrstuvwxyz"

	rand.Seed(time.Now().UTC().UnixNano())

	b := make([]byte, 10)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	dfu.name = "Dfu" + string(b)
	dfu.address = ""
}

func (dfu *Dfu) enterBootloader() error {
	rebooted := false
	err := dfu.boot.Subscribe(ble.SubscriptionTypeIndication, func(data []byte) {
		dfu.responseChannel <- data
	})
	defer func() {
		if !rebooted {
			dfu.boot.Unsubscribe(ble.SubscriptionTypeIndication)
		}
	}()

	if err != nil {
		return errors.Wrap(err, "failed to subscribe to control characteristic")
	}
	err = dfu.boot.Subscribe(ble.SubscriptionTypeNotification, func(data []byte) {
		dfu.responseChannel <- data
	})
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to control characteristic")
	}
	defer func() {
		if !rebooted {
			dfu.boot.Unsubscribe(ble.SubscriptionTypeNotification)
		}
	}()

	if dfu.addressChange {
		dfu.generateDeviceName()
		jww.INFO.Printf("Changing bootloader advertisment name to '%s'\n", dfu.name)
		err = dfu.sendBootloaderAdvName(dfu.name)
		if err != nil {
			return errors.Wrap(err, "failed to set bootloaer advertisment name")
		}
	}

	err = dfu.sendEnterBootloader()
	if err != nil {
		return errors.Wrap(err, "failed to enter bootloader")
	}
	rebooted = true

	return nil
}

func (dfu *Dfu) SetDeviceAddress(address string) {
	dfu.address = address
	dfu.name = ""
}

func (dfu *Dfu) SetDeviceName(name string) {
	dfu.address = ""
	dfu.name = name
}

func (dfu *Dfu) Update(filename string, progress DfuProgress) error {
	err := dfu.connect()
	if err != nil {
		return errors.Wrap(err, "failed to connect to peripheral")
	}
	defer dfu.disconnect()

	if dfu.control == nil || dfu.packet == nil {
		jww.INFO.Println("DFU Characteristic not found. Attempting to reboot device.")
		err = dfu.enterBootloader()
		if err != nil {
			return errors.Wrap(err, "failed to enter bootloader")
		}

		tries := 5
		jww.INFO.Println("Reconnecting to peripheral")
		for {
			err = dfu.connect()
			if err != nil {
				return errors.Wrap(err, "failed to reconnect")
			}
			if (dfu.control != nil && dfu.packet != nil) {
				jww.INFO.Printf("Connected to %s\n", dfu.peripheral.Addr())
				break
			}
			tries--
			if tries == 0 {
				jww.ERROR.Printf("Failed to connect to %s\n", dfu.peripheral.Addr())
				break
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}

	err = dfu.control.Subscribe(ble.SubscriptionTypeNotification, func(data []byte) {
		dfu.responseChannel <- data
	})
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to control characteristic")
	}
	defer dfu.control.Unsubscribe(ble.SubscriptionTypeNotification)

	dfu.progress = progress

	err = dfu.readFirmwareArchive(filename)
	if err != nil {
		return errors.Wrap(err, "failed to open firmware file")
	}
	defer dfu.firmwareZipFile.Close()

	jww.INFO.Println("Transferring firmware.")

	err = dfu.transfer(0x01, dfu.initDataFile)
	if err != nil {
		return errors.Wrap(err, "failed to transfer init data")
	}

	err = dfu.transfer(0x02, dfu.firmwareFile)
	if err != nil {
		return errors.Wrap(err, "failed to transfer firmware data")
	}

	return nil
}

func (dfu *Dfu) EnterBootloader() error {
	err := dfu.connect()
	if err != nil {
		return errors.Wrap(err, "failed to connect to peripheral")
	}
	defer dfu.disconnect()

	if dfu.control != nil && dfu.packet != nil {
		jww.INFO.Println("Bootloader already active.")
	} else {
		jww.INFO.Println("Switching to DFU mode.")
		err = dfu.enterBootloader()
		if err != nil {
			return errors.Wrap(err, "failed to enter bootloader")
		}

		// TODO: Hack to wait for reponse...
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}
