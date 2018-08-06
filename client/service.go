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

package client

import "github.com/KunTengRom/xfrps/models/config"

type Service struct {
	// manager control connection with server
	ctl *Control

	closedCh chan int
}

//新建service结构，新建ctrol结构
func NewService(pxyCfgs map[string]config.ProxyConf) (svr *Service) {
	svr = &Service{
		closedCh: make(chan int),
	}
	ctl := NewControl(svr, pxyCfgs)
	svr.ctl = ctl
	return
}

// service触发control启动运行
func (svr *Service) Run() error {
	err := svr.ctl.Run()
	if err != nil {
		return err
	}

	<-svr.closedCh
	return nil
}
