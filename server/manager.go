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

package server

import (
	"fmt"
	"sync"
)

//端口管理
type PortManager struct {
	freePort map[string]int64
	ftpPort  map[string]int64

	mu sync.RWMutex
}

//新加端口管理
func NewPortManager() *PortManager {
	return &PortManager{
		freePort: make(map[string]int64),
		ftpPort:  make(map[string]int64),
	}
}

//增加端口隐射
func (pm *PortManager) Add(runId string, port int64) (oldPort int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	oldPort, ok := pm.freePort[runId]
	if ok {
		return
	}
	pm.freePort[runId] = port
	return
}

//增加ftp端口隐射
func (pm *PortManager) AddFtp(runId string, port int64) (oldPort int64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	oldPort, ok := pm.ftpPort[runId]
	if ok {
		return
	}
	pm.ftpPort[runId] = port
	return
}

//获取端口
func (pm *PortManager) GetById(runId string) (port int64, ok bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	port, ok = pm.freePort[runId]
	return
}

//获取ftp端口
func (pm *PortManager) GetFtpById(runId string) (port int64, ok bool) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	port, ok = pm.ftpPort[runId]
	return
}

type ControlManager struct {
	// controls indexed by run id
	ctlsByRunId map[string]*Control

	mu sync.RWMutex
}

//新建ctrl-Manager, 用runid来做mapping管理
func NewControlManager() *ControlManager {
	return &ControlManager{
		ctlsByRunId: make(map[string]*Control),
	}
}

//增加
func (cm *ControlManager) Add(runId string, ctl *Control) (oldCtl *Control) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	oldCtl, ok := cm.ctlsByRunId[runId]
	if ok {
		oldCtl.Replaced(ctl)
	}
	cm.ctlsByRunId[runId] = ctl
	return
}

//获取
func (cm *ControlManager) GetById(runId string) (ctl *Control, ok bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	ctl, ok = cm.ctlsByRunId[runId]
	return
}

//代理
type ProxyManager struct {
	// proxies indexed by proxy name
	pxys map[string]Proxy

	mu sync.RWMutex
}

//代理管理结构
func NewProxyManager() *ProxyManager {
	return &ProxyManager{
		pxys: make(map[string]Proxy),
	}
}

//新加Proxy
func (pm *ProxyManager) Add(name string, pxy Proxy) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.pxys[name]; ok {
		return fmt.Errorf("proxy name [%s] is already in use", name)
	}

	pm.pxys[name] = pxy
	return nil
}

//删除proxy
func (pm *ProxyManager) Del(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.pxys, name)
}

//获取proxy
func (pm *ProxyManager) GetByName(name string) (pxy Proxy, ok bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	pxy, ok = pm.pxys[name]
	return
}
