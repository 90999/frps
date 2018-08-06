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
	"time"

	"github.com/KunTengRom/xfrps/assets"
	"github.com/KunTengRom/xfrps/models/config"
	"github.com/KunTengRom/xfrps/models/msg"
	"github.com/KunTengRom/xfrps/utils/log"
	frpNet "github.com/KunTengRom/xfrps/utils/net"
	"github.com/KunTengRom/xfrps/utils/util"
	"github.com/KunTengRom/xfrps/utils/version"
	"github.com/KunTengRom/xfrps/utils/vhost"

	"github.com/xtaci/smux"
)

const (
	connReadTimeout time.Duration = 10 * time.Second
)

var ServerService *Service

// 服务结构
// Server service.
type Service struct {
	// Accept listener
	// Accept connections from client.
	listener frpNet.Listener

	// HTTP vhost
	// For http proxies, route requests to different clients by hostname and other infomation.
	VhostHttpMuxer *vhost.HttpMuxer

	// HTTPS
	// For https proxies, route requests to different clients by hostname and other infomation.
	VhostHttpsMuxer *vhost.HttpsMuxer

	//control管理
	// Manage all controllers.
	ctlManager *ControlManager

	//proxy管理
	// Manage all proxies.
	pxyManager *ProxyManager

	//port管理
	// Manage all free port for each client
	portManager *PortManager
}

//新建serivce向外提供服务
func NewService() (svr *Service, err error) {
	svr = &Service{
		ctlManager:  NewControlManager(), //新建ctl-Manager
		pxyManager:  NewProxyManager(),   //新建Proxy-Manager
		portManager: NewPortManager(),    //新建Port-Manager
	}

	// Init assets.
	//资源文件加载
	err = assets.Load(config.ServerCommonCfg.AssetsDir)
	if err != nil {
		err = fmt.Errorf("Load assets error: %v", err)
		return
	}

	// 服务监听绑定端口
	svr.listener, err = frpNet.ListenTcp(config.ServerCommonCfg.BindAddr, config.ServerCommonCfg.BindPort)
	if err != nil {
		err = fmt.Errorf("Create server listener error, %v", err)
		return
	}

	//http vhost port,  HTTP
	if config.ServerCommonCfg.VhostHttpPort != 0 {
		var l frpNet.Listener
		//提供vhost http服务
		l, err = frpNet.ListenTcp(config.ServerCommonCfg.BindAddr, config.ServerCommonCfg.VhostHttpPort)
		if err != nil {
			err = fmt.Errorf("Create vhost http listener error, %v", err)
			return
		}
		svr.VhostHttpMuxer, err = vhost.NewHttpMuxer(l, 30*time.Second)
		if err != nil {
			err = fmt.Errorf("Create vhost httpMuxer error, %v", err)
			return
		}
	}

	// Create https vhost muxer. HTTPS
	if config.ServerCommonCfg.VhostHttpsPort != 0 {
		var l frpNet.Listener

		//建立服务器监听
		l, err = frpNet.ListenTcp(config.ServerCommonCfg.BindAddr, config.ServerCommonCfg.VhostHttpsPort)
		if err != nil {
			err = fmt.Errorf("Create vhost https listener error, %v", err)
			return
		}
		svr.VhostHttpsMuxer, err = vhost.NewHttpsMuxer(l, 30*time.Second)
		if err != nil {
			err = fmt.Errorf("Create vhost httpsMuxer error, %v", err)
			return
		}
	}

	// Create dashboard web server. 管理后台, 默认为bindPort + 1
	if config.ServerCommonCfg.DashboardPort == 0 {
		config.ServerCommonCfg.DashboardPort = config.ServerCommonCfg.BindPort + 1
	}
	//启动Dashboard服务
	err = RunDashboardServer(config.ServerCommonCfg.BindAddr, config.ServerCommonCfg.DashboardPort)
	if err != nil {
		err = fmt.Errorf("Create dashboard web server error, %v", err)
		return
	}
	log.Info("Dashboard listen on %s:%d", config.ServerCommonCfg.BindAddr, config.ServerCommonCfg.DashboardPort)
	return
}

