// Copyright 2017 fatedier, fatedier@gmail.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	mathRand "math/rand"
)

// RandId return a rand string used in frp.
func RandId() (id string, err error) {
	return RandIdWithLen(8)
}

// RandIdWithLen return a rand string with idLen length.
func RandIdWithLen(idLen int) (id string, err error) {
	b := make([]byte, idLen)
	_, err = rand.Read(b)
	if err != nil {
		return
	}

	id = fmt.Sprintf("%x", b)
	return
}

func GetAuthKey(token string, timestamp int64) (key string) {
	token = token + fmt.Sprintf("%d", timestamp)
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(token))
	data := md5Ctx.Sum(nil)
	return hex.EncodeToString(data)
}

// for example: rangeStr is "1000-2000,2001,2002,3000-4000", return an array as port ranges.
func GetPortRanges(rangeStr string) (portRanges [][2]int64, err error) {
	// for example: 1000-2000,2001,2002,3000-4000
	rangeArray := strings.Split(rangeStr, ",")
	for _, portRangeStr := range rangeArray {
		// 1000-2000 or 2001
		portArray := strings.Split(portRangeStr, "-")
		// length: only 1 or 2 is correct
		rangeType := len(portArray)
		if rangeType == 1 {
			singlePort, err := strconv.ParseInt(portArray[0], 10, 64)
			if err != nil {
				return [][2]int64{}, err
			}
			portRanges = append(portRanges, [2]int64{singlePort, singlePort})
		} else if rangeType == 2 {
			min, err := strconv.ParseInt(portArray[0], 10, 64)
			if err != nil {
				return [][2]int64{}, err
			}
			max, err := strconv.ParseInt(portArray[1], 10, 64)
			if err != nil {
				return [][2]int64{}, err
			}
			if max < min {
				return [][2]int64{}, fmt.Errorf("range incorrect")
			}
			portRanges = append(portRanges, [2]int64{min, max})
		} else {
			return [][2]int64{}, fmt.Errorf("format error")
		}
	}
	return portRanges, nil
}

func ContainsPort(portRanges [][2]int64, port int64) bool {
	for _, pr := range portRanges {
		if port >= pr[0] && port <= pr[1] {
			return true
		}
	}
	return false
}

func PortRangesCut(portRanges [][2]int64, port int64) [][2]int64 {
	var tmpRanges [][2]int64
	for _, pr := range portRanges {
		if port >= pr[0] && port <= pr[1] {
			leftRange := [2]int64{pr[0], port - 1}
			rightRange := [2]int64{port + 1, pr[1]}
			if leftRange[0] <= leftRange[1] {
				tmpRanges = append(tmpRanges, leftRange)
			}
			if rightRange[0] <= rightRange[1] {
				tmpRanges = append(tmpRanges, rightRange)
			}
		} else {
			tmpRanges = append(tmpRanges, pr)
		}
	}
	return tmpRanges
}

const (
	minTCPPort         = 0
	maxTCPPort         = 65535
	maxReservedTCPPort = 1024
	maxRandTCPPort     = maxTCPPort - (maxReservedTCPPort + 1)
)

var (
	tcpPortRand = mathRand.New(mathRand.NewSource(time.Now().UnixNano()))
)

//检查端口是否可用
// IsTCPPortAvailable returns a flag indicating whether or not a TCP port is
// available.
func IsTCPPortAvailable(port int) bool {
	if IsPortValid(port) == false {
		return false
	}

	//用tcp去尝试去监听一把
	conn, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

//端口是否在合法范围
func IsPortValid(port int) bool {
	if port < minTCPPort || port > maxTCPPort {
		return false
	}

	return true
}

// RandomTCPPort gets a free, random TCP port between 1025-65535. If no free
// ports are available -1 is returned.
func RandomTCPPort() int {
	for i := maxReservedTCPPort; i < maxTCPPort; i++ {
		p := tcpPortRand.Intn(maxRandTCPPort) + maxReservedTCPPort + 1
		if IsTCPPortAvailable(p) {
			return p
		}
	}
	return -1
}
