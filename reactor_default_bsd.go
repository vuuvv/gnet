// Copyright (c) 2019 Andy Pan
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

//go:build (freebsd || dragonfly || darwin) && !poll_opt
// +build freebsd dragonfly darwin
// +build !poll_opt

package gnet

import (
	"runtime"

	"github.com/panjf2000/gnet/internal/netpoll"
	"github.com/panjf2000/gnet/pkg/errors"
)

func (el *eventloop) activateMainReactor(lockOSThread bool) {
	if lockOSThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	defer el.svr.signalShutdown()

	err := el.poller.Polling(func(fd int, filter int16) error { return el.svr.acceptNewConnection(filter) })
	if err == errors.ErrServerShutdown {
		el.svr.opts.Logger.Debugf("main reactor is exiting in terms of the demand from user, %v", err)
	} else if err != nil {
		el.svr.opts.Logger.Errorf("main reactor is exiting due to error: %v", err)
	}
}

func (el *eventloop) activateSubReactor(lockOSThread bool) {
	if lockOSThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	defer func() {
		el.closeAllConns()
		el.svr.signalShutdown()
	}()

	err := el.poller.Polling(func(fd int, filter int16) (err error) {
		if c, ack := el.connections[fd]; ack {
			switch filter {
			case netpoll.EVFilterSock:
				err = el.loopCloseConn(c, nil)
			case netpoll.EVFilterWrite:
				if !c.outboundBuffer.IsEmpty() {
					err = el.loopWrite(c)
				}
			case netpoll.EVFilterRead:
				err = el.loopRead(c)
			}
		}
		return
	})
	if err == errors.ErrServerShutdown {
		el.svr.opts.Logger.Debugf("event-loop(%d) is exiting in terms of the demand from user, %v", el.idx, err)
	} else if err != nil {
		el.svr.opts.Logger.Errorf("event-loop(%d) is exiting due to error: %v", el.idx, err)
	}
}

func (el *eventloop) loopRun(lockOSThread bool) {
	if lockOSThread {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}

	defer func() {
		el.closeAllConns()
		el.ln.close()
		el.svr.signalShutdown()
	}()

	err := el.poller.Polling(func(fd int, filter int16) (err error) {
		if c, ack := el.connections[fd]; ack {
			switch filter {
			case netpoll.EVFilterSock:
				err = el.loopCloseConn(c, nil)
			case netpoll.EVFilterWrite:
				if !c.outboundBuffer.IsEmpty() {
					err = el.loopWrite(c)
				}
			case netpoll.EVFilterRead:
				err = el.loopRead(c)
			}
			return
		}
		return el.loopAccept(filter)
	})
	el.getLogger().Debugf("event-loop(%d) is exiting due to error: %v", el.idx, err)
}