//服务运行
func (svr *Service) Run() {
	// Listen for incoming connections from client.
	for {
		//监听句柄上来了新的连接
		c, err := svr.listener.Accept()
		if err != nil {
			log.Warn("Listener for incoming connections from client closed")
			return
		}

		// 新起一个协程来处理这个连接
		// Start a new goroutine for dealing connections.
		go func(frpConn frpNet.Conn) {
			dealFn := func(conn frpNet.Conn) {
				var rawMsg msg.Message
				conn.SetReadDeadline(time.Now().Add(connReadTimeout))

				//读消息
				if rawMsg, err = msg.ReadMsg(conn); err != nil {
					log.Warn("Failed to read message: %v", err)
					conn.Close()
					return
				}
				conn.SetReadDeadline(time.Time{})

				//判断消息类型
				switch m := rawMsg.(type) {
				case *msg.Login:
					// 用户注册消息
					err = svr.RegisterControl(conn, m)
					// If login failed, send error message there.
					// Otherwise send success message in control's work goroutine.
					if err != nil {
						conn.Warn("%v", err)
						msg.WriteMsg(conn, &msg.LoginResp{
							Version: version.Full(),
							Error:   err.Error(),
						})
						conn.Close()
					}
				case *msg.NewWorkConn:
					// frpc connected frps for its proxy, and store this connection in control's workConnCh
					svr.RegisterWorkConn(conn, m)
				default:
					log.Warn("Error message type for the new connection [%s]", conn.RemoteAddr().String())
					conn.Close()
				}
			}

			if config.ServerCommonCfg.TcpMux {
				session, err := smux.Server(frpConn, nil)
				if err != nil {
					log.Warn("Failed to create mux connection: %v", err)
					frpConn.Close()
					return
				}

				for {
					stream, err := session.AcceptStream()
					if err != nil {
						log.Warn("Accept new mux stream error: %v", err)
						session.Close()
						return
					}
					wrapConn := frpNet.WrapConn(stream)
					go dealFn(wrapConn)
				}
			} else {
				dealFn(frpConn)
			}
		}(c)
	}
}

//客户端注册control
func (svr *Service) RegisterControl(ctlConn frpNet.Conn, loginMsg *msg.Login) (err error) {
	ctlConn.Info("client login info: ip [%s] version [%s] hostname [%s] os [%s] arch [%s] runId [%s]",
		ctlConn.RemoteAddr().String(), loginMsg.Version, loginMsg.Hostname, loginMsg.Os, loginMsg.Arch, loginMsg.RunId)

	// Check client version.
	if ok, msg := version.Compat(loginMsg.Version); !ok {
		err = fmt.Errorf("%s", msg)
		return
	}

	// Check auth.
	nowTime := time.Now().Unix()
	if config.ServerCommonCfg.AuthTimeout != 0 && nowTime-loginMsg.Timestamp > config.ServerCommonCfg.AuthTimeout {
		err = fmt.Errorf("authorization timeout")
		return
	}

	//获取认证key
	if util.GetAuthKey(config.ServerCommonCfg.PrivilegeToken, loginMsg.Timestamp) != loginMsg.PrivilegeKey {
		err = fmt.Errorf("authorization failed")
		return
	}

	// runid不能为空
	if loginMsg.RunId == "" {
		err = fmt.Errorf("need RunId")
		return
	}

	// 新建一个control，记录connection+loginMsg
	ctl := NewControl(svr, ctlConn, loginMsg)
	//加入ctl管理
	if oldCtl := svr.ctlManager.Add(loginMsg.RunId, ctl); oldCtl != nil {
		oldCtl.allShutdown.WaitDown()
	}

	//log
	ctlConn.AddLogPrefix(loginMsg.RunId)

	//启用新建的ctrl
	ctl.Start()

	// 更新数据统计
	StatsNewClient(loginMsg.RunId)
	return
}

//
// RegisterWorkConn register a new work connection to control and proxies need it.
func (svr *Service) RegisterWorkConn(workConn frpNet.Conn, newMsg *msg.NewWorkConn) {

	// 拿到对应的ctrl
	ctl, exist := svr.ctlManager.GetById(newMsg.RunId)
	if !exist {
		workConn.Warn("No client control found for run id [%s]", newMsg.RunId)
		return
	}

	//这个ctrl上注册新的workConnection
	ctl.RegisterWorkConn(workConn)

	return
}

//放到proxyManager中管理
func (svr *Service) RegisterProxy(name string, pxy Proxy) error {
	err := svr.pxyManager.Add(name, pxy)
	return err
}

func (svr *Service) DelProxy(name string) {
	svr.pxyManager.Del(name)
}
