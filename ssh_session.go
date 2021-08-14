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

	"github.com/gliderlabs/ssh"
)

func makeSSHSessionHandler(shell string) ssh.Handler {
	return func(s ssh.Session) {
		log.Printf("New login from %s@%s", s.User(), s.RemoteAddr().String())
		_, _, isPty := s.Pty()

		switch {
		case isPty:
			log.Println("PTY requested")

			createPty(s, shell)

		case len(s.Command()) > 0:
			log.Printf("No PTY requested, executing command: '%s'", s.RawCommand())

			var (
				ctx, cancel = context.WithCancel(context.Background())
				cmd         = exec.CommandContext(ctx, s.Command()[0], s.Command()[1:]...)
			)
			defer cancel()

			if stdin, err := cmd.StdinPipe(); err != nil {
				log.Println("Could not initialize StdinPipe", err)
				s.Exit(1)
				return
			} else {
				go func() {
					if _, err := io.Copy(stdin, s); err != nil {
						log.Printf("Error while copying input from %s to stdin: %s", s.RemoteAddr().String(), err)
					}
					// When the copy returns, kill the child process
					// by cancelling the context. Everything is cleaned
					// up automatically.
					cancel()
				}()
			}

			cmd.Stdout = s
			cmd.Stderr = s

			logError := func(f string, v ...interface{}) {
				log.Printf(f, v...)
				fmt.Fprintf(s, f, v...)
			}

			// The pattern with context is described here:
			// https://blog.golang.org/context
			c := make(chan error, 1)
			go func() { c <- cmd.Run() }()

			select {

			// cmd.Run() terminated.
			case err := <-c:
				if err != nil {
					logError("Command execution failed: %s\n", err)
					s.Exit(255)
					return
				}

				// No error case.
				if cmd.ProcessState != nil {
					s.Exit(cmd.ProcessState.ExitCode())
				} else {
					// When the process state is not populated something strange
					// happenend. I would consider this a bug that I overlooked.
					logError("Unknown error happenend. Bug?\n")
					logError("You may report it: https://github.com/Fahrj/reverse-ssh/issues\n")
				}
				return

			// cmd.Run() was killed externally.
			case <-ctx.Done():
				logError("Command execution failed: %s\n", ctx.Err())
				s.Exit(254)
				return

			// The TCP connection died.
			case <-s.Context().Done():
				logError("Connection terminated unexpectedly: %s\n", s.Context().Err())
				return
			}

		default:
			log.Println("No PTY requested, no command supplied")

			select {
			case <-s.Context().Done():
				log.Println("Session closed")
			}
		}
	}
}
