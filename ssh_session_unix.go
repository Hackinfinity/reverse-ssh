// reverseSSH - a lightweight ssh server with a reverse connection feature
// Copyright (C) 2021  Ferdinor <ferdinor@mailbox.org>

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
)

func createPty(s ssh.Session, shell string) {
	var (
		ptyReq, winCh, _ = s.Pty()
		ctx, cancel      = context.WithCancel(context.Background())
		cmd              = exec.CommandContext(ctx, shell)
	)
	defer cancel()

	cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
	f, err := pty.Start(cmd)
	if err != nil {
		log.Fatalln("Could not start shell:", err)
	}
	go func() {
		for win := range winCh {
			winSize := &pty.Winsize{Rows: uint16(win.Height), Cols: uint16(win.Width)}
			pty.Setsize(f, winSize)
		}
	}()

	go io.Copy(f, s)
	go io.Copy(s, f)

	if err := cmd.Wait(); err != nil {
		log.Println("Session ended with error:", err)
		s.Exit(255)
		return
	}

	if cmd.ProcessState != nil {
		s.Exit(cmd.ProcessState.ExitCode())
	} else {
		// When the process state is not populated something strange
		// happenend. I would consider this a bug that I overlooked.
		log.Printf("Unknown error happenend. Bug?\n")
		log.Printf("You may report it: https://github.com/Fahrj/reverse-ssh/issues\n")
	}
}
